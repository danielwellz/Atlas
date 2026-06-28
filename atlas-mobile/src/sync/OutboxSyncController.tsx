import React, { useCallback, useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useOnlineStatus } from '../network/onlineManager';
import { useAuth } from '../state/AuthContext';
import { useMockMode } from '../state/MockModeContext';
import { useOutboxPendingCount } from './hooks';
import { flushOutbox } from './outbox';

const RETRY_INTERVAL_MS = 15_000;

export function OutboxSyncController(): React.JSX.Element | null {
  const queryClient = useQueryClient();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const isOnline = useOnlineStatus();
  const pendingCount = useOutboxPendingCount();
  const isFlushingRef = useRef(false);

  const flushPending = useCallback(async () => {
    const accessToken = session?.tokens.accessToken ?? '';
    const mockEnabled = __DEV__ && isMockMode;
    if (!isOnline || mockEnabled || pendingCount === 0 || isFlushingRef.current) {
      return;
    }

    isFlushingRef.current = true;
    try {
      const result = await flushOutbox(accessToken);
      if (result.flushedCount > 0) {
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: ['workout'] }),
          queryClient.invalidateQueries({ queryKey: ['dashboard'] }),
          queryClient.invalidateQueries({ queryKey: ['food-logs'] }),
        ]);
      }
    } finally {
      isFlushingRef.current = false;
    }
  }, [
    isMockMode,
    isOnline,
    pendingCount,
    queryClient,
    session?.tokens.accessToken,
  ]);

  useEffect(() => {
    flushPending().catch(() => {});
  }, [flushPending]);

  useEffect(() => {
    if (!isOnline || pendingCount === 0) {
      return;
    }

    const interval = setInterval(() => {
      flushPending().catch(() => {});
    }, RETRY_INTERVAL_MS);

    return () => {
      clearInterval(interval);
    };
  }, [isOnline, pendingCount, flushPending]);

  return null;
}
