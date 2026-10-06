import React from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import ReactTestRenderer from 'react-test-renderer';
import * as Keychain from 'react-native-keychain';
import { getCurrentUser } from '../../src/api/services/authService';
import { getTokens, resetTokenManagerForTests } from '../../src/auth/tokenManager';
import { AuthProvider, useAuth } from '../../src/state/AuthContext';
import { USER, installFetchStub, jsonResponse, tokens } from '../../testUtils/authFetchStub';

jest.mock('../../src/state/MockModeContext', () => ({
  useMockMode: () => ({ isMockMode: false, isHydrated: true }),
}));

const SESSION_USER_KEY = 'atlas.mobile.auth.user.v1';
const mockedGetGenericPassword = Keychain.getGenericPassword as jest.Mock;

type AuthValue = ReturnType<typeof useAuth>;

function storeTokens(value: ReturnType<typeof tokens>): void {
  mockedGetGenericPassword.mockResolvedValueOnce({
    username: 'atlas',
    password: JSON.stringify(value),
  });
}

async function renderAuth(): Promise<{ current: () => AuthValue }> {
  let latest: AuthValue | null = null;
  function Probe(): null {
    latest = useAuth();
    return null;
  }

  await ReactTestRenderer.act(async () => {
    ReactTestRenderer.create(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
  });
  // Let hydration's async steps settle.
  for (let i = 0; i < 5 && !(latest as AuthValue | null)?.isHydrated; i += 1) {
    await ReactTestRenderer.act(async () => {
      await new Promise(resolve => setTimeout(() => resolve(undefined), 0));
    });
  }

  return {
    current: () => {
      if (!latest) {
        throw new Error('AuthProvider did not render');
      }
      return latest;
    },
  };
}

describe('AuthProvider session hydration', () => {
  beforeEach(async () => {
    resetTokenManagerForTests();
    jest.clearAllMocks();
    await AsyncStorage.clear();
  });

  it('keeps the session on an offline start', async () => {
    storeTokens(tokens('1'));
    await AsyncStorage.setItem(SESSION_USER_KEY, JSON.stringify(USER));
    installFetchStub(() => {
      throw new TypeError('Network request failed');
    });

    const auth = await renderAuth();

    expect(auth.current().isHydrated).toBe(true);
    expect(auth.current().session).toEqual({ user: USER, tokens: tokens('1') });
    expect(getTokens()).toEqual(tokens('1'));
    expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
  });

  it('refreshes an expired access token on a cold start instead of logging out', async () => {
    storeTokens(tokens('1'));
    const calls = installFetchStub(request => {
      if (request.path === '/api/v1/auth/refresh') {
        return jsonResponse(200, tokens('2'));
      }
      return request.authorization === 'Bearer access-2'
        ? jsonResponse(200, { user: USER })
        : jsonResponse(401, { error: 'unauthorized', message: 'expired' });
    });

    const auth = await renderAuth();

    expect(auth.current().session).toEqual({ user: USER, tokens: tokens('2') });
    expect(calls.filter(call => call.path === '/api/v1/auth/refresh')).toHaveLength(1);
    expect(JSON.parse((await AsyncStorage.getItem(SESSION_USER_KEY)) ?? 'null')).toEqual(USER);
  });

  it('logs out on a cold start when the refresh token is rejected', async () => {
    storeTokens(tokens('1'));
    await AsyncStorage.setItem(SESSION_USER_KEY, JSON.stringify(USER));
    installFetchStub(() => jsonResponse(401, { error: 'unauthorized', message: 'revoked' }));

    const auth = await renderAuth();

    expect(auth.current().isHydrated).toBe(true);
    expect(auth.current().session).toBeNull();
    expect(getTokens()).toBeNull();
    expect(Keychain.resetGenericPassword).toHaveBeenCalled();
    expect(await AsyncStorage.getItem(SESSION_USER_KEY)).toBeNull();
  });

  it('keeps tokens in sync after a refresh and logs out when the session later expires', async () => {
    storeTokens(tokens('1'));
    let refreshAllowed = true;
    let access1Valid = true;
    installFetchStub(request => {
      if (request.path === '/api/v1/auth/refresh') {
        return refreshAllowed
          ? jsonResponse(200, tokens('2'))
          : jsonResponse(401, { error: 'unauthorized', message: 'revoked' });
      }
      if (access1Valid && request.authorization === 'Bearer access-1') {
        return jsonResponse(200, { user: USER });
      }
      return jsonResponse(401, { error: 'unauthorized', message: 'expired' });
    });

    const auth = await renderAuth();
    expect(auth.current().session?.tokens).toEqual(tokens('1'));

    // access-1 now expires; a request refreshes to access-2, which the server also rejects.
    // The retry 401s, so the tokens rotate but the call still fails.
    access1Valid = false;
    await ReactTestRenderer.act(async () => {
      await getCurrentUser().catch(() => undefined);
    });
    expect(auth.current().session?.tokens).toEqual(tokens('2'));

    // Then the refresh token is revoked: the next 401 ends the session.
    refreshAllowed = false;
    await ReactTestRenderer.act(async () => {
      await getCurrentUser().catch(() => undefined);
    });
    expect(auth.current().session).toBeNull();
    expect(getTokens()).toBeNull();
  });

  it('logs out with the current tokens and forgets the session', async () => {
    storeTokens(tokens('1'));
    const calls = installFetchStub(request =>
      request.path === '/api/v1/auth/logout'
        ? new Response(null, { status: 204 })
        : jsonResponse(200, { user: USER }),
    );

    const auth = await renderAuth();
    await ReactTestRenderer.act(async () => {
      await auth.current().logout();
    });

    const logoutCall = calls.find(call => call.path === '/api/v1/auth/logout');
    expect(logoutCall?.authorization).toBe('Bearer access-1');
    expect(JSON.parse(logoutCall?.body ?? '{}')).toEqual({ refreshToken: 'refresh-1' });
    expect(auth.current().session).toBeNull();
    expect(getTokens()).toBeNull();
    expect(await AsyncStorage.getItem(SESSION_USER_KEY)).toBeNull();
  });
});
