/**
 * @format
 */

// Resolve build-time config first so a misconfigured build fails at launch.
import './src/config';
import 'react-native-gesture-handler';
import { AppRegistry } from 'react-native';
import App from './App';
import { name as appName } from './app.json';

AppRegistry.registerComponent(appName, () => App);
