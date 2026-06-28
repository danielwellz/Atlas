import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { loadMockModePreference, saveMockModePreference } from '../storage/mockModeStorage';

type MockModeContextValue = {
  isMockMode: boolean;
  canUseMockMode: boolean;
  isHydrated: boolean;
  setMockMode: (enabled: boolean) => Promise<void>;
  toggleMockMode: () => Promise<void>;
};

type MockModeProviderProps = {
  children: React.ReactNode;
  initialMode?: boolean;
  disablePersistence?: boolean;
};

const MockModeContext = createContext<MockModeContextValue | undefined>(undefined);
const CAN_USE_MOCK_MODE = __DEV__;

export function MockModeProvider({
  children,
  initialMode = false,
  disablePersistence = false,
}: MockModeProviderProps): React.JSX.Element {
  const [isMockMode, setIsMockMode] = useState(CAN_USE_MOCK_MODE && initialMode);
  const [isHydrated, setIsHydrated] = useState(disablePersistence || !CAN_USE_MOCK_MODE);

  useEffect(() => {
    if (disablePersistence || !CAN_USE_MOCK_MODE) {
      if (!disablePersistence) {
        setIsMockMode(false);
        setIsHydrated(true);
      }
      return;
    }

    let cancelled = false;

    const hydrate = async () => {
      try {
        const stored = await loadMockModePreference();

        if (!cancelled && stored !== null) {
          setIsMockMode(stored);
        }
      } finally {
        if (!cancelled) {
          setIsHydrated(true);
        }
      }
    };

    hydrate().catch(() => {
      if (!cancelled) {
        setIsHydrated(true);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [disablePersistence]);

  const setMockMode = useCallback(
    async (enabled: boolean) => {
      if (!CAN_USE_MOCK_MODE) {
        setIsMockMode(false);
        return;
      }

      setIsMockMode(enabled);
      if (!disablePersistence) {
        await saveMockModePreference(enabled);
      }
    },
    [disablePersistence],
  );

  const toggleMockMode = useCallback(async () => {
    if (!CAN_USE_MOCK_MODE) {
      return;
    }

    const next = !isMockMode;
    await setMockMode(next);
  }, [isMockMode, setMockMode]);

  const value = useMemo(
    () => ({
      isMockMode,
      canUseMockMode: CAN_USE_MOCK_MODE,
      isHydrated,
      setMockMode,
      toggleMockMode,
    }),
    [isMockMode, isHydrated, setMockMode, toggleMockMode],
  );

  return <MockModeContext.Provider value={value}>{children}</MockModeContext.Provider>;
}

export function useMockMode(): MockModeContextValue {
  const context = useContext(MockModeContext);

  if (!context) {
    throw new Error('useMockMode must be used within MockModeProvider');
  }

  return context;
}
