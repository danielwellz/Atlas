import React from 'react';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { GoalsScreen } from '../screens/onboarding/GoalsScreen';
import { EquipmentScreen } from '../screens/onboarding/EquipmentScreen';
import { LimitationsScreen } from '../screens/onboarding/LimitationsScreen';
import { PreferencesScreen } from '../screens/onboarding/PreferencesScreen';
import { ScheduleScreen } from '../screens/onboarding/ScheduleScreen';
import { ReadinessScreen } from '../screens/onboarding/ReadinessScreen';
import { MomentumSprintEnrollmentScreen } from '../screens/onboarding/MomentumSprintEnrollmentScreen';
import type { OnboardingStackParamList } from './types';

const Stack = createNativeStackNavigator<OnboardingStackParamList>();

export function OnboardingNavigator(): React.JSX.Element {
  return (
    <Stack.Navigator>
      <Stack.Screen
        name="Goals"
        component={GoalsScreen}
        options={{ title: 'Onboarding: Goals' }}
      />
      <Stack.Screen
        name="Equipment"
        component={EquipmentScreen}
        options={{ title: 'Onboarding: Equipment' }}
      />
      <Stack.Screen
        name="Limitations"
        component={LimitationsScreen}
        options={{ title: 'Onboarding: Limitations' }}
      />
      <Stack.Screen
        name="Preferences"
        component={PreferencesScreen}
        options={{ title: 'Onboarding: Preferences' }}
      />
      <Stack.Screen
        name="Schedule"
        component={ScheduleScreen}
        options={{ title: 'Onboarding: Schedule' }}
      />
      <Stack.Screen
        name="Readiness"
        component={ReadinessScreen}
        options={{ title: 'Onboarding: Readiness' }}
      />
      <Stack.Screen
        name="MomentumSprintEnrollment"
        component={MomentumSprintEnrollmentScreen}
        options={{ title: 'Onboarding: Momentum Sprint' }}
      />
    </Stack.Navigator>
  );
}
