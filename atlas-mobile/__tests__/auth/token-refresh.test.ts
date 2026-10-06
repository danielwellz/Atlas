import * as Keychain from 'react-native-keychain';
import { atlasApiClient } from '../../src/api/client';
import { ApiRequestError, getCurrentUser } from '../../src/api/services/authService';
import { installAuthMiddleware } from '../../src/auth';
import {
  getTokens,
  getValidAccessToken,
  resetTokenManagerForTests,
  setTokens,
  subscribeTokens,
} from '../../src/auth/tokenManager';
import {
  USER,
  installFetchStub,
  jsonResponse,
  jwtWithExp,
  tokens,
} from '../../testUtils/authFetchStub';

installAuthMiddleware();

function unauthorized(): Response {
  return jsonResponse(401, { error: 'unauthorized', message: 'token expired' });
}

describe('token refresh', () => {
  beforeEach(() => {
    resetTokenManagerForTests();
    jest.clearAllMocks();
  });

  it('refreshes an expired access token and retries the request', async () => {
    setTokens(tokens('1'));
    const calls = installFetchStub(request => {
      if (request.path === '/api/v1/auth/refresh') {
        return jsonResponse(200, tokens('2'));
      }
      return request.authorization === 'Bearer access-2'
        ? jsonResponse(200, { user: USER })
        : unauthorized();
    });

    await expect(getCurrentUser()).resolves.toEqual({ user: USER });

    expect(calls.map(call => `${call.path} ${call.authorization ?? ''}`)).toEqual([
      '/api/v1/me Bearer access-1',
      '/api/v1/auth/refresh ',
      '/api/v1/me Bearer access-2',
    ]);
    expect(JSON.parse(calls[1].body)).toEqual({ refreshToken: 'refresh-1' });
    expect(getTokens()).toEqual(tokens('2'));
    expect(Keychain.setGenericPassword).toHaveBeenCalledWith(
      'atlas',
      JSON.stringify(tokens('2')),
      expect.anything(),
    );
  });

  it('replays the request body when retrying a POST', async () => {
    setTokens(tokens('1'));
    const calls = installFetchStub(request => {
      if (request.path === '/api/v1/auth/refresh') {
        return jsonResponse(200, tokens('2'));
      }
      return request.authorization === 'Bearer access-2'
        ? jsonResponse(202, { accepted: true })
        : unauthorized();
    });
    const body = {
      eventName: 'workout_completed' as const,
      eventTime: '2026-10-06T10:00:00.000Z',
      consentGranted: true,
    };

    const response = await atlasApiClient.POST('/api/v1/events', { body });

    expect(response.response.status).toBe(202);
    const eventCalls = calls.filter(call => call.path === '/api/v1/events');
    expect(eventCalls).toHaveLength(2);
    expect(JSON.parse(eventCalls[1].body)).toEqual(body);
  });

  it('shares one refresh between parallel requests', async () => {
    setTokens(tokens('1'));
    let releaseRefresh: () => void = () => {};
    const refreshGate = new Promise<void>(resolve => {
      releaseRefresh = resolve;
    });
    const calls = installFetchStub(async request => {
      if (request.path === '/api/v1/auth/refresh') {
        await refreshGate;
        return jsonResponse(200, tokens('2'));
      }
      return request.authorization === 'Bearer access-2'
        ? jsonResponse(200, { user: USER })
        : unauthorized();
    });

    const both = Promise.all([getCurrentUser(), getCurrentUser()]);
    // Let both requests receive their 401 before the refresh completes.
    await new Promise(resolve => setTimeout(() => resolve(undefined), 10));
    releaseRefresh();

    await expect(both).resolves.toEqual([{ user: USER }, { user: USER }]);
    expect(calls.filter(call => call.path === '/api/v1/auth/refresh')).toHaveLength(1);
    expect(calls.filter(call => call.authorization === 'Bearer access-2')).toHaveLength(2);
  });

  it('clears the session when the refresh token is rejected', async () => {
    setTokens(tokens('1'));
    const reasons: string[] = [];
    subscribeTokens((_tokens, reason) => reasons.push(reason));
    const calls = installFetchStub(() => unauthorized());

    const error = await getCurrentUser().catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(ApiRequestError);
    expect((error as ApiRequestError).status).toBe(401);
    expect(calls.map(call => call.path)).toEqual(['/api/v1/me', '/api/v1/auth/refresh']);
    expect(getTokens()).toBeNull();
    expect(reasons).toEqual(['expired']);
    expect(Keychain.resetGenericPassword).toHaveBeenCalled();
  });

  it('keeps the session when the refresh fails because the network is down', async () => {
    setTokens(tokens('1'));
    installFetchStub(request => {
      if (request.path === '/api/v1/auth/refresh') {
        throw new TypeError('Network request failed');
      }
      return unauthorized();
    });

    await expect(getCurrentUser()).rejects.toBeInstanceOf(ApiRequestError);
    expect(getTokens()).toEqual(tokens('1'));
    expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
  });

  it('replaces a stale Authorization header passed by the caller', async () => {
    setTokens(tokens('2'));
    const calls = installFetchStub(() => jsonResponse(200, { user: USER }));

    await getCurrentUser('stale-access-token');

    expect(calls[0].authorization).toBe('Bearer access-2');
  });

  it('leaves /auth endpoints and signed-out requests alone', async () => {
    const calls = installFetchStub(request =>
      request.path === '/api/v1/me'
        ? jsonResponse(200, { user: USER })
        : new Response(null, { status: 204 }),
    );

    // Signed out (e.g. mock mode): the caller's header is kept.
    await getCurrentUser('caller-token');
    // Signed in, but /auth/* is excluded: the caller's header is kept.
    setTokens(tokens('1'));
    await atlasApiClient.POST('/api/v1/auth/logout', {
      body: { refreshToken: 'refresh-1' },
      headers: { Authorization: 'Bearer caller-token' },
    });

    expect(calls.map(call => `${call.path} ${call.authorization}`)).toEqual([
      '/api/v1/me Bearer caller-token',
      '/api/v1/auth/logout Bearer caller-token',
    ]);
  });

  it('refreshes before sending when the JWT has already expired', async () => {
    const nowSeconds = Math.floor(Date.now() / 1000);
    setTokens(tokens('1', jwtWithExp(nowSeconds - 60, 'old')));
    const fresh = tokens('2', jwtWithExp(nowSeconds + 900, 'new'));
    const calls = installFetchStub(request =>
      request.path === '/api/v1/auth/refresh'
        ? jsonResponse(200, fresh)
        : jsonResponse(200, { user: USER }),
    );

    await expect(getValidAccessToken()).resolves.toBe(fresh.accessToken);
    await getCurrentUser();

    expect(calls.map(call => call.path)).toEqual(['/api/v1/auth/refresh', '/api/v1/me']);
    expect(calls[1].authorization).toBe(`Bearer ${fresh.accessToken}`);
  });
});
