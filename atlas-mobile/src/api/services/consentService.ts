import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type Consent = components['schemas']['Consent'];
type ConsentType = components['schemas']['ConsentType'];
type GrantConsentBody = operations['PostConsentsGrant']['requestBody']['content']['application/json'];
type RevokeConsentBody = operations['PostConsentsRevoke']['requestBody']['content']['application/json'];

export const PRIVACY_CONSENT_TYPES = [
  'movement_screen_camera',
  'form_check_local',
  'form_check_upload',
  'product_analytics',
] as const;

export type PrivacyConsentType = (typeof PRIVACY_CONSENT_TYPES)[number];

type ConsentRequestContext = {
  accessToken: string;
};

type GrantConsentInput = ConsentRequestContext & {
  consentType: PrivacyConsentType;
  metadataJson?: GrantConsentBody['metadataJson'];
};

type RevokeConsentInput = ConsentRequestContext & {
  consentType: PrivacyConsentType;
};

const mockConsents = new Map<PrivacyConsentType, Consent>();
const analyticsConsentCache = new Map<
  string,
  {
    expiresAt: number;
    granted: boolean;
  }
>();
const ANALYTICS_CACHE_TTL_MS = 30_000;

function createMockConsent(consentType: PrivacyConsentType, revokedAt: string | null = null): Consent {
  return {
    id: `mock-consent-${consentType}`,
    consentType: consentType as ConsentType,
    grantedAt: new Date().toISOString(),
    revokedAt,
    metadataJson: {
      source: 'mock',
    },
  };
}

export async function fetchConsents(
  accessToken: string,
  useMockMode: boolean,
): Promise<Consent[]> {
  if (useMockMode) {
    return Array.from(mockConsents.values()).sort((a, b) => a.consentType.localeCompare(b.consentType));
  }

  const response = await atlasApiClient.GET('/api/v1/consents', {
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load consents.'));
  }

  return response.data.consents;
}

export async function hasAnalyticsConsent(
  accessToken: string,
  useMockMode: boolean,
): Promise<boolean> {
  if (!accessToken) {
    return false;
  }

  const cacheKey = `${useMockMode ? 'mock' : 'live'}:${accessToken}`;
  const cached = analyticsConsentCache.get(cacheKey);
  if (cached && cached.expiresAt > Date.now()) {
    return cached.granted;
  }

  const consents = await fetchConsents(accessToken, useMockMode);
  const granted = consents.some(
    consent => consent.consentType === 'product_analytics' && !consent.revokedAt,
  );

  analyticsConsentCache.set(cacheKey, {
    granted,
    expiresAt: Date.now() + ANALYTICS_CACHE_TTL_MS,
  });

  return granted;
}

export async function grantConsent(input: GrantConsentInput, useMockMode: boolean): Promise<Consent> {
  if (useMockMode) {
    const consent = createMockConsent(input.consentType);
    mockConsents.set(input.consentType, consent);
    analyticsConsentCache.clear();
    return consent;
  }

  const body: GrantConsentBody = {
    consentType: input.consentType as ConsentType,
    metadataJson: input.metadataJson,
  };

  const response = await atlasApiClient.POST('/api/v1/consents/grant', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to grant consent.'));
  }

  analyticsConsentCache.clear();
  return response.data.consent;
}

export async function revokeConsent(input: RevokeConsentInput, useMockMode: boolean): Promise<Consent> {
  if (useMockMode) {
    const current = mockConsents.get(input.consentType);
    if (!current || current.revokedAt) {
      throw new Error('Active consent not found');
    }
    const revoked = createMockConsent(input.consentType, new Date().toISOString());
    mockConsents.set(input.consentType, revoked);
    analyticsConsentCache.clear();
    return revoked;
  }

  const body: RevokeConsentBody = {
    consentType: input.consentType as ConsentType,
  };

  const response = await atlasApiClient.POST('/api/v1/consents/revoke', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to revoke consent.'));
  }

  analyticsConsentCache.clear();
  return response.data.consent;
}
