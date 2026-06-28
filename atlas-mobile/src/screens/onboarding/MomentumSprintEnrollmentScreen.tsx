import React, { useState } from 'react';
import { Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import type { OnboardingStackParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { enrollMomentumSprint } from '../../api/services/habitService';
import { trackProductEvent } from '../../analytics/eventClient';

type Props = NativeStackScreenProps<OnboardingStackParamList, 'MomentumSprintEnrollment'>;

function resolveSprintGoal(goal: string | null): string {
  if (!goal || !goal.trim()) {
    return 'general_fitness';
  }
  return goal;
}

export function MomentumSprintEnrollmentScreen({ navigation, route }: Props): React.JSX.Element {
  const { profile, completeOnboarding } = useOnboarding();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const finishOnboarding = async (enrollInSprint: boolean) => {
    setErrorMessage(null);
    setIsSubmitting(true);

    try {
      if (enrollInSprint && session?.tokens.accessToken) {
        await enrollMomentumSprint({
          accessToken: session.tokens.accessToken,
          goal: resolveSprintGoal(profile.goal),
        });
      }

      await completeOnboarding();

      await trackProductEvent({
        accessToken: session?.tokens.accessToken,
        eventName: 'onboarding_completed',
        consentGranted: true,
        useMockMode: isMockMode,
        properties: {
          goal: profile.goal,
          days_per_week: profile.scheduleDays.length,
          equipment_count: profile.equipment.length,
          readiness_provided: route.params.readinessProvided,
          momentum_sprint_enrolled: enrollInSprint,
          source: 'momentum_sprint_enrollment_screen',
          platform: Platform.OS,
          app_version: '0.0.1',
        },
      });
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : 'Unable to complete onboarding right now.',
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-momentum-sprint-screen">
      <Text style={styles.title}>Momentum Sprint (optional)</Text>
      <Text style={styles.subtitle}>
        Start a structured 14-day sprint now to build consistency from day one.
      </Text>

      <Card>
        <Text style={styles.sectionTitle}>What you get</Text>
        <Text style={styles.helperText}>- Goal-tied daily checklist</Text>
        <Text style={styles.helperText}>- Streak and completion tracking</Text>
        <Text style={styles.helperText}>- Reward milestones at days 3, 7, and 14</Text>
      </Card>

      {errorMessage ? <Text style={styles.error}>{errorMessage}</Text> : null}

      <View style={styles.footer}>
        <Button
          label="Back"
          variant="secondary"
          onPress={() => navigation.navigate('Readiness')}
          disabled={isSubmitting}
          testID="momentum-sprint-back-button"
        />
        <Button
          label="Skip for now"
          variant="secondary"
          onPress={() => {
            finishOnboarding(false).catch(() => {});
          }}
          disabled={isSubmitting}
          testID="momentum-sprint-skip-button"
        />
        <Button
          label="Start 14-day sprint"
          onPress={() => {
            finishOnboarding(true).catch(() => {});
          }}
          loading={isSubmitting}
          disabled={isSubmitting}
          testID="momentum-sprint-start-button"
        />
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 20,
    gap: 16,
    backgroundColor: '#f8fafc',
  },
  title: {
    fontSize: 24,
    color: '#0f172a',
    fontWeight: '700',
  },
  subtitle: {
    color: '#475569',
  },
  sectionTitle: {
    fontSize: 16,
    color: '#0f172a',
    fontWeight: '600',
    marginBottom: 8,
  },
  helperText: {
    color: '#334155',
    fontSize: 14,
    marginBottom: 4,
  },
  footer: {
    gap: 10,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
});
