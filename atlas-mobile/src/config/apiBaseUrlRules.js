/**
 * API base URL rules, shared by the app at startup (src/config/appConfig.ts)
 * and by the build-time generator (scripts/generate-env.js), so a bad URL
 * fails the build and, if it slips through, fails the app on launch.
 *
 * Plain CommonJS so Node can require it without a TypeScript toolchain.
 */

const ENV_NAMES = ['local', 'staging', 'prod'];

const PLACEHOLDER_PATTERN = /TODO/i;
const URL_PATTERN = /^(https?):\/\/[^\s/?#@]+(\/[^\s?#]*)?$/;

/**
 * @param {unknown} name
 * @returns {name is 'local' | 'staging' | 'prod'}
 */
function isEnvName(name) {
  return typeof name === 'string' && ENV_NAMES.includes(name);
}

/**
 * Returns why `url` is not an acceptable API base URL for `envName`, or null if it is.
 * Every environment except `local` must use HTTPS; `requireHttps` extends that to
 * `local` (used for release builds, which must never talk plain HTTP).
 *
 * @param {string} envName
 * @param {unknown} url
 * @param {{ requireHttps?: boolean }} [options]
 * @returns {string | null}
 */
function getApiBaseUrlError(envName, url, options = {}) {
  if (typeof url !== 'string' || url.trim() === '') {
    return `API base URL for "${envName}" is empty.`;
  }
  if (PLACEHOLDER_PATTERN.test(url)) {
    return `API base URL for "${envName}" is still a placeholder (${url}). Set it in src/config/environments.json or via ATLAS_API_BASE_URL.`;
  }
  const match = URL_PATTERN.exec(url);
  if (!match) {
    return `API base URL for "${envName}" is not a valid http(s) URL without query or fragment: ${url}`;
  }
  const requireHttps = envName !== 'local' || options.requireHttps === true;
  if (requireHttps && match[1] !== 'https') {
    return `API base URL for "${envName}" must use https:// (got ${url}).`;
  }
  return null;
}

/**
 * @param {string} url
 * @returns {string}
 */
function normalizeApiBaseUrl(url) {
  return url.replace(/\/+$/, '');
}

module.exports = {
  ENV_NAMES,
  isEnvName,
  getApiBaseUrlError,
  normalizeApiBaseUrl,
};
