import { Platform } from 'react-native';
import createClient from 'openapi-fetch';
import type { paths } from './generated/openapi';

const API_PORT = 8080;

export const API_BASE_URL = Platform.select({
  android: `http://10.0.2.2:${API_PORT}`,
  ios: `http://localhost:${API_PORT}`,
  default: `http://localhost:${API_PORT}`,
}) as string;

export const atlasApiClient = createClient<paths>({
  baseUrl: API_BASE_URL,
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
