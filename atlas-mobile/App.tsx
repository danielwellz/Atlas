import React from 'react';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { AppProviders } from './src/app/AppProviders';
import { RootNavigator } from './src/navigation/RootNavigator';

function App(): React.JSX.Element {
  return (
    <SafeAreaProvider>
      <AppProviders>
        <RootNavigator />
      </AppProviders>
    </SafeAreaProvider>
  );
}

export default App;
