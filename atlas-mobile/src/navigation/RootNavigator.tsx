import React from 'react';
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';
import { NavigationContainer } from '@react-navigation/native';
import { AuthNavigator } from './AuthNavigator';
import { MainTabsNavigator } from './MainTabsNavigator';
import { OnboardingNavigator } from './OnboardingNavigator';
import { useAuth } from '../state/AuthContext';
import { useMockMode } from '../state/MockModeContext';
import { useOnboarding } from '../state/OnboardingContext';

function AppLoading(): React.JSX.Element {
  return (
    <View style={styles.loadingContainer} testID="app-loading">
      <ActivityIndicator size="large" color="#0f766e" />
      <Text style={styles.loadingText}>Booting Atlas...</Text>
    </View>
  );
}

export function RootNavigator(): React.JSX.Element {
  const { isAuthenticated, isHydrated: authHydrated } = useAuth();
  const { isHydrated: mockHydrated } = useMockMode();
  const { isHydrated: onboardingHydrated, isOnboardingComplete } = useOnboarding();

  if (!authHydrated || !mockHydrated || !onboardingHydrated) {
    return <AppLoading />;
  }

  return (
    <NavigationContainer>
      {!isAuthenticated ? (
        <AuthNavigator />
      ) : !isOnboardingComplete ? (
        <OnboardingNavigator />
      ) : (
        <MainTabsNavigator />
      )}
    </NavigationContainer>
  );
}

const styles = StyleSheet.create({
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f8fafc',
    gap: 12,
  },
  loadingText: {
    color: '#334155',
    fontSize: 16,
  },
});
