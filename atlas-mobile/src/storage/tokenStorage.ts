import * as Keychain from 'react-native-keychain';
import type { components } from '../api/generated/openapi';

const TOKEN_SERVICE = 'atlas.mobile.tokens';
const TOKEN_ACCOUNT = 'atlas';

type TokenResponse = components['schemas']['TokenResponse'];

export async function saveTokens(tokens: TokenResponse): Promise<void> {
  await Keychain.setGenericPassword(TOKEN_ACCOUNT, JSON.stringify(tokens), {
    service: TOKEN_SERVICE,
  });
}

export async function loadTokens(): Promise<TokenResponse | null> {
  const credentials = await Keychain.getGenericPassword({ service: TOKEN_SERVICE });

  if (!credentials) {
    return null;
  }

  try {
    return JSON.parse(credentials.password) as TokenResponse;
  } catch {
    await clearTokens();
    return null;
  }
}

export async function clearTokens(): Promise<void> {
  await Keychain.resetGenericPassword({ service: TOKEN_SERVICE });
}
