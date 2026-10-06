import {
  AppConfigError,
  resolveAppConfig,
  type EnvironmentDefinitions,
} from '../../src/config/appConfig';

const environments: EnvironmentDefinitions = {
  local: {
    apiBaseUrl: {
      android: 'http://10.0.2.2:8080',
      ios: 'http://localhost:8080',
      default: 'http://localhost:8080',
    },
  },
  staging: { apiBaseUrl: 'https://api.staging.atlas.test/' },
  prod: { apiBaseUrl: 'https://api.atlas.test' },
};

describe('resolveAppConfig', () => {
  it('uses the per-platform local defaults in debug builds', () => {
    const generated = { name: 'local', apiBaseUrlOverride: null };

    expect(resolveAppConfig({ generated, platform: 'android', isDev: true, environments })).toEqual({
      envName: 'local',
      apiBaseUrl: 'http://10.0.2.2:8080',
    });
    expect(resolveAppConfig({ generated, platform: 'ios', isDev: true, environments }).apiBaseUrl).toBe(
      'http://localhost:8080',
    );
    expect(resolveAppConfig({ generated, platform: 'web', isDev: true, environments }).apiBaseUrl).toBe(
      'http://localhost:8080',
    );
  });

  it('resolves staging and prod to HTTPS and strips trailing slashes', () => {
    expect(
      resolveAppConfig({
        generated: { name: 'staging', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: false,
        environments,
      }),
    ).toEqual({ envName: 'staging', apiBaseUrl: 'https://api.staging.atlas.test' });
    expect(
      resolveAppConfig({
        generated: { name: 'prod', apiBaseUrlOverride: null },
        platform: 'ios',
        isDev: false,
        environments,
      }),
    ).toEqual({ envName: 'prod', apiBaseUrl: 'https://api.atlas.test' });
  });

  it('prefers the build-time override over environments.json', () => {
    const config = resolveAppConfig({
      generated: { name: 'local', apiBaseUrlOverride: 'http://192.168.1.20:8080' },
      platform: 'android',
      isDev: true,
      environments,
    });

    expect(config.apiBaseUrl).toBe('http://192.168.1.20:8080');
  });

  it('rejects an http:// URL for prod', () => {
    expect(() =>
      resolveAppConfig({
        generated: { name: 'prod', apiBaseUrlOverride: 'http://api.atlas.test' },
        platform: 'android',
        isDev: false,
        environments,
      }),
    ).toThrow(AppConfigError);
    expect(() =>
      resolveAppConfig({
        generated: { name: 'prod', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: true,
        environments: { ...environments, prod: { apiBaseUrl: 'http://api.atlas.test' } },
      }),
    ).toThrow(/must use https/);
  });

  it('rejects http:// for staging even in a debug build', () => {
    expect(() =>
      resolveAppConfig({
        generated: { name: 'staging', apiBaseUrlOverride: 'http://api.staging.atlas.test' },
        platform: 'ios',
        isDev: true,
        environments,
      }),
    ).toThrow(/must use https/);
  });

  it('rejects plain HTTP in a release build even for the local environment', () => {
    expect(() =>
      resolveAppConfig({
        generated: { name: 'local', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: false,
        environments,
      }),
    ).toThrow(/must use https/);
  });

  it('rejects TODO(owner) placeholders', () => {
    expect(() =>
      resolveAppConfig({
        generated: { name: 'prod', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: false,
        environments: {
          ...environments,
          prod: { apiBaseUrl: 'TODO(owner): production API base URL' },
        },
      }),
    ).toThrow(/placeholder/);
  });

  it('rejects malformed URLs and unknown environments', () => {
    expect(() =>
      resolveAppConfig({
        generated: { name: 'prod', apiBaseUrlOverride: 'api.atlas.test' },
        platform: 'android',
        isDev: false,
        environments,
      }),
    ).toThrow(/not a valid/);
    expect(() =>
      resolveAppConfig({
        generated: { name: 'production', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: false,
        environments,
      }),
    ).toThrow(/Unknown environment "production"/);
  });

  it('ships staging and prod as placeholders until the owner sets them', () => {
    // Uses the committed src/config/environments.json. When the owner fills in real
    // URLs, flip this expectation to assert they resolve.
    expect(() =>
      resolveAppConfig({
        generated: { name: 'prod', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: false,
      }),
    ).toThrow(/placeholder/);
    expect(
      resolveAppConfig({
        generated: { name: 'local', apiBaseUrlOverride: null },
        platform: 'android',
        isDev: true,
      }).apiBaseUrl,
    ).toBe('http://10.0.2.2:8080');
  });
});
