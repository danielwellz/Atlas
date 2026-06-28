import React from 'react';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { AnatomyScreen } from '../screens/anatomy/AnatomyScreen';
import { PaywallScreen } from '../screens/billing/PaywallScreen';
import { CoachSessionPlayerScreen } from '../screens/community/CoachSessionPlayerScreen';
import { CrewScreen } from '../screens/community/CrewScreen';
import { DashboardScreen } from '../screens/dashboard/DashboardScreen';
import { BarcodeScanScreen } from '../screens/nutrition/BarcodeScanScreen';
import { FoodScreen } from '../screens/nutrition/FoodScreen';
import { MealPlanScreen } from '../screens/nutrition/MealPlanScreen';
import { WeeklyCheckInScreen } from '../screens/nutrition/WeeklyCheckInScreen';
import { ProgramsScreen } from '../screens/programs/ProgramsScreen';
import { PrivacySettingsScreen } from '../screens/settings/PrivacySettingsScreen';
import { FormCheckScreen } from '../screens/workout/FormCheckScreen';
import { WorkoutRunnerScreen } from '../screens/workout/WorkoutRunnerScreen';
import type { MainTabParamList } from './types';

const Tab = createBottomTabNavigator<MainTabParamList>();

export function MainTabsNavigator(): React.JSX.Element {
  return (
    <Tab.Navigator
      screenOptions={{
        headerTitleAlign: 'center',
      }}>
      <Tab.Screen name="Dashboard" component={DashboardScreen} />
      <Tab.Screen name="Crew" component={CrewScreen} />
      <Tab.Screen
        name="Paywall"
        component={PaywallScreen}
        options={{
          title: 'Upgrade',
          tabBarButton: () => null,
          tabBarStyle: { display: 'none' },
        }}
      />
      <Tab.Screen name="Anatomy" component={AnatomyScreen} />
      <Tab.Screen name="Food" component={FoodScreen} />
      <Tab.Screen name="MealPlan" component={MealPlanScreen} options={{ title: 'Meal Plan' }} />
      <Tab.Screen
        name="BarcodeScan"
        component={BarcodeScanScreen}
        options={{
          title: 'Scan Barcode',
          tabBarButton: () => null,
          tabBarStyle: { display: 'none' },
        }}
      />
      <Tab.Screen
        name="WeeklyCheckIn"
        component={WeeklyCheckInScreen}
        options={{ title: 'Weekly Check-In' }}
      />
      <Tab.Screen name="Programs" component={ProgramsScreen} />
      <Tab.Screen name="WorkoutRunner" component={WorkoutRunnerScreen} options={{ title: 'Workout' }} />
      <Tab.Screen
        name="FormCheck"
        component={FormCheckScreen}
        options={{
          title: 'Form Check',
          tabBarButton: () => null,
          tabBarStyle: { display: 'none' },
        }}
      />
      <Tab.Screen
        name="PrivacySettings"
        component={PrivacySettingsScreen}
        options={{ title: 'Privacy' }}
      />
      <Tab.Screen
        name="CoachSessionPlayer"
        component={CoachSessionPlayerScreen}
        options={{
          title: 'Coach Session',
          tabBarButton: () => null,
          tabBarStyle: { display: 'none' },
        }}
      />
    </Tab.Navigator>
  );
}
