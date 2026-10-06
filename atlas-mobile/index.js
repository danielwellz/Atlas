/**
 * @format
 */

import 'react-native-gesture-handler';
// Resolve build-time config before the app so a misconfigured build fails at launch.
import './src/config';
import { AppRegistry } from 'react-native';
import App from './App';
import { name as appName } from './app.json';

AppRegistry.registerComponent(appName, () => App);
