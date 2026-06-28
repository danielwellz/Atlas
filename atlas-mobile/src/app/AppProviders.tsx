import React, { useState } from 'react';
import { QueryClient } from '@tanstack/react-query';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { PersistQueryClientProvider } from '@tanstack/react-query-persist-client';
import { createAsyncStoragePersister } from '@tanstack/query-async-storage-persister';
import { configureOnlineManager } from '../network/onlineManager';
import { AuthProvider } from '../state/AuthContext';
import { MockModeProvider } from '../state/MockModeContext';
import { OnboardingProvider } from '../state/OnboardingContext';
import { OutboxSyncController } from '../sync/OutboxSyncController';

type AppProvidersProps = {
  children: React.ReactNode;
};

const QUERY_CACHE_STORAGE_KEY = 'atlas.mobile.query-cache.v1';

export function AppProviders({ children }: AppProvidersProps): React.JSX.Element {
  configureOnlineManager();

  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30_000,
            retry: 1,
          },
        },
      }),
  );

  const [queryPersister] = useState(() =>
    createAsyncStoragePersister({
      storage: AsyncStorage,
      key: QUERY_CACHE_STORAGE_KEY,
      throttleTime: 1_000,
    }),
  );

  return (
    <PersistQueryClientProvider
      client={queryClient}
      persistOptions={{
        persister: queryPersister,
        maxAge: 24 * 60 * 60 * 1000,
      }}
      onSuccess={() => {
        queryClient.resumePausedMutations().catch(() => {});
      }}>
      <MockModeProvider>
        <AuthProvider>
          <OutboxSyncController />
          <OnboardingProvider>{children}</OnboardingProvider>
        </AuthProvider>
      </MockModeProvider>
    </PersistQueryClientProvider>
  );
}
