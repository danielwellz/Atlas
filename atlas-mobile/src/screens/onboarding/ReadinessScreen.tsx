import React from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import type { OnboardingStackParamList } from '../../navigation/types';

type Props = NativeStackScreenProps<OnboardingStackParamList, 'Readiness'>;

const ENERGY_OPTIONS = ['low', 'moderate', 'high'];
const SORENESS_OPTIONS = ['low', 'moderate', 'high'];
const SLEEP_OPTIONS = ['poor', 'okay', 'good'];

export function ReadinessScreen({ navigation }: Props): React.JSX.Element {
  const { profile, setReadinessSignals } = useOnboarding();

  const readinessSignals = profile.readinessSignals ?? {};

  const setSignal = (key: string, value: string) => {
    setReadinessSignals({
      ...readinessSignals,
      [key]: value,
    });
  };

  const continueToSprintEnrollment = (skipReadiness: boolean) => {
    if (skipReadiness) {
      setReadinessSignals(null);
    }

    navigation.navigate('MomentumSprintEnrollment', {
      readinessProvided: !skipReadiness,
    });
  };

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-readiness-screen">
      <Text style={styles.title}>Readiness signals (optional)</Text>
      <Text style={styles.subtitle}>
        Add a quick readiness snapshot now, or skip and update this later.
      </Text>

      <Card>
        <Text style={styles.sectionTitle}>Energy</Text>
        <View style={styles.pillRow}>
          {ENERGY_OPTIONS.map(option => (
            <Button
              key={`energy-${option}`}
              label={option}
              variant={readinessSignals.energy === option ? 'primary' : 'secondary'}
              onPress={() => setSignal('energy', option)}
              testID={`readiness-energy-${option}`}
            />
          ))}
        </View>
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Soreness</Text>
        <View style={styles.pillRow}>
          {SORENESS_OPTIONS.map(option => (
            <Button
              key={`soreness-${option}`}
              label={option}
              variant={readinessSignals.soreness === option ? 'primary' : 'secondary'}
              onPress={() => setSignal('soreness', option)}
              testID={`readiness-soreness-${option}`}
            />
          ))}
        </View>
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Sleep quality</Text>
        <View style={styles.pillRow}>
          {SLEEP_OPTIONS.map(option => (
            <Button
              key={`sleep-${option}`}
              label={option}
              variant={readinessSignals.sleepQuality === option ? 'primary' : 'secondary'}
              onPress={() => setSignal('sleepQuality', option)}
              testID={`readiness-sleep-${option}`}
            />
          ))}
        </View>
      </Card>

      <View style={styles.footer}>
        <Button
          label="Back"
          variant="secondary"
          onPress={() => navigation.navigate('Schedule')}
          testID="readiness-back-button"
        />
        <Button
          label="Skip for now"
          variant="secondary"
          onPress={() => continueToSprintEnrollment(true)}
          testID="readiness-skip-button"
        />
        <Button
          label="Continue"
          onPress={() => continueToSprintEnrollment(false)}
          testID="readiness-finish-button"
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
  pillRow: {
    gap: 8,
  },
  footer: {
    gap: 10,
  },
});
