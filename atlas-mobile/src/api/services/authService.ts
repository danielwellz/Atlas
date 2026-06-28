import type { operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type LoginRequest = operations['PostAuthLogin']['requestBody']['content']['application/json'];
type RegisterRequest = operations['PostAuthRegister']['requestBody']['content']['application/json'];
type LogoutRequest = operations['PostAuthLogout']['requestBody']['content']['application/json'];
type AuthResponse = operations['PostAuthLogin']['responses'][200]['content']['application/json'];
type UserResponse = operations['GetMe']['responses'][200]['content']['application/json'];

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

export async function getCurrentUser(accessToken: string): Promise<UserResponse> {
  const response = await atlasApiClient.GET('/api/v1/me', {
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load user profile.'));
  }

  return response.data;
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
