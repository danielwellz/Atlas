import { onlineManager } from '@tanstack/react-query';
import NetInfo from '@react-native-community/netinfo';
import { useEffect, useState } from 'react';

let isConfigured = false;

export function configureOnlineManager(): void {
  if (isConfigured) {
    return;
  }

  onlineManager.setEventListener(setOnline => {
    return NetInfo.addEventListener(state => {
      const isConnected = Boolean(state.isConnected);
      const isReachable = state.isInternetReachable !== false;
      setOnline(isConnected && isReachable);
    });
  });

  isConfigured = true;
}

export function isNetworkOnline(): boolean {
  return onlineManager.isOnline();
}

export function setNetworkOnlineForTests(online: boolean): void {
  onlineManager.setOnline(online);
}

export function useOnlineStatus(): boolean {
  const [isOnline, setIsOnline] = useState(isNetworkOnline());

  useEffect(() => {
    return onlineManager.subscribe(() => {
      setIsOnline(isNetworkOnline());
    });
  }, []);

  return isOnline;
}

