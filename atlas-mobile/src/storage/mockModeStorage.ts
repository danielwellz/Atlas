import AsyncStorage from '@react-native-async-storage/async-storage';

const MOCK_MODE_KEY = 'atlas.mobile.mock_mode';

export async function loadMockModePreference(): Promise<boolean | null> {
  const value = await AsyncStorage.getItem(MOCK_MODE_KEY);

  if (value === null) {
    return null;
  }

  return value === 'true';
}

export async function saveMockModePreference(enabled: boolean): Promise<void> {
  await AsyncStorage.setItem(MOCK_MODE_KEY, String(enabled));
}
