import type { operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type LoginRequest = operations['PostAuthLogin']['requestBody']['content']['application/json'];
type RegisterRequest = operations['PostAuthRegister']['requestBody']['content']['application/json'];
type LogoutRequest = operations['PostAuthLogout']['requestBody']['content']['application/json'];
type RefreshRequest = operations['PostAuthRefresh']['requestBody']['content']['application/json'];
type TokenResponse = operations['PostAuthRefresh']['responses'][200]['content']['application/json'];
type AuthResponse = operations['PostAuthLogin']['responses'][200]['content']['application/json'];
type UserResponse = operations['GetMe']['responses'][200]['content']['application/json'];

/** The API answered with a non-2xx status (as opposed to a network failure). */
export class ApiRequestError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
  }
}

/** The API refused the refresh token (expired, revoked, or malformed): the session is over. */
export class RefreshRejectedError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'RefreshRejectedError';
  }
}

type LogoutInput = {
  refreshToken: string;
  accessToken: string;
};

function createMockAuthResponse(email: string): AuthResponse {
  const now = new Date().toISOString();

  return {
    user: {
      id: `mock-${email}`,
      email,
      isPro: true,
      entitlements: [
        'barcode_scan',
        'deep_nutrition',
        'biomechanics_overlays',
        'form_check_upload',
        'coach_tier_pro',
      ],
      coachTier: 'pro',
      createdAt: now,
    },
    tokens: {
      accessToken: 'mock-access-token',
      refreshToken: 'mock-refresh-token',
      tokenType: 'Bearer',
      expiresIn: 900,
    },
  };
}

export async function loginUser(
  payload: LoginRequest,
  useMockMode: boolean,
): Promise<AuthResponse> {
  if (useMockMode) {
    return createMockAuthResponse(payload.email);
  }

  const response = await atlasApiClient.POST('/api/v1/auth/login', {
    body: payload,
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Login failed.'));
  }

  return response.data;
}

export async function registerUser(
  payload: RegisterRequest,
  useMockMode: boolean,
): Promise<AuthResponse> {
  if (useMockMode) {
    return createMockAuthResponse(payload.email);
  }

  const response = await atlasApiClient.POST('/api/v1/auth/register', {
    body: payload,
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Registration failed.'));
  }

  return response.data;
}

/**
 * Loads the signed-in user. The auth middleware supplies (and refreshes) the access
 * token; `accessToken` is only a fallback for callers outside the token manager.
 * Throws ApiRequestError for HTTP errors; network failures propagate from fetch.
 */
export async function getCurrentUser(accessToken?: string): Promise<UserResponse> {
  const response = await atlasApiClient.GET('/api/v1/me', {
    headers: accessToken
      ? {
          Authorization: `Bearer ${accessToken}`,
        }
      : undefined,
  });

  if (!response.data) {
    throw new ApiRequestError(
      getApiErrorMessage(response.error, 'Unable to load user profile.'),
      response.response.status,
    );
  }

  return response.data;
}

/**
 * Exchanges a refresh token for a new token pair. The API rotates refresh tokens,
 * so the old one is revoked once this succeeds. Throws RefreshRejectedError when the
 * refresh token is no longer valid, and a plain Error (or fetch's network error) when
 * the refresh could not be completed for now.
 */
export async function refreshAuthTokens(refreshToken: string): Promise<TokenResponse> {
  const body: RefreshRequest = { refreshToken };
  const response = await atlasApiClient.POST('/api/v1/auth/refresh', {
    body,
  });

  if (response.data) {
    return response.data;
  }

  const message = getApiErrorMessage(response.error, 'Unable to refresh the session.');
  const status = response.response.status;
  if (status === 400 || status === 401) {
    throw new RefreshRejectedError(message);
  }

  throw new ApiRequestError(message, status);
}

export async function logoutUser(input: LogoutInput): Promise<void> {
  const body: LogoutRequest = {
    refreshToken: input.refreshToken,
  };

  const response = await atlasApiClient.POST('/api/v1/auth/logout', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (response.error) {
    throw new Error(getApiErrorMessage(response.error, 'Logout failed.'));
  }
}
