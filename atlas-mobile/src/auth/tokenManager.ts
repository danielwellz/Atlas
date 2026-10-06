import type { components } from '../api/generated/openapi';
import { RefreshRejectedError, refreshAuthTokens } from '../api/services/authService';
import { clearTokens, saveTokens } from '../storage/tokenStorage';

type TokenResponse = components['schemas']['TokenResponse'];

/**
 * - `refreshed`: a valid token pair is now current (possibly refreshed by another caller).
 * - `rejected`: the refresh token is invalid; the session has been cleared.
 * - `unavailable`: the refresh could not be completed (offline, server error); tokens kept.
 */
export type RefreshOutcome = 'refreshed' | 'rejected' | 'unavailable';

export type TokenChangeReason = 'set' | 'refreshed' | 'expired';

type TokenListener = (tokens: TokenResponse | null, reason: TokenChangeReason) => void;

/** Refresh slightly before the JWT `exp` so requests don't race the expiry. */
const EXPIRY_SKEW_SECONDS = 30;

let currentTokens: TokenResponse | null = null;
let refreshInFlight: Promise<RefreshOutcome> | null = null;
// Bumped whenever the session is replaced or cleared, so a refresh that started for an
// earlier session can't resurrect it after logout or a new login.
let sessionGeneration = 0;
const listeners = new Set<TokenListener>();

function emit(reason: TokenChangeReason): void {
  for (const listener of listeners) {
    listener(currentTokens, reason);
  }
}

function decodeBase64Url(segment: string): string | null {
  // Hermes and Node provide atob; RN's type definitions don't declare it.
  const atob = (globalThis as { atob?: (data: string) => string }).atob;
  if (typeof atob !== 'function') {
    return null;
  }
  const base64 = segment.replace(/-/g, '+').replace(/_/g, '/');
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
  return atob(padded);
}

/** Returns the access token's JWT `exp` (seconds since epoch), or null if it can't be read. */
export function readAccessTokenExpiry(accessToken: string): number | null {
  const payloadSegment = accessToken.split('.')[1];
  if (!payloadSegment) {
    return null;
  }
  try {
    const json = decodeBase64Url(payloadSegment);
    if (!json) {
      return null;
    }
    const exp = (JSON.parse(json) as { exp?: unknown }).exp;
    return typeof exp === 'number' && Number.isFinite(exp) ? exp : null;
  } catch {
    return null;
  }
}

function isExpired(accessToken: string, nowMs: number): boolean {
  const exp = readAccessTokenExpiry(accessToken);
  return exp !== null && exp - EXPIRY_SKEW_SECONDS <= nowMs / 1000;
}

export function getTokens(): TokenResponse | null {
  return currentTokens;
}

export function getAccessToken(): string | null {
  return currentTokens?.accessToken ?? null;
}

/** Makes `tokens` the current session in memory (persisting is the caller's job). */
export function setTokens(tokens: TokenResponse | null): void {
  sessionGeneration += 1;
  currentTokens = tokens;
  emit('set');
}

/** Replaces the session with a freshly issued token pair and persists it. */
export async function startSession(tokens: TokenResponse): Promise<void> {
  await saveTokens(tokens);
  setTokens(tokens);
}

/** Forgets the session in memory and in the Keychain. */
export async function endSession(): Promise<void> {
  setTokens(null);
  await clearTokens();
}

async function expireSession(): Promise<void> {
  sessionGeneration += 1;
  currentTokens = null;
  await clearTokens();
  emit('expired');
}

/**
 * Refreshes the token pair. Concurrent callers share one request (single-flight).
 * Pass the access token that was rejected: if the current token already differs,
 * someone else refreshed in the meantime and no new request is made.
 */
export function refreshTokens(staleAccessToken?: string): Promise<RefreshOutcome> {
  if (refreshInFlight) {
    return refreshInFlight;
  }

  const tokens = currentTokens;
  if (!tokens?.refreshToken) {
    return Promise.resolve('rejected');
  }
  if (staleAccessToken && tokens.accessToken !== staleAccessToken) {
    return Promise.resolve('refreshed');
  }

  const generation = sessionGeneration;
  refreshInFlight = (async (): Promise<RefreshOutcome> => {
    let nextTokens: TokenResponse;
    try {
      nextTokens = await refreshAuthTokens(tokens.refreshToken);
    } catch (error) {
      if (generation !== sessionGeneration) {
        return currentTokens ? 'refreshed' : 'rejected';
      }
      if (error instanceof RefreshRejectedError) {
        await expireSession();
        return 'rejected';
      }
      return 'unavailable';
    }

    if (generation !== sessionGeneration) {
      // Logged out or logged in again while refreshing; drop the result.
      return currentTokens ? 'refreshed' : 'rejected';
    }

    currentTokens = nextTokens;
    emit('refreshed');
    await saveTokens(nextTokens);
    return 'refreshed';
  })().finally(() => {
    refreshInFlight = null;
  });

  return refreshInFlight;
}

/**
 * The access token to use right now, refreshed first if its JWT `exp` has passed.
 * Returns null when there is no session. If the refresh can't complete (offline),
 * returns the current token and lets the request fail or be retried later.
 */
export async function getValidAccessToken(): Promise<string | null> {
  const tokens = currentTokens;
  if (!tokens) {
    return null;
  }
  if (refreshInFlight || isExpired(tokens.accessToken, Date.now())) {
    await refreshTokens(tokens.accessToken);
  }
  return currentTokens?.accessToken ?? null;
}

export function subscribeTokens(listener: TokenListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Test helper: drop all state between tests. */
export function resetTokenManagerForTests(): void {
  currentTokens = null;
  refreshInFlight = null;
  sessionGeneration += 1;
  listeners.clear();
}
