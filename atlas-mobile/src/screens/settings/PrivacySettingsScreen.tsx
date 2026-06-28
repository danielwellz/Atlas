import React, { useMemo } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Switch, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import { useNavigation } from '@react-navigation/native';
import {
  fetchConsents,
  grantConsent,
  PRIVACY_CONSENT_TYPES,
  revokeConsent,
  type PrivacyConsentType,
} from '../../api/services/consentService';
import { hasFormCheckUploadEntitlement } from '../../features/entitlements';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { Button, Card } from '../../ui';

type ConsentItem = {
  consentType: PrivacyConsentType;
  label: string;
  description: string;
  requiresUploadEntitlement?: boolean;
  requiresConsentType?: PrivacyConsentType;
};

const CONSENT_ITEMS: ConsentItem[] = [
  {
    consentType: 'product_analytics',
    label: 'Product Analytics',
    description: 'Allow anonymous and signed-in product telemetry for app improvements.',
  },
  {
    consentType: 'movement_screen_camera',
    label: 'Movement Screen Camera',
    description: 'Allow camera access for onboarding movement screening.',
  },
  {
    consentType: 'form_check_local',
    label: 'Form Check Local',
    description: 'Allow on-device video form analysis without upload.',
  },
  {
    consentType: 'form_check_upload',
    label: 'Form Check Upload',
    description: 'Allow uploading form-check video for server or coach review.',
    requiresUploadEntitlement: true,
    requiresConsentType: 'form_check_local',
  },
];

export function PrivacySettingsScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const queryClient = useQueryClient();

  const consentsQuery = useQuery({
    queryKey: ['privacy-consents', session?.user.id, isMockMode],
    queryFn: () => fetchConsents(session!.tokens.accessToken, isMockMode),
    enabled: Boolean(session?.tokens.accessToken),
  });

  const toggleConsentMutation = useMutation({
    mutationFn: async ({
      consentType,
      isEnabled,
    }: {
      consentType: PrivacyConsentType;
      isEnabled: boolean;
    }) => {
      if (!session?.tokens.accessToken) {
        throw new Error('Missing authentication token.');
      }

      if (isEnabled) {
        return grantConsent(
          {
            accessToken: session.tokens.accessToken,
            consentType,
            metadataJson: {
              source: 'privacy_settings_screen',
            },
          },
          isMockMode,
        );
      }

      return revokeConsent(
        {
          accessToken: session.tokens.accessToken,
          consentType,
        },
        isMockMode,
      );
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['privacy-consents', session?.user.id, isMockMode],
      });
    },
  });

  const enabledConsents = useMemo(() => {
    const set = new Set<PrivacyConsentType>();
    for (const consent of consentsQuery.data ?? []) {
      if (!PRIVACY_CONSENT_TYPES.includes(consent.consentType as PrivacyConsentType)) {
        continue;
      }
      if (!consent.revokedAt) {
        set.add(consent.consentType as PrivacyConsentType);
      }
    }
    return set;
  }, [consentsQuery.data]);

  const uploadEntitled = hasFormCheckUploadEntitlement(session?.user);

  if (!session) {
    return (
      <View style={styles.center} testID="privacy-settings-no-session">
        <Text style={styles.error}>You must be logged in to manage privacy consents.</Text>
      </View>
    );
  }

  if (consentsQuery.isLoading) {
    return (
      <View style={styles.center} testID="privacy-settings-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (consentsQuery.isError) {
    return (
      <View style={styles.center} testID="privacy-settings-error">
        <Text style={styles.error}>Unable to load privacy consents.</Text>
      </View>
    );
  }

  const activeMutationConsentType = toggleConsentMutation.isPending
    ? toggleConsentMutation.variables?.consentType
    : undefined;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="privacy-settings-screen">
      <Text style={styles.title}>Privacy &amp; Consents</Text>
      <Text style={styles.subtitle}>
        Manage analytics, camera, and form-check consent. Form check stays local by default and only
        uploads on explicit action.
      </Text>

      {CONSENT_ITEMS.map(item => {
        const isEnabled = enabledConsents.has(item.consentType);
        const entitlementLocked = item.requiresUploadEntitlement && !uploadEntitled && !isEnabled;
        const dependentConsentMissing = item.requiresConsentType
          ? !enabledConsents.has(item.requiresConsentType) && !isEnabled
          : false;
        const isPending =
          toggleConsentMutation.isPending && activeMutationConsentType === item.consentType;

        return (
          <Card key={item.consentType}>
            <View style={styles.row}>
              <View style={styles.copy}>
                <Text style={styles.label}>{item.label}</Text>
                <Text style={styles.description}>{item.description}</Text>
                {entitlementLocked ? (
                  <>
                    <Text style={styles.lockedCopy}>
                      Requires subscription entitlement to enable uploads.
                    </Text>
                    <Button
                      label="Unlock"
                      variant="secondary"
                      onPress={() => {
                        navigation.navigate('Paywall', {
                          feature: 'form_check_upload',
                        });
                      }}
                      testID={`consent-upgrade-${item.consentType}`}
                    />
                  </>
                ) : null}
                {dependentConsentMissing ? (
                  <Text style={styles.lockedCopy}>
                    Enable Form Check Local consent before enabling uploads.
                  </Text>
                ) : null}
              </View>
              <Switch
                testID={`consent-toggle-${item.consentType}`}
                value={isEnabled}
                disabled={isPending || entitlementLocked || dependentConsentMissing}
                onValueChange={nextValue => {
                  toggleConsentMutation.mutate({
                    consentType: item.consentType,
                    isEnabled: nextValue,
                  });
                }}
                trackColor={{ false: '#cbd5e1', true: '#14b8a6' }}
                thumbColor={isEnabled ? '#0f766e' : '#f1f5f9'}
              />
            </View>
          </Card>
        );
      })}

      {toggleConsentMutation.isError ? (
        <Text style={styles.error}>Unable to update consent. Please try again.</Text>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 12,
    backgroundColor: '#f8fafc',
  },
  center: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 16,
    backgroundColor: '#f8fafc',
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#334155',
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
  },
  copy: {
    flex: 1,
    gap: 4,
  },
  label: {
    fontSize: 17,
    fontWeight: '700',
    color: '#0f172a',
  },
  description: {
    color: '#334155',
  },
  lockedCopy: {
    color: '#a16207',
    fontSize: 12,
    fontWeight: '600',
  },
  error: {
    color: '#b91c1c',
    fontSize: 14,
  },
});
