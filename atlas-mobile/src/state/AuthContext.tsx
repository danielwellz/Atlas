import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { components } from '../api/generated/openapi';
import { ApiRequestError, getCurrentUser, logoutUser } from '../api/services/authService';
import {
  endSession,
  getTokens,
  getValidAccessToken,
  installAuthMiddleware,
  refreshTokens,
  setTokens,
  startSession,
  subscribeTokens,
} from '../auth';
import { clearSessionUser, loadSessionUser, saveSessionUser } from '../storage/sessionUserStorage';
import { loadTokens, saveTokens } from '../storage/tokenStorage';
import { useMockMode } from './MockModeContext';

installAuthMiddleware();

type User = components['schemas']['User'];
type TokenResponse = components['schemas']['TokenResponse'];
type AuthResponse = components['schemas']['AuthResponse'];

type AuthSession = {
  user: User;
  tokens: TokenResponse;
};

type AuthContextValue = {
  session: AuthSession | null;
  isAuthenticated: boolean;
  isHydrated: boolean;
  applyAuthResponse: (response: AuthResponse) => Promise<void>;
  refreshCurrentUser: () => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

type AuthProviderProps = {
  children: React.ReactNode;
};

function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiRequestError && error.status === 401;
}

type StartupUser = { kind: 'signed-in'; user: User } | { kind: 'signed-out' } | { kind: 'offline' };

/**
 * Loads the user for a stored session. Signs out only when the API rejects the refresh
 * token; network or server errors keep the session so the app can start offline.
 */
async function loadUserForStoredSession(storedAccessToken: string): Promise<StartupUser> {
  try {
    // The auth middleware attaches the token and refreshes/retries once on a 401.
    return { kind: 'signed-in', user: (await getCurrentUser()).user };
  } catch (error) {
    if (!isUnauthorized(error)) {
      return { kind: 'offline' };
    }
  }

  // Still 401: confirm with an explicit refresh (a no-op if the middleware already refreshed).
  const outcome = await refreshTokens(storedAccessToken);
  if (outcome === 'rejected') {
    return { kind: 'signed-out' };
  }
  if (outcome === 'unavailable') {
    return { kind: 'offline' };
  }

  try {
    return { kind: 'signed-in', user: (await getCurrentUser()).user };
  } catch (error) {
    return isUnauthorized(error) ? { kind: 'signed-out' } : { kind: 'offline' };
  }
}

function buildMockUserFromToken(token: TokenResponse): User {
  return {
    id: 'local-mock-user',
    email: 'athlete@atlas.local',
    isPro: true,
    entitlements: [
      'barcode_scan',
      'deep_nutrition',
      'biomechanics_overlays',
      'form_check_upload',
      'coach_tier_pro',
    ],
    coachTier: 'pro',
    createdAt: new Date(Date.now() - token.expiresIn * 1000).toISOString(),
  };
}

export function AuthProvider({ children }: AuthProviderProps): React.JSX.Element {
  const { isMockMode, isHydrated: mockHydrated } = useMockMode();
  const [session, setSession] = useState<AuthSession | null>(null);
  const [isHydrated, setIsHydrated] = useState(false);

  const hydrateSession = useCallback(async () => {
    if (!mockHydrated) {
      return;
    }

    setIsHydrated(false);

    const storedTokens = await loadTokens();

    if (!storedTokens) {
      setTokens(null);
      setSession(null);
      setIsHydrated(true);
      return;
    }

    if (isMockMode) {
      // Mock sessions never reach the API, so keep their tokens out of the token manager.
      setTokens(null);
      setSession({
        user: buildMockUserFromToken(storedTokens),
        tokens: storedTokens,
      });
      setIsHydrated(true);
      return;
    }

    setTokens(storedTokens);
    try {
      const result = await loadUserForStoredSession(storedTokens.accessToken);
      if (result.kind === 'signed-in') {
        await saveSessionUser(result.user);
        setSession({ user: result.user, tokens: getTokens() ?? storedTokens });
        return;
      }

      if (result.kind === 'signed-out') {
        await endSession();
        await clearSessionUser();
        setSession(null);
        return;
      }

      // Offline start: keep the tokens and use the last known profile, if any.
      const cachedUser = await loadSessionUser();
      const tokens = getTokens();
      setSession(cachedUser && tokens ? { user: cachedUser, tokens } : null);
    } finally {
      setIsHydrated(true);
    }
  }, [isMockMode, mockHydrated]);

  useEffect(
    () =>
      subscribeTokens((tokens, reason) => {
        if (reason === 'refreshed' && tokens) {
          setSession(current => (current ? { ...current, tokens } : current));
        } else if (reason === 'expired') {
          clearSessionUser().catch(() => {});
          setSession(null);
        }
      }),
    [],
  );

  useEffect(() => {
    hydrateSession().catch(() => {
      setSession(null);
      setIsHydrated(true);
    });
  }, [hydrateSession]);

  const applyAuthResponse = useCallback(
    async (response: AuthResponse) => {
      if (isMockMode) {
        await saveTokens(response.tokens);
        setTokens(null);
      } else {
        await startSession(response.tokens);
        await saveSessionUser(response.user);
      }
      setSession({
        user: response.user,
        tokens: response.tokens,
      });
    },
    [isMockMode],
  );

  const logout = useCallback(async () => {
    const tokens = getTokens();
    if (!isMockMode && tokens?.refreshToken) {
      try {
        await logoutUser({
          refreshToken: tokens.refreshToken,
          accessToken: (await getValidAccessToken()) ?? tokens.accessToken,
        });
      } catch {
        // Ignore API logout failures to keep local session cleanup deterministic.
      }
    }

    await endSession();
    await clearSessionUser();
    setSession(null);
  }, [isMockMode]);

  const hasSession = Boolean(session);
  const refreshCurrentUser = useCallback(async () => {
    if (!hasSession || isMockMode) {
      return;
    }

    const meResponse = await getCurrentUser();
    await saveSessionUser(meResponse.user);
    setSession(current => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        user: meResponse.user,
      };
    });
  }, [hasSession, isMockMode]);

  const value = useMemo(
    () => ({
      session,
      isAuthenticated: Boolean(session),
      isHydrated,
      applyAuthResponse,
      refreshCurrentUser,
      logout,
    }),
    [session, isHydrated, applyAuthResponse, refreshCurrentUser, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }

  return context;
}
