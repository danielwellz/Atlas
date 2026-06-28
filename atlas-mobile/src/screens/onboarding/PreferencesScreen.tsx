import React from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import type { OnboardingStackParamList } from '../../navigation/types';

const MODALITIES = [
  'barbell_strength',
  'dumbbell_hypertrophy',
  'bodyweight_training',
  'interval_conditioning',
  'mobility_focus',
];

const HISTORY_OPTIONS: Array<{ id: string; label: string; yearsConsistent: number }> = [
  { id: 'none', label: 'No consistent history', yearsConsistent: 0 },
  { id: 'some', label: '1-2 years consistent', yearsConsistent: 2 },
  { id: 'advanced', label: '3+ years consistent', yearsConsistent: 4 },
];

type Props = NativeStackScreenProps<OnboardingStackParamList, 'Preferences'>;

function toLabel(value: string): string {
  return value
    .split('_')
    .map(part => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

export function PreferencesScreen({ navigation }: Props): React.JSX.Element {
  const { profile, toggleModalityPreference, setPriorTrainingHistory } = useOnboarding();
  const selectedYears =
    typeof profile.priorTrainingHistory?.yearsConsistent === 'number'
      ? profile.priorTrainingHistory.yearsConsistent
      : null;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-preferences-screen">
      <Text style={styles.title}>Modality preferences</Text>
      <Text style={styles.subtitle}>Choose the training styles you enjoy and can sustain.</Text>

      <View style={styles.options}>
        {MODALITIES.map(item => {
          const selected = profile.modalityPreferences.includes(item);

          return (
            <Card key={item} style={[styles.option, selected ? styles.optionSelected : undefined]}>
              <Text style={styles.optionText}>{toLabel(item)}</Text>
              <Button
                label={selected ? 'Added' : 'Add'}
                variant={selected ? 'secondary' : 'primary'}
                onPress={() => toggleModalityPreference(item)}
                testID={`preferences-option-${item}`}
              />
            </Card>
          );
        })}
      </View>

      <Card>
        <Text style={styles.sectionTitle}>Prior training history (optional)</Text>
        <Text style={styles.sectionSubtitle}>
          This helps tune your opening session difficulty.
        </Text>
        <View style={styles.historyOptions}>
          {HISTORY_OPTIONS.map(option => {
            const selected = selectedYears === option.yearsConsistent;
            return (
              <Button
                key={option.id}
                label={option.label}
                variant={selected ? 'primary' : 'secondary'}
                onPress={() =>
                  setPriorTrainingHistory({
                    yearsConsistent: option.yearsConsistent,
                    source: option.id,
                  })
                }
                testID={`history-option-${option.id}`}
              />
            );
          })}
          <Button
            label="Clear history"
            variant="secondary"
            onPress={() => setPriorTrainingHistory(null)}
            testID="history-clear-button"
          />
        </View>
      </Card>

      <View style={styles.footer}>
        <Button
          label="Back"
          variant="secondary"
          onPress={() => navigation.navigate('Limitations')}
          testID="preferences-back-button"
        />
        <Button
          label="Next: Schedule"
          onPress={() => navigation.navigate('Schedule')}
          disabled={profile.modalityPreferences.length === 0}
          testID="preferences-next-button"
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
  sectionTitle: {
    fontSize: 16,
    color: '#0f172a',
    fontWeight: '600',
    marginBottom: 4,
  },
  sectionSubtitle: {
    color: '#475569',
    marginBottom: 10,
  },
  historyOptions: {
    gap: 8,
  },
  footer: {
    gap: 10,
  },
});
