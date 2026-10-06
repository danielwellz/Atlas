import defaultEnvironments from './environments.json';
import { getApiBaseUrlError, isEnvName, normalizeApiBaseUrl } from './apiBaseUrlRules';

export type AtlasEnvName = 'local' | 'staging' | 'prod';

/** Shape of src/config/env.generated.ts, written by scripts/generate-env.js. */
export type GeneratedEnv = {
  name: string;
  apiBaseUrlOverride: string | null;
};

type PerPlatformValue = string | { default: string; android?: string; ios?: string };

export type EnvironmentDefinitions = Record<AtlasEnvName, { apiBaseUrl: PerPlatformValue }>;

export type AppConfig = {
  envName: AtlasEnvName;
  apiBaseUrl: string;
};

export class AppConfigError extends Error {
  constructor(message: string) {
    super(`[Atlas config] ${message}`);
    this.name = 'AppConfigError';
  }
}

type ResolveAppConfigInput = {
  generated: GeneratedEnv;
  platform: string;
  /** `__DEV__`; release builds (false) must use HTTPS even for the local environment. */
  isDev: boolean;
  environments?: EnvironmentDefinitions;
};

function pickForPlatform(value: PerPlatformValue, platform: string): string {
  if (typeof value === 'string') {
    return value;
  }
  if (platform === 'android' && value.android) {
    return value.android;
  }
  if (platform === 'ios' && value.ios) {
    return value.ios;
  }
  return value.default;
}

/**
 * Resolves the runtime config for the environment baked in at build time.
 * Throws AppConfigError instead of returning a config that would send traffic
 * to a placeholder host or over plain HTTP from a release build.
 */
export function resolveAppConfig({
  generated,
  platform,
  isDev,
  environments = defaultEnvironments as EnvironmentDefinitions,
}: ResolveAppConfigInput): AppConfig {
  const envName = generated.name;
  if (!isEnvName(envName)) {
    throw new AppConfigError(`Unknown environment "${envName}". Expected local, staging, or prod.`);
  }

  const apiBaseUrl =
    generated.apiBaseUrlOverride ?? pickForPlatform(environments[envName].apiBaseUrl, platform);
  const error = getApiBaseUrlError(envName, apiBaseUrl, { requireHttps: !isDev });
  if (error) {
    throw new AppConfigError(error);
  }

  return {
    envName,
    apiBaseUrl: normalizeApiBaseUrl(apiBaseUrl),
  };
}
