import { randomToken } from './security.mjs';
import { normalizeDomain } from './signup.mjs';

export function configuration(env = process.env) {
  const origin = new URL(env.PUBLIC_ORIGIN || 'http://127.0.0.1:5173');
  const production = env.NODE_ENV === 'production';
  if (production && (origin.protocol !== 'https:' || !env.DATABASE_URL || !env.RATE_LIMIT_SECRET || !env.GATEWAY_SHARED_SECRET)) throw Error('Production requires HTTPS, DATABASE_URL, RATE_LIMIT_SECRET and GATEWAY_SHARED_SECRET');
  const jmapURL = env.MAIL_JMAP_URL || '';
  if (jmapURL && new URL(jmapURL).protocol !== 'https:' && !(env.NODE_ENV === 'test' && new URL(jmapURL).hostname === '127.0.0.1')) throw Error('JMAP requires HTTPS');
  const mailKey = env.MAIL_ENCRYPTION_KEY ? Buffer.from(env.MAIL_ENCRYPTION_KEY, 'base64') : null;
  if (mailKey && mailKey.length !== 32) throw Error('MAIL_ENCRYPTION_KEY must be 32 bytes in base64');
  const searchProvider = env.SEARCH_PROVIDER || 'opensearch';
  const corpusKind=env.SEARCH_CORPUS_KIND||'development';
  if(!['development','approved-web'].includes(corpusKind))throw Error('Invalid search corpus kind');
  if(corpusKind==='approved-web'&&env.SEARCH_CORPUS_REVIEWED!=='true')throw Error('Approved web corpus requires an operator rights/provenance review');
  if (searchProvider === 'bing') throw Error('The public Bing Search API is retired; configure a supported search provider');
  const validProviders = ['opensearch', 'local', 'brave', 'hybrid'];
  if (!validProviders.includes(searchProvider)) throw Error('Invalid search provider');
  const searchOptions = {
    searchProvider,
    corpusKind,
    braveKey: env.BRAVE_SEARCH_API_KEY,
    braveApproved: env.BRAVE_API_TERMS_APPROVED === 'true',
    braveDailyLimit: Number(env.BRAVE_DAILY_REQUEST_LIMIT || 1000),
    braveRps: Number(env.BRAVE_REQUESTS_PER_SECOND || 1),
  };
  if (!Number.isInteger(searchOptions.braveDailyLimit) || searchOptions.braveDailyLimit < 1 || !Number.isInteger(searchOptions.braveRps) || searchOptions.braveRps < 1) throw Error('Invalid search provider limits');
  if (searchOptions.searchProvider === 'brave' && (!searchOptions.braveKey || !searchOptions.braveApproved)) {
    if (!searchOptions.braveKey) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=brave configured but BRAVE_SEARCH_API_KEY is missing');
    if (!searchOptions.braveApproved) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=brave configured but BRAVE_API_TERMS_APPROVED is not true');
    if (production) throw Error('Production SEARCH_PROVIDER=brave requires BRAVE_SEARCH_API_KEY and BRAVE_API_TERMS_APPROVED=true');
  }
  // Hosted mailbox domain for username@domain signup. Non-production defaults to the reserved .test TLD.
  const mailDomain = env.MAIL_DOMAIN ? normalizeDomain(env.MAIL_DOMAIN) : (production ? null : 'openscanner.test');
  const smtpFrom = env.SMTP_FROM || (env.SMTP_FROM_EMAIL ? `${env.SMTP_FROM_NAME || 'OpenScanner'} <${env.SMTP_FROM_EMAIL}>` : undefined);
  return { ...searchOptions, mailDomain, origin: origin.origin, production, env: env.NODE_ENV, ingressSecret: env.GATEWAY_SHARED_SECRET, databaseURL: env.DATABASE_URL, port: Number(env.GATEWAY_PORT || 8090), host: env.GATEWAY_HOST || '127.0.0.1', searchURL: env.SEARCH_BACKEND_URL || 'http://127.0.0.1:8080', smtpMode: env.SMTP_MODE || 'auto', smtpHost: env.SMTP_HOST, smtpPort: Number(env.SMTP_PORT || 587), smtpUser: env.SMTP_USER, smtpPassword: env.SMTP_PASSWORD || env.SMTP_PASS, smtpFrom, smtpFromName: env.SMTP_FROM_NAME || 'OpenScanner', smtpFromEmail: env.SMTP_FROM_EMAIL || 'no-reply@openscanner.com', smtpReplyTo: env.SMTP_REPLY_TO || 'support@openscanner.com', smtpPoolEnabled: env.SMTP_POOL_ENABLED === 'true', smtpConnectionTimeoutMs: Number(env.SMTP_CONNECTION_TIMEOUT_MS || 10000), smtpSecure: env.SMTP_SECURE === 'true' || Number(env.SMTP_PORT) === 465, jmapURL, mailKey, clamHost: env.CLAMAV_HOST, clamPort: Number(env.CLAMAV_PORT || 3310), rateSecret: env.RATE_LIMIT_SECRET || randomToken() };
}
