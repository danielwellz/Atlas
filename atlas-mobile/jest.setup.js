/* eslint-env jest */

import 'react-native-gesture-handler/jestSetup';

jest.mock('@react-native-async-storage/async-storage', () => {
  const store = {};

  const mockAsyncStorage = {
    getItem: jest.fn(async key => store[key] ?? null),
    setItem: jest.fn(async (key, value) => {
      store[key] = value;
    }),
    removeItem: jest.fn(async key => {
      delete store[key];
    }),
    clear: jest.fn(async () => {
      Object.keys(store).forEach(key => {
        delete store[key];
      });
    }),
  };

  return {
    __esModule: true,
    default: mockAsyncStorage,
    ...mockAsyncStorage,
  };
});

jest.mock('react-native-keychain', () => ({
  setGenericPassword: jest.fn().mockResolvedValue(true),
  getGenericPassword: jest.fn().mockResolvedValue(false),
  resetGenericPassword: jest.fn().mockResolvedValue(true),
}));

jest.mock('@react-native-community/netinfo', () => {
  let state = {
    isConnected: true,
    isInternetReachable: true,
  };

  const listeners = new Set();

  const addEventListener = jest.fn(listener => {
    listeners.add(listener);
    listener(state);

    return () => {
      listeners.delete(listener);
    };
  });

  const setState = nextState => {
    state = {
      ...state,
      ...nextState,
    };

    listeners.forEach(listener => {
      listener(state);
    });
  };

  const fetch = jest.fn(async () => state);
  const useNetInfo = jest.fn(() => state);

  return {
    __esModule: true,
    default: {
      addEventListener,
      fetch,
      useNetInfo,
      __setNetInfoState: setState,
    },
    addEventListener,
    fetch,
    useNetInfo,
    __setNetInfoState: setState,
  };
});

jest.mock('react-native-camera-kit', () => {
  const React = require('react');
  const { View } = require('react-native');

  return {
    Camera: (props) => React.createElement(View, { ...props }),
    CameraType: {
      Front: 'front',
      Back: 'back',
    },
    default: {},
  };
});

jest.mock('react-native-permissions', () => ({
  PERMISSIONS: {
    IOS: {
      CAMERA: 'ios.permission.CAMERA',
    },
    ANDROID: {
      CAMERA: 'android.permission.CAMERA',
    },
  },
  RESULTS: {
    UNAVAILABLE: 'unavailable',
    BLOCKED: 'blocked',
    DENIED: 'denied',
    GRANTED: 'granted',
    LIMITED: 'limited',
  },
  check: jest.fn(async () => 'granted'),
  request: jest.fn(async () => 'granted'),
  openSettings: jest.fn(async () => {}),
}));

jest.mock('react-native-iap', () => ({
  initConnection: jest.fn(async () => true),
  endConnection: jest.fn(async () => true),
  fetchProducts: jest.fn(async () => []),
  requestPurchase: jest.fn(async () => null),
  getAvailablePurchases: jest.fn(async () => []),
  finishTransaction: jest.fn(async () => undefined),
}));
