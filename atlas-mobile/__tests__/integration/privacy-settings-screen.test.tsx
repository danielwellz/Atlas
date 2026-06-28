import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ReactTestRenderer from 'react-test-renderer';
import { PrivacySettingsScreen } from '../../src/screens/settings/PrivacySettingsScreen';
import type { components } from '../../src/api/generated/openapi';
import { fetchConsents, grantConsent, revokeConsent } from '../../src/api/services/consentService';

const mockNavigate = jest.fn();

jest.mock('@react-navigation/native', () => ({
  useNavigation: () => ({
    navigate: mockNavigate,
  }),
}));

jest.mock('../../src/api/services/consentService', () => ({
  fetchConsents: jest.fn(),
  grantConsent: jest.fn(),
  revokeConsent: jest.fn(),
  PRIVACY_CONSENT_TYPES: [
    'product_analytics',
    'movement_screen_camera',
    'form_check_local',
    'form_check_upload',
  ],
}));

jest.mock('../../src/state/AuthContext', () => ({
  useAuth: () => ({
    session: {
      user: {
        id: 'user-1',
        email: 'athlete@atlas.local',
        createdAt: '2026-01-01T00:00:00.000Z',
      },
      tokens: {
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        tokenType: 'Bearer',
        expiresIn: 900,
      },
    },
  }),
}));

jest.mock('../../src/state/MockModeContext', () => ({
  useMockMode: () => ({
    isMockMode: false,
  }),
}));

type Consent = components['schemas']['Consent'];
type ConsentType = components['schemas']['ConsentType'];

function buildConsent(consentType: ConsentType, revokedAt: string | null = null): Consent {
  return {
    id: `consent-${consentType}`,
    consentType,
    grantedAt: '2026-01-01T00:00:00.000Z',
    revokedAt,
    metadataJson: {
      source: 'test',
    },
  };
}

describe('PrivacySettingsScreen integration', () => {
  const mockedFetchConsents = fetchConsents as jest.MockedFunction<typeof fetchConsents>;
  const mockedGrantConsent = grantConsent as jest.MockedFunction<typeof grantConsent>;
  const mockedRevokeConsent = revokeConsent as jest.MockedFunction<typeof revokeConsent>;

  beforeEach(() => {
    jest.clearAllMocks();
    mockNavigate.mockReset();
    mockedFetchConsents.mockResolvedValue([
      buildConsent('movement_screen_camera'),
      buildConsent('form_check_local', '2026-01-01T00:00:00.000Z'),
    ]);
    mockedGrantConsent.mockResolvedValue(buildConsent('form_check_local'));
    mockedRevokeConsent.mockResolvedValue(buildConsent('movement_screen_camera', '2026-01-02T00:00:00.000Z'));
  });

  it('renders toggles for privacy consent items', async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <PrivacySettingsScreen />
        </QueryClientProvider>,
      );
    });

    await ReactTestRenderer.act(async () => {
      await new Promise<void>(resolve => {
        setTimeout(() => {
          resolve();
        }, 0);
      });
    });

    expect(renderer!.root.findByProps({ testID: 'consent-toggle-movement_screen_camera' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'consent-toggle-form_check_local' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'consent-toggle-form_check_upload' })).toBeTruthy();
    expect(renderer!.root.findByProps({ testID: 'consent-toggle-product_analytics' })).toBeTruthy();
    expect(
      renderer!.root.findByProps({ testID: 'consent-toggle-form_check_upload' }).props.disabled,
    ).toBe(true);

    await ReactTestRenderer.act(async () => {
      renderer!.root.findByProps({ testID: 'consent-upgrade-form_check_upload' }).props.onPress();
    });
    expect(mockNavigate).toHaveBeenCalledWith('Paywall', {
      feature: 'form_check_upload',
    });

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });

  it('calls grant and revoke APIs when toggles change', async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    let renderer: ReactTestRenderer.ReactTestRenderer;

    await ReactTestRenderer.act(async () => {
      renderer = ReactTestRenderer.create(
        <QueryClientProvider client={queryClient}>
          <PrivacySettingsScreen />
        </QueryClientProvider>,
      );
    });

    await ReactTestRenderer.act(async () => {
      await new Promise<void>(resolve => {
        setTimeout(() => {
          resolve();
        }, 0);
      });
    });

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'consent-toggle-form_check_local' })
        .props.onValueChange(true);
    });

    expect(mockedGrantConsent).toHaveBeenCalledWith(
      {
        accessToken: 'access-token',
        consentType: 'form_check_local',
        metadataJson: {
          source: 'privacy_settings_screen',
        },
      },
      false,
    );

    await ReactTestRenderer.act(async () => {
      renderer!.root
        .findByProps({ testID: 'consent-toggle-movement_screen_camera' })
        .props.onValueChange(false);
    });

    expect(mockedRevokeConsent).toHaveBeenCalledWith(
      {
        accessToken: 'access-token',
        consentType: 'movement_screen_camera',
      },
      false,
    );

    await ReactTestRenderer.act(async () => {
      renderer!.unmount();
    });
    queryClient.clear();
  });
});
