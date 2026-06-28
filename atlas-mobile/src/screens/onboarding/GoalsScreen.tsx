import React, { useEffect, useRef, useState } from 'react';
import { Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { NativeStackScreenProps } from '@react-navigation/native-stack';
import { Button, Card } from '../../ui';
import { useOnboarding } from '../../state/OnboardingContext';
import type { OnboardingStackParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { trackProductEvent } from '../../analytics/eventClient';

const GOALS = ['Build strength', 'Lose fat', 'Improve endurance', 'General fitness'];

type Props = NativeStackScreenProps<OnboardingStackParamList, 'Goals'>;

export function GoalsScreen({ navigation }: Props): React.JSX.Element {
  const { profile, setGoal } = useOnboarding();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const hasLoggedScreenViewRef = useRef(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (hasLoggedScreenViewRef.current) {
      return;
    }

    hasLoggedScreenViewRef.current = true;
    trackProductEvent({
      accessToken: session?.tokens.accessToken,
      eventName: 'onboarding_started',
      consentGranted: true,
      useMockMode: isMockMode,
      properties: {
        entry_point: 'goals_screen',
        platform: Platform.OS,
        app_version: '0.0.1',
      },
    }).catch(() => {});
  }, [isMockMode, session?.tokens.accessToken]);

  const goToEquipment = async () => {
    setSaveError(null);

    try {
      await trackProductEvent({
        accessToken: session?.tokens.accessToken,
        eventName: 'onboarding_goal_selected',
        consentGranted: true,
        useMockMode: isMockMode,
        properties: {
          goal: profile.goal,
          source: 'goals_screen',
          platform: Platform.OS,
          app_version: '0.0.1',
        },
      });
      navigation.navigate('Equipment');
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : 'Unable to save onboarding selections.');
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container} testID="onboarding-goals-screen">
      <Text style={styles.title}>What is your primary goal?</Text>
      <Text style={styles.subtitle}>We will tailor program suggestions around this.</Text>

      <View style={styles.options}>
        {GOALS.map(goal => {
          const selected = profile.goal === goal;

          return (
            <Card key={goal} style={[styles.option, selected ? styles.optionSelected : undefined]}>
              <Text style={styles.optionText}>{goal}</Text>
              <Button
                label={selected ? 'Selected' : 'Select'}
                variant={selected ? 'secondary' : 'primary'}
                onPress={() => {
                  setSaveError(null);
                  setGoal(goal);
                }}
                testID={`goal-option-${goal}`}
              />
            </Card>
          );
        })}
      </View>

      {saveError ? <Text style={styles.error}>{saveError}</Text> : null}
      <Button
        label="Next: Equipment"
        onPress={() => {
          goToEquipment().catch(() => {});
        }}
        disabled={!profile.goal}
        testID="goals-next-button"
      />
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
});
