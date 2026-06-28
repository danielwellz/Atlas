import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { components } from '../api/generated/openapi';
import { getCurrentUser, logoutUser } from '../api/services/authService';
import { clearTokens, loadTokens, saveTokens } from '../storage/tokenStorage';
import { useMockMode } from './MockModeContext';

type User = components['schemas']['User'];
type TokenResponse = components['schemas']['TokenResponse'];
type AuthResponse = components['schemas']['AuthResponse'];

type AuthSession = {
  user: User;
  tokens: TokenResponse;
};

type AuthContextValue = {
  session: AuthSession | null;
  isAuthenticated: boolean;
  isHydrated: boolean;
  applyAuthResponse: (response: AuthResponse) => Promise<void>;
  refreshCurrentUser: () => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

type AuthProviderProps = {
  children: React.ReactNode;
};

function buildMockUserFromToken(token: TokenResponse): User {
  return {
    id: 'local-mock-user',
    email: 'athlete@atlas.local',
    isPro: true,
    entitlements: [
      'barcode_scan',
      'deep_nutrition',
      'biomechanics_overlays',
      'form_check_upload',
      'coach_tier_pro',
    ],
    coachTier: 'pro',
    createdAt: new Date(Date.now() - token.expiresIn * 1000).toISOString(),
  };
}

export function AuthProvider({ children }: AuthProviderProps): React.JSX.Element {
  const { isMockMode, isHydrated: mockHydrated } = useMockMode();
  const [session, setSession] = useState<AuthSession | null>(null);
  const [isHydrated, setIsHydrated] = useState(false);

  const hydrateSession = useCallback(async () => {
    if (!mockHydrated) {
      return;
    }

    setIsHydrated(false);

    const storedTokens = await loadTokens();

    if (!storedTokens) {
      setSession(null);
      setIsHydrated(true);
      return;
    }

    if (isMockMode) {
      setSession({
        user: buildMockUserFromToken(storedTokens),
        tokens: storedTokens,
      });
      setIsHydrated(true);
      return;
    }

    try {
      const meResponse = await getCurrentUser(storedTokens.accessToken);
      setSession({
        user: meResponse.user,
        tokens: storedTokens,
      });
    } catch {
      await clearTokens();
      setSession(null);
    } finally {
      setIsHydrated(true);
    }
  }, [isMockMode, mockHydrated]);

  useEffect(() => {
    hydrateSession().catch(() => {
      setSession(null);
      setIsHydrated(true);
    });
  }, [hydrateSession]);

  const applyAuthResponse = useCallback(async (response: AuthResponse) => {
    await saveTokens(response.tokens);
    setSession({
      user: response.user,
      tokens: response.tokens,
    });
  }, []);

  const logout = useCallback(async () => {
    if (!isMockMode && session?.tokens.refreshToken && session.tokens.accessToken) {
      try {
        await logoutUser({
          refreshToken: session.tokens.refreshToken,
          accessToken: session.tokens.accessToken,
        });
      } catch {
        // Ignore API logout failures to keep local session cleanup deterministic.
      }
    }

    await clearTokens();
    setSession(null);
  }, [isMockMode, session]);

  const refreshCurrentUser = useCallback(async () => {
    if (!session?.tokens.accessToken || isMockMode) {
      return;
    }

    const meResponse = await getCurrentUser(session.tokens.accessToken);
    setSession(current => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        user: meResponse.user,
      };
    });
  }, [isMockMode, session?.tokens.accessToken]);

  const value = useMemo(
    () => ({
      session,
      isAuthenticated: Boolean(session),
      isHydrated,
      applyAuthResponse,
      refreshCurrentUser,
      logout,
    }),
    [session, isHydrated, applyAuthResponse, refreshCurrentUser, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }

  return context;
}
