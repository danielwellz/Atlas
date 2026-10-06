import AsyncStorage from '@react-native-async-storage/async-storage';
import type { components } from '../api/generated/openapi';

type User = components['schemas']['User'];

// Last user profile loaded from /me, so the app can start signed in while offline.
// Not secret (tokens stay in the Keychain); cleared on logout and session expiry.
const SESSION_USER_STORAGE_KEY = 'atlas.mobile.auth.user.v1';

export async function saveSessionUser(user: User): Promise<void> {
  await AsyncStorage.setItem(SESSION_USER_STORAGE_KEY, JSON.stringify(user));
}

export async function loadSessionUser(): Promise<User | null> {
  const raw = await AsyncStorage.getItem(SESSION_USER_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<User> | null;
    return parsed && typeof parsed.id === 'string' && typeof parsed.email === 'string'
      ? (parsed as User)
      : null;
  } catch {
    return null;
  }
}

export async function clearSessionUser(): Promise<void> {
  await AsyncStorage.removeItem(SESSION_USER_STORAGE_KEY);
}
