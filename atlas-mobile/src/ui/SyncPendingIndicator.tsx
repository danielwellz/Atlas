import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useOnlineStatus } from '../network/onlineManager';
import { useOutboxPendingCount } from '../sync/hooks';

export function SyncPendingIndicator(): React.JSX.Element | null {
  const pendingCount = useOutboxPendingCount();
  const isOnline = useOnlineStatus();

  if (pendingCount < 1) {
    return null;
  }

  return (
    <View style={styles.container} testID="sync-pending-indicator">
      <Text style={styles.text}>
        Sync pending: {pendingCount}
        {!isOnline ? ' (offline)' : ''}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    borderRadius: 8,
    backgroundColor: '#fef3c7',
    borderWidth: 1,
    borderColor: '#f59e0b',
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  text: {
    color: '#7c2d12',
    fontSize: 13,
    fontWeight: '600',
  },
});

