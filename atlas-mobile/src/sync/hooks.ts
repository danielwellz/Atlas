import { useEffect, useState } from 'react';
import { getOutboxPendingCount, subscribeOutbox } from './outbox';

export function useOutboxPendingCount(): number {
  const [pendingCount, setPendingCount] = useState(0);

  useEffect(() => {
    let cancelled = false;

    getOutboxPendingCount()
      .then(count => {
        if (!cancelled) {
          setPendingCount(count);
        }
      })
      .catch(() => {});

    const unsubscribe = subscribeOutbox(items => {
      if (!cancelled) {
        setPendingCount(items.length);
      }
    });

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  return pendingCount;
}
