'use strict';

const fs = require('fs');
const path = require('path');

const DEFAULT_CLIENT = Object.freeze({
  name: 'DeepSeek',
  platform: 'android',
  androidApiLevel: '35',
  locale: 'zh_CN',
});

const DEFAULT_BASE_HEADERS = Object.freeze({
  Host: 'chat.deepseek.com',
  Accept: 'application/json',
  'Accept-Language': 'zh-CN,zh;q=0.9',
  'Content-Type': 'application/json',
  'accept-charset': 'UTF-8',
});

const DEFAULT_SKIP_PATTERNS = Object.freeze([
  'quasi_status',
  'elapsed_secs',
  'token_usage',
  'pending_fragment',
  'conversation_mode',
  'fragments/-1/status',
  'fragments/-2/status',
  'fragments/-3/status',
]);

const DEFAULT_SKIP_EXACT_PATHS = Object.freeze([
  'response/search_status',
]);

function asNonEmptyString(value) {
  return typeof value === 'string' && value !== '' ? value : '';
}

function envString(name) {
  return asNonEmptyString(process.env[name] || '').trim();
}

function normalizeClient(raw) {
  const client = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw : {};
  const out = {
    name: asNonEmptyString(client.name) || DEFAULT_CLIENT.name,
    platform: asNonEmptyString(client.platform) || DEFAULT_CLIENT.platform,
    version: asNonEmptyString(client.version),
    androidApiLevel: asNonEmptyString(client.android_api_level) || DEFAULT_CLIENT.androidApiLevel,
    locale: asNonEmptyString(client.locale) || DEFAULT_CLIENT.locale,
  };
  out.name = envString('DS2API_DEEPSEEK_CLIENT_NAME') || out.name;
  out.platform = (envString('DS2API_DEEPSEEK_CLIENT_PLATFORM') || out.platform).toLowerCase();
  out.version = envString('DS2API_DEEPSEEK_CLIENT_VERSION') || out.version;
  out.androidApiLevel = envString('DS2API_DEEPSEEK_ANDROID_API_LEVEL') || out.androidApiLevel;
  out.locale = envString('DS2API_DEEPSEEK_CLIENT_LOCALE') || out.locale;
  return out;
}

function buildBaseHeaders(parsed, client) {
  const rawBaseHeaders = parsed && typeof parsed.base_headers === 'object' && !Array.isArray(parsed.base_headers)
    ? parsed.base_headers
    : {};
  const baseHeaders = { ...DEFAULT_BASE_HEADERS, ...rawBaseHeaders };
  if (client.name && client.version) {
    const androidSuffix = client.platform === 'android' && client.androidApiLevel
      ? ` Android/${client.androidApiLevel}`
      : '';
    baseHeaders['User-Agent'] = `${client.name}/${client.version}${androidSuffix}`;
  }
  if (client.platform) {
    baseHeaders['x-client-platform'] = client.platform;
  }
  if (client.version) {
    baseHeaders['x-client-version'] = client.version;
  }
  if (client.locale) {
    baseHeaders['x-client-locale'] = client.locale;
  }
  const userAgent = envString('DS2API_DEEPSEEK_USER_AGENT');
  if (userAgent) {
    baseHeaders['User-Agent'] = userAgent;
  }
  const acceptLanguage = envString('DS2API_DEEPSEEK_ACCEPT_LANGUAGE');
  if (acceptLanguage) {
    baseHeaders['Accept-Language'] = acceptLanguage;
  }
  const locale = envString('DS2API_DEEPSEEK_CLIENT_LOCALE');
  if (locale) {
    baseHeaders['x-client-locale'] = locale;
  }
  return baseHeaders;
}

function sharedConstantsPaths() {
  return [
    path.resolve(__dirname, '../../deepseek/protocol/constants_shared.json'),
    path.resolve(process.cwd(), 'internal/deepseek/protocol/constants_shared.json'),
  ];
}

function readSharedConstants() {
  try {
    return require('../../deepseek/protocol/constants_shared.json');
  } catch (_err) {
    // Fall through to filesystem candidates for test and local execution variants.
  }
  for (const sharedPath of sharedConstantsPaths()) {
    try {
      const raw = fs.readFileSync(sharedPath, 'utf8');
      return JSON.parse(raw);
    } catch (_err) {
      // Try the next candidate path; fall back to in-file structural defaults below.
    }
  }
  return {};
}

function loadSharedConstants() {
  const parsed = readSharedConstants();
  const client = normalizeClient(parsed && parsed.client);
  const skipPatterns = Array.isArray(parsed && parsed.skip_contains_patterns)
    ? parsed.skip_contains_patterns.filter((v) => typeof v === 'string' && v !== '')
    : [...DEFAULT_SKIP_PATTERNS];
  const skipExactPaths = Array.isArray(parsed && parsed.skip_exact_paths)
    ? parsed.skip_exact_paths.filter((v) => typeof v === 'string' && v !== '')
    : [...DEFAULT_SKIP_EXACT_PATHS];
  return {
    client,
    baseHeaders: buildBaseHeaders(parsed, client),
    skipPatterns,
    skipExactPaths,
  };
}

const shared = loadSharedConstants();

module.exports = {
  CLIENT: Object.freeze({ ...shared.client }),
  CLIENT_VERSION: shared.client.version,
  BASE_HEADERS: Object.freeze(shared.baseHeaders),
  SKIP_PATTERNS: Object.freeze(shared.skipPatterns),
  SKIP_EXACT_PATHS: new Set(shared.skipExactPaths),
  __test: {
    buildBaseHeaders,
    normalizeClient,
    sharedConstantsPaths,
  },
};
