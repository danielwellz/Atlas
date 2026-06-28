import React, { useState } from 'react';
import { Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import type { OnboardingStackParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { trackProductEvent } from '../../analytics/eventClient';

const EQUIPMENT = ['Bodyweight only', 'Dumbbells', 'Kettlebells', 'Resistance bands', 'Barbell setup'];

type Props = NativeStackScreenProps<OnboardingStackParamList, 'Equipment'>;

export function EquipmentScreen({ navigation }: Props): React.JSX.Element {
  const { profile, toggleEquipment } = useOnboarding();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const [saveError, setSaveError] = useState<string | null>(null);

  const goToLimitations = async () => {
    setSaveError(null);

    try {
      await trackProductEvent({
        accessToken: session?.tokens.accessToken,
        eventName: 'onboarding_equipment_selected',
        consentGranted: true,
        useMockMode: isMockMode,
        properties: {
          equipment_count: profile.equipment.length,
          source: 'equipment_screen',
          platform: Platform.OS,
          app_version: '0.0.1',
        },
      });
      navigation.navigate('Limitations');
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : 'Unable to save onboarding selections.');
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-equipment-screen">
      <Text style={styles.title}>Select your equipment</Text>
      <Text style={styles.subtitle}>Choose all that are available to you.</Text>

      <View style={styles.options}>
        {EQUIPMENT.map(item => {
          const selected = profile.equipment.includes(item);

          return (
            <Card key={item} style={[styles.option, selected ? styles.optionSelected : undefined]}>
              <Text style={styles.optionText}>{item}</Text>
              <Button
                label={selected ? 'Added' : 'Add'}
                variant={selected ? 'secondary' : 'primary'}
                onPress={() => {
                  setSaveError(null);
                  toggleEquipment(item);
                }}
                testID={`equipment-option-${item}`}
              />
            </Card>
          );
        })}
      </View>

      {saveError ? <Text style={styles.error}>{saveError}</Text> : null}
      <View style={styles.footer}>
        <Button
          label="Back"
          variant="secondary"
          onPress={() => navigation.navigate('Goals')}
          testID="equipment-back-button"
        />
        <Button
          label="Next: Limitations"
          onPress={() => {
            goToLimitations().catch(() => {});
          }}
          disabled={profile.equipment.length === 0}
          testID="equipment-next-button"
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
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
});
