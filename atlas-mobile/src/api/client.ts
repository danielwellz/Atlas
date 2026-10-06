import createClient from 'openapi-fetch';
import { appConfig } from '../config';
import type { paths } from './generated/openapi';

export const atlasApiClient = createClient<paths>({
  baseUrl: appConfig.apiBaseUrl,
  // Resolve fetch per request (instead of capturing it once) so tests can swap globalThis.fetch.
  fetch: (request: Request) => globalThis.fetch(request),
});

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: string }).message;
    if (message) {
      return message;
    }
  }

  return fallback;
}
