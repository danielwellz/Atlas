import React from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import type { OnboardingStackParamList } from '../../navigation/types';

const LIMITATIONS = [
  'none',
  'lower_back_sensitivity',
  'knee_sensitivity',
  'shoulder_sensitivity',
  'limited_overhead_mobility',
];

type Props = NativeStackScreenProps<OnboardingStackParamList, 'Limitations'>;

function toLabel(value: string): string {
  return value
    .split('_')
    .map(part => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

export function LimitationsScreen({ navigation }: Props): React.JSX.Element {
  const { profile, toggleInjuryLimitation } = useOnboarding();

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-limitations-screen">
      <Text style={styles.title}>Injuries & limitations</Text>
      <Text style={styles.subtitle}>Select anything we should account for.</Text>

      <View style={styles.options}>
        {LIMITATIONS.map(item => {
          const selected = profile.injuriesLimitations.includes(item);

          return (
            <Card key={item} style={[styles.option, selected ? styles.optionSelected : undefined]}>
              <Text style={styles.optionText}>{toLabel(item)}</Text>
              <Button
                label={selected ? 'Added' : 'Add'}
                variant={selected ? 'secondary' : 'primary'}
                onPress={() => toggleInjuryLimitation(item)}
                testID={`limitations-option-${item}`}
              />
            </Card>
          );
        })}
      </View>

      <View style={styles.footer}>
        <Button
          label="Back"
          variant="secondary"
          onPress={() => navigation.navigate('Equipment')}
          testID="limitations-back-button"
        />
        <Button
          label="Next: Preferences"
          onPress={() => navigation.navigate('Preferences')}
          disabled={profile.injuriesLimitations.length === 0}
          testID="limitations-next-button"
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
  footer: {
    gap: 10,
  },
});
