import { atlasApiClient } from '../api/client';
import { createAuthMiddleware } from './authMiddleware';

export {
  endSession,
  getAccessToken,
  getTokens,
  getValidAccessToken,
  refreshTokens,
  setTokens,
  startSession,
  subscribeTokens,
} from './tokenManager';
export type { RefreshOutcome, TokenChangeReason } from './tokenManager';

let installed = false;

/** Registers the auth middleware on the shared API client (idempotent). */
export function installAuthMiddleware(): void {
  if (installed) {
    return;
  }
  installed = true;
  atlasApiClient.use(createAuthMiddleware());
}
