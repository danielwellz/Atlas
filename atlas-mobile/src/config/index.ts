import { Platform } from 'react-native';
import { resolveAppConfig } from './appConfig';
import { GENERATED_ENV } from './env.generated';

export type { AppConfig, AtlasEnvName } from './appConfig';

/**
 * Build-time environment config. Evaluated on first import (index.js imports it
 * before App), so a misconfigured build fails at launch.
 * See "Environments" in atlas-mobile/README.md.
 */
export const appConfig = resolveAppConfig({
  generated: GENERATED_ENV,
  platform: Platform.OS,
  isDev: __DEV__,
});
