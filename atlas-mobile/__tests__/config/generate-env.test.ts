const {
  parseArgs,
  renderModule,
  resolveGeneratedEnv,
} = require('../../scripts/generate-env');

const environments = {
  local: { apiBaseUrl: { android: 'http://10.0.2.2:8080', default: 'http://localhost:8080' } },
  staging: { apiBaseUrl: 'TODO(owner): staging API base URL' },
  prod: { apiBaseUrl: 'TODO(owner): production API base URL' },
};

describe('scripts/generate-env', () => {
  it('parses --env and --if-missing', () => {
    expect(parseArgs(['--env', 'prod', '--if-missing'])).toEqual({ env: 'prod', ifMissing: true });
    expect(parseArgs(['--env=staging'])).toEqual({ env: 'staging', ifMissing: false });
    expect(() => parseArgs(['--prod'])).toThrow(/Unknown argument/);
  });

  it('defaults to local and lets ATLAS_ENV pick the environment', () => {
    expect(resolveGeneratedEnv({ envArg: undefined, processEnv: {}, environments })).toEqual({
      name: 'local',
      apiBaseUrlOverride: null,
    });
    expect(() =>
      resolveGeneratedEnv({ envArg: undefined, processEnv: { ATLAS_ENV: 'qa' }, environments }),
    ).toThrow(/Unknown ATLAS_ENV "qa"/);
  });

  it('fails a prod build that still has the placeholder URL', () => {
    expect(() => resolveGeneratedEnv({ envArg: 'prod', processEnv: {}, environments })).toThrow(
      /placeholder/,
    );
  });

  it('fails a prod build with an http:// override and accepts https://', () => {
    expect(() =>
      resolveGeneratedEnv({
        envArg: 'prod',
        processEnv: { ATLAS_API_BASE_URL: 'http://api.atlas.test' },
        environments,
      }),
    ).toThrow(/must use https/);
    expect(
      resolveGeneratedEnv({
        envArg: 'prod',
        processEnv: { ATLAS_API_BASE_URL: ' https://api.atlas.test ' },
        environments,
      }),
    ).toEqual({ name: 'prod', apiBaseUrlOverride: 'https://api.atlas.test' });
  });

  it('renders a module the app can import', () => {
    expect(renderModule({ name: 'prod', apiBaseUrlOverride: 'https://api.atlas.test' })).toContain(
      "apiBaseUrlOverride: 'https://api.atlas.test',",
    );
    expect(renderModule({ name: 'local', apiBaseUrlOverride: null })).toContain(
      'apiBaseUrlOverride: null,',
    );
  });
});
