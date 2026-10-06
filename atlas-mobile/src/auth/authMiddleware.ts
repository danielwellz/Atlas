import type { Middleware } from 'openapi-fetch';
import { getValidAccessToken, refreshTokens } from './tokenManager';

const AUTH_PATH_PREFIX = '/api/v1/auth/';

function isAuthEndpoint(schemaPath: string): boolean {
  return schemaPath.startsWith(AUTH_PATH_PREFIX);
}

function withBearer(request: Request, accessToken: string): Request {
  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${accessToken}`);
  return new Request(request, { headers });
}

type PendingRequest = {
  /** Unsent copy of the request, so it can be replayed after a refresh. */
  replay: Request;
  accessToken: string;
};

/**
 * openapi-fetch middleware that:
 * - sets `Authorization` from the token manager (replacing any stale header a caller passed);
 * - on a 401, refreshes once (shared with concurrent requests) and retries the request once.
 * `/api/v1/auth/*` endpoints are left alone. Requests made while signed out (no tokens in
 * the token manager, e.g. mock mode) keep whatever headers the caller set.
 */
export function createAuthMiddleware(): Middleware {
  const pending = new Map<string, PendingRequest>();

  return {
    async onRequest({ request, schemaPath, id }) {
      if (isAuthEndpoint(schemaPath)) {
        return undefined;
      }

      const accessToken = await getValidAccessToken();
      if (!accessToken) {
        return undefined;
      }

      const authorized = withBearer(request, accessToken);
      pending.set(id, { replay: authorized.clone(), accessToken });
      return authorized;
    },

    async onResponse({ response, options, id }) {
      const entry = pending.get(id);
      pending.delete(id);
      if (!entry || response.status !== 401) {
        return undefined;
      }

      const outcome = await refreshTokens(entry.accessToken);
      const freshToken = await getValidAccessToken();
      if (outcome !== 'refreshed' || !freshToken || freshToken === entry.accessToken) {
        return undefined;
      }

      // Sent with fetch directly, not through the middleware chain, so it retries at most once.
      return options.fetch(withBearer(entry.replay, freshToken));
    },

    onError({ id }) {
      pending.delete(id);
      return undefined;
    },
  };
}
