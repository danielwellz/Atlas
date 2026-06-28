import React, { useState } from 'react';
import { Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import { summarizeSchedule } from '../../utils/schedule';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { trackProductEvent } from '../../analytics/eventClient';
import type { OnboardingStackParamList } from '../../navigation/types';

const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

type Props = NativeStackScreenProps<OnboardingStackParamList, 'Schedule'>;

export function ScheduleScreen({ navigation }: Props): React.JSX.Element {
  const { profile, toggleScheduleDay } = useOnboarding();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const [isContinuing, setIsContinuing] = useState(false);
  const [completionError, setCompletionError] = useState<string | null>(null);

  const continueToReadiness = async () => {
    setCompletionError(null);
    setIsContinuing(true);

    try {
      await trackProductEvent({
        accessToken: session?.tokens.accessToken,
        eventName: 'onboarding_schedule_selected',
        consentGranted: true,
        useMockMode: isMockMode,
        properties: {
          days_per_week: profile.scheduleDays.length,
          source: 'schedule_screen',
          platform: Platform.OS,
          app_version: '0.0.1',
        },
      });
      navigation.navigate('Readiness');
    } catch (error) {
      setCompletionError(
        error instanceof Error ? error.message : 'Unable to continue onboarding right now.',
      );
    } finally {
      setIsContinuing(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-schedule-screen">
      <Text style={styles.title}>Pick training days</Text>
      <Text style={styles.subtitle}>Selected: {summarizeSchedule(profile.scheduleDays)}</Text>

      <View style={styles.options}>
        {DAYS.map(day => {
          const selected = profile.scheduleDays.includes(day);

          return (
            <Card key={day} style={[styles.option, selected ? styles.optionSelected : undefined]}>
              <Text style={styles.optionText}>{day}</Text>
              <Button
                label={selected ? 'Selected' : 'Select'}
                variant={selected ? 'secondary' : 'primary'}
                onPress={() => {
                  setCompletionError(null);
                  toggleScheduleDay(day);
                }}
                testID={`schedule-option-${day}`}
              />
            </Card>
          );
        })}
      </View>

      {completionError ? <Text style={styles.error}>{completionError}</Text> : null}
      <View style={styles.footer}>
        <Button
          label="Back"
          variant="secondary"
          onPress={() => navigation.navigate('Preferences')}
          testID="schedule-back-button"
        />
        <Button
          label="Next: Readiness"
          onPress={() => {
            continueToReadiness().catch(() => {});
          }}
          disabled={profile.scheduleDays.length === 0 || isContinuing}
          loading={isContinuing}
          testID="schedule-next-button"
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
  options: {
    gap: 10,
  },
  option: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  optionSelected: {
    borderColor: '#0f766e',
    backgroundColor: '#ecfeff',
  },
  optionText: {
    fontSize: 16,
    color: '#0f172a',
    flex: 1,
    marginRight: 8,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
  footer: {
    gap: 10,
  },
});
