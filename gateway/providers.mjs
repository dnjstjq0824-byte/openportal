import { fail } from './mail.mjs';

// Unified search-provider abstraction (RV-01). Every provider normalizes to the
// canonical hit contract {title, url, description, source} (+ optional site_name /
// document_id) so the gateway and UI never branch on provider-specific shapes.
// Error convention: fail(code, status) from mail.mjs (message carries the code).

export async function readSearchJSON(response) {
  if (!response.headers.get('content-type')?.includes('application/json')) throw fail('SEARCH_UNAVAILABLE', 502);
  let length = 0; const chunks = [];
  for await (const chunk of response.body || []) { length += chunk.length; if (length > 2 * 1024 * 1024) throw fail('SEARCH_UNAVAILABLE', 502); chunks.push(chunk); }
  try { const body = JSON.parse(Buffer.concat(chunks).toString('utf8')); if (!body || typeof body !== 'object' || Array.isArray(body)) throw Error(); return body; } catch { throw fail('SEARCH_UNAVAILABLE', 502); }
}

// Canonicalize URL for deduplication and comparison
export function normalizeURL(raw) {
  try {
    const u = new URL(raw);
    let pathname = u.pathname;
    if (pathname.endsWith('/')) {
      pathname = pathname.slice(0, -1);
    }
    return `${u.protocol}//${u.hostname.toLowerCase()}${u.port ? ':' + u.port : ''}${pathname}${u.search}`;
  } catch {
    return String(raw || '').trim().toLowerCase();
  }
}

// Reciprocal Rank Fusion (RRF) rank merger (SE-11)
// score(d) = sum(1 / (k + rank_i(d)))
export function rrfMerge(localHits = [], externalHits = [], k = 60) {
  const map = new Map();
  const seenExternal = new Set();

  // 1. Process local hits
  localHits.forEach((hit, idx) => {
    const rank = idx + 1;
    const key = normalizeURL(hit.url);
    if (map.has(key)) return;
    const score = 1.0 / (k + rank);
    map.set(key, {
      ...hit,
      rrf_score: score,
      local_rank: rank,
      source: hit.source || 'opensearch',
    });
  });

  // 2. Process external hits
  externalHits.forEach((hit, idx) => {
    const rank = idx + 1;
    const key = normalizeURL(hit.url);
    if (seenExternal.has(key)) return;
    seenExternal.add(key);
    const score = 1.0 / (k + rank);
    if (map.has(key)) {
      const existing = map.get(key);
      existing.rrf_score += score;
      existing.external_rank = rank;
      // Retain local metadata (title/desc) if available, or fill if empty
      if (!existing.title && hit.title) existing.title = hit.title;
      if (!existing.description && hit.description) existing.description = hit.description;
    } else {
      map.set(key, {
        ...hit,
        rrf_score: score,
        external_rank: rank,
        source: hit.source || 'external-api',
      });
    }
  });

  // 3. Sort by descending RRF score
  const merged = Array.from(map.values()).sort((a, b) => b.rrf_score - a.rrf_score);
  return merged;
}

// Canonical hit choke point: every adapter output passes here. Rejects
// non-http(s) URLs, embedded credentials, and non-string title/description.
// site_name/document_id pass through only when they are safe strings.
export function normalizeHit(raw, fallbackSource) {
  if (!raw || typeof raw !== 'object') return null;
  if (typeof raw.title !== 'string' || typeof raw.description !== 'string') return null;
  let url;
  try {
    const u = new URL(raw.url);
    if (!['http:', 'https:'].includes(u.protocol) || u.username || u.password) return null;
    url = raw.url;
  } catch {
    return null;
  }
  const hit = { title: raw.title, url, description: raw.description, source: typeof raw.source === 'string' ? raw.source : fallbackSource };
  if (typeof raw.site_name === 'string' && raw.site_name) hit.site_name = raw.site_name.slice(0, 200);
  if (typeof raw.document_id === 'string' && raw.document_id) hit.document_id = raw.document_id.slice(0, 256);
  return hit;
}

export function stripTags(value) {
  return String(value || '').replace(/<[^>]*>/g, '');
}

// --- Resilience primitives (gateway level, per provider) ---

// Retry with exponential backoff + jitter. Retries network throws, 429 and 5xx
// only; 4xx/validation errors fail immediately (never retry a rejected query).
// Configuration errors (missing credentials, unverified spec) also never
// retry: no backoff window will fix a missing key or approval flag.
function isConfigError(error) {
  const code = String(error?.message || '');
  return code === 'SEARCH_NOT_CONFIGURED' || code.startsWith('SPEC_UNVERIFIED');
}
export async function withRetry(operation, { attempts = 3, baseMs = 200, maxMs = 2000, sleep = defaultSleep } = {}) {
  let tried = 0;
  for (;;) {
    tried++;
    try {
      return { value: await operation(), attempts: tried };
    } catch (error) {
      const status = Number(error?.status || 0);
      const retryable = !isConfigError(error) && (status === 0 || status === 429 || status >= 500);
      if (!retryable || tried >= attempts) throw error;
      const backoff = Math.min(maxMs, baseMs * 2 ** (tried - 1));
      await sleep(backoff / 2 + Math.random() * (backoff / 2));
    }
  }
}

function defaultSleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// Minimal circuit breaker: closed -> open after failureThreshold consecutive
// failures; half-open single probe after cooldownMs; any failure re-opens.
export class CircuitBreaker {
  constructor({ failureThreshold = 3, cooldownMs = 30000 } = {}) {
    this.failureThreshold = failureThreshold;
    this.cooldownMs = cooldownMs;
    this.failures = 0;
    this.state = 'closed';
    this.openedAt = 0;
  }
  async exec(operation) {
    if (this.state === 'open') {
      if (Date.now() - this.openedAt < this.cooldownMs) throw fail('PROVIDER_CIRCUIT_OPEN', 503);
      this.state = 'half-open';
    }
    try {
      const value = await operation();
      this.failures = 0;
      this.state = 'closed';
      return { value, opened: false };
    } catch (error) {
      const status = Number(error?.status || 0);
      const counts = !isConfigError(error) && (status === 0 || status === 429 || status >= 500);
      if (counts) {
        this.failures++;
        if (this.state === 'half-open' || this.failures >= this.failureThreshold) {
          this.state = 'open';
          this.openedAt = Date.now();
          return { value: undefined, opened: true, error };
        }
      }
      throw error;
    }
  }
}

const breakers = new Map();
export function breakerFor(provider) {
  if (!breakers.has(provider)) breakers.set(provider, new CircuitBreaker());
  return breakers.get(provider);
}
export function resetBreakers() {
  breakers.clear();
}

// --- Provider registry ---

// SPEC_UNVERIFIED (fail-closed): selectable names whose wire spec is NOT
// confirmed yet. Boot refusal names the exact missing confirmation so the
// operator knows what to verify (W-02/W-03/W-04). Enabling a provider means
// removing it from this map AND implementing its fetchPage below. Never stub
// a wire response: unknown wire shape => refuse, never invent.
export const UNVERIFIED_PROVIDERS = {
  serper: 'W-03 (P-02 page/num/gl/hl/total parameters via playground verification)',
  youcom: 'W-04 (MSA display terms, offset upper bound)',
};

export function assertProviderAvailable(name) {
  if (UNVERIFIED_PROVIDERS[name]) throw fail(`SPEC_UNVERIFIED:${name}:${UNVERIFIED_PROVIDERS[name]}`, 503);
}

function timeoutMs(config) {
  return Number(config.upstreamTimeoutMs || 8000);
}

async function fetchOpenSearchPage(ctx) {
  const { config, q, page, safeSearch, fetcher } = ctx;
  const target = new URL('/api/v1/search', config.searchURL);
  target.search = new URLSearchParams({ q, page: String(page), safe_search: safeSearch }).toString();
  let upstream;
  try {
    upstream = await fetcher(target, { redirect: 'error', signal: AbortSignal.timeout(timeoutMs(config)) });
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  if (!upstream.ok) throw fail(upstream.status === 429 ? 'RATE_LIMITED' : 'SEARCH_UNAVAILABLE', upstream.status === 429 ? 429 : 503);
  const data = await readSearchJSON(upstream);
  if (!Array.isArray(data.hits) || (data.total !== null && !Number.isInteger(data.total))) throw fail('SEARCH_UNAVAILABLE', 502);
  const seen = new Set();
  const hits = [];
  for (const raw of data.hits) {
    const hit = normalizeHit({ ...raw, source: 'opensearch-index' }, 'opensearch-index');
    if (hit && !seen.has(normalizeURL(hit.url))) {
      seen.add(normalizeURL(hit.url));
      hits.push(hit);
    }
  }
  return {
    query: q,
    total: data.total,
    hits,
    hasMore: Boolean(data.has_more ?? (data.total !== null && page * 10 < data.total)),
    source: 'opensearch-index',
    corrected_query: typeof data.corrected_query === 'string' ? data.corrected_query : undefined,
    did_you_mean: typeof data.did_you_mean === 'string' ? data.did_you_mean : undefined,
    // Full validated upstream payload (sponsored_ads, intent, request_id, ...).
    // The gateway spreads it so the owned-index wire shape stays identical;
    // only hits are renormalized to the canonical contract (RV-01).
    upstream: data,
  };
}

async function fetchBravePage(ctx) {
  const { config, q, page, safeSearch, fetcher } = ctx;
  if (!config.braveApproved || !config.braveKey) throw fail('SEARCH_NOT_CONFIGURED', 503);
  if (!Number.isInteger(page) || page < 1 || page > 10) throw fail('INVALID_QUERY');
  const braveSafe = ['strict', 'moderate', 'off'].includes(safeSearch) ? safeSearch : 'moderate';
  const url = new URL('https://api.search.brave.com/res/v1/web/search');
  url.search = new URLSearchParams({
    q,
    count: '10',
    offset: String(page - 1),
    safesearch: braveSafe,
    text_decorations: 'false',
    result_filter: 'web',
  }).toString();
  let response;
  try {
    response = await fetcher(url, {
      headers: { Accept: 'application/json', 'X-Subscription-Token': config.braveKey },
      signal: AbortSignal.timeout(timeoutMs(config)),
      redirect: 'error',
    });
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  if (!response.ok) {
    throw fail(response.status === 429 ? 'RATE_LIMITED' : 'SEARCH_UNAVAILABLE', response.status === 429 ? 429 : 503);
  }
  let body;
  try {
    body = await readSearchJSON(response);
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }
  if (typeof body.query?.original !== 'string' || (body.web && !Array.isArray(body.web.results))) {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }
  const seen = new Set();
  const hits = [];
  for (const raw of body.web?.results || []) {
    const hit = normalizeHit({ title: raw.title, url: raw.url, description: raw.description, source: 'brave-api' }, 'brave-api');
    if (hit && !seen.has(hit.url)) {
      seen.add(hit.url);
      hits.push(hit);
    }
  }
  return {
    query: q,
    total: null,
    hits,
    hasMore: page < 10 && body.query.more_results_available === true,
    source: 'brave-api',
  };
}

// Naver OpenAPI webkr adapter. Wire spec verified against the official docs
// (developers.naver.com/docs/serviceapi/search/web/web.md, fetched 2026-11-24):
// GET https://openapi.naver.com/v1/search/webkr.json with query/display/start
// params (display 1..100 default 10; start 1..1000) and X-Naver-Client-Id +
// X-Naver-Client-Secret headers. sort=sim is sent (valid per the documented
// SE04 sort validation; harmless if ignored for webkr). Errors: SE01-SE04/SE06
// arrive as HTTP 400 (mapped to INVALID_QUERY), 403 means the app lacks search
// permission (configuration fault -> SEARCH_NOT_CONFIGURED), 429 quota,
// SE99 500. Response {lastBuildDate,total,start,display,items[]} with
// items[{title,link,description}] and <b> match markup (stripped here).
// Terms enforcement (R-02-3, R-07-1): display-only use. This adapter performs
// exactly one live fetch per call and persists NOTHING (no cache, no DB, no
// file); gateway responses already carry Cache-Control: no-store. Results must
// never feed AI training/input, resale, or ad-bearing pages without a
// separate Naver partnership review. Enabling requires NAVER_CLIENT_ID +
// NAVER_CLIENT_SECRET + NAVER_API_TERMS_APPROVED=true (fail-fast in config).
async function fetchNaverPage(ctx) {
  const { config, q, page, fetcher } = ctx;
  if (!config.naverApproved || !config.naverClientId || !config.naverClientSecret) throw fail('SEARCH_NOT_CONFIGURED', 503);
  if (!Number.isInteger(page) || page < 1 || page > 100) throw fail('INVALID_QUERY');
  const url = new URL('https://openapi.naver.com/v1/search/webkr.json');
  url.search = new URLSearchParams({
    query: q,
    display: '10',
    start: String((page - 1) * 10 + 1),
    sort: 'sim',
  }).toString();
  let response;
  try {
    response = await fetcher(url, {
      headers: {
        Accept: 'application/json',
        'X-Naver-Client-Id': config.naverClientId,
        'X-Naver-Client-Secret': config.naverClientSecret,
      },
      signal: AbortSignal.timeout(timeoutMs(config)),
      redirect: 'error',
    });
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  if (!response.ok) {
    // Official webkr errors: 400 SE01-SE04/SE06 (bad query/params),
    // 403 app lacks search permission, 429 quota, SE99 500.
    if (response.status === 429) throw fail('RATE_LIMITED', 429);
    if (response.status === 400) throw fail('INVALID_QUERY', 400);
    if (response.status === 403) throw fail('SEARCH_NOT_CONFIGURED', 503);
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  let body;
  try {
    body = await readSearchJSON(response);
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }
  if (!Array.isArray(body.items) || (body.total !== undefined && !Number.isInteger(body.total))) {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }
  const seen = new Set();
  const hits = [];
  for (const raw of body.items) {
    // Naver wraps matched terms in <b> tags; strip markup, never render raw HTML.
    const hit = normalizeHit({ title: stripTags(raw.title), url: raw.link, description: stripTags(raw.description), source: 'naver-api' }, 'naver-api');
    if (hit && !seen.has(hit.url)) {
      seen.add(hit.url);
      hits.push(hit);
    }
  }
  const total = Number.isInteger(body.total) ? body.total : null;
  return {
    query: q,
    total,
    hits,
    hasMore: total !== null ? page * 10 < total : hits.length === 10,
    source: 'naver-api',
  };
}

async function fetchHybridPage(ctx) {
  const { config, q, page, safeSearch, fetcher } = ctx;
  if (!Number.isInteger(page) || page < 1) throw fail('INVALID_QUERY');
  // External legs cap at page 10 (Brave offset limit). Without external
  // credentials hybrid degrades to the local leg and keeps the local range.
  if (page > 10 && config.braveKey && config.braveApproved) throw fail('INVALID_QUERY');
  const localPromise = fetchOpenSearchPage({ ...ctx, page }).catch(() => null);
  let externalPromise = Promise.resolve(null);
  if (config.braveKey && config.braveApproved) {
    externalPromise = fetchBravePage({ ...ctx, page }).catch(() => null);
  }
  const [localRes, externalRes] = await Promise.all([localPromise, externalPromise]);
  if (!localRes && !externalRes) {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  const localHits = (localRes?.hits || []).map(h => ({ ...h, source: 'opensearch-index' }));
  const externalHits = externalRes?.hits || [];
  const mergedHits = rrfMerge(localHits, externalHits, 60);
  // Each provider returns one page. The overlap and the combined corpus size
  // are unknown; a local count cannot describe the union of both providers.
  const externalRequested = Boolean(config.braveKey && config.braveApproved);
  const total = externalRequested ? null : localRes?.total ?? null;
  const hasMore = Boolean((!externalRequested || page < 10) && ((localRes && (localRes.hasMore ?? (Number.isInteger(localRes.total) && page * 10 < localRes.total))) || externalRes?.hasMore));
  return {
    query: q,
    total,
    hits: mergedHits,
    hasMore,
    degraded: !localRes || (externalRequested && !externalRes),
    source: 'hybrid-rrf',
    did_you_mean: localRes?.corrected_query || undefined,
  };
}

function unverifiedFetch(name) {
  return async () => {
    assertProviderAvailable(name);
    throw fail('SEARCH_UNAVAILABLE', 503);
  };
}

// Mojeek Search API adapter. Wire spec verified against the official docs
// (mojeek.com/support/api/search, fetched 2026-11-24):
// GET https://www.mojeek.com/search with api_key/q/s/t/fmt/safe params;
// s is 1-based (1=first, 11=second page at 10/page), t is page size
// (default 10), fmt=json selects JSON. No s/t upper bound is documented, so
// no cap is invented: pages map to s=(p-1)*10+1 and hasMore derives from the
// head.results total. safe=1 for strict/moderate, 0 for off.
// Response: response.status "OK" | "ERROR: <message>", head.results total,
// results[] {url,title,desc,...}. Auth key travels as a query parameter
// (server-side only; gateway error paths never log upstream URLs).
// Terms posture: single live fetch per call, no caching/storage of results
// (V-01-3 index-folding ban); enabling requires MOJEK_API_KEY +
// MOJEK_API_TERMS_APPROVED=true. Re-verify the current Mojeek API terms at
// key issuance: the §1.8.3 boundary below comes from the 2026-11-11
// investigation report, not from a terms page read in this session.
async function fetchMojeekPage(ctx) {
  const { config, q, page, safeSearch, fetcher } = ctx;
  if (!config.mojekApproved || !config.mojekKey) throw fail('SEARCH_NOT_CONFIGURED', 503);
  if (!Number.isInteger(page) || page < 1 || page > 100) throw fail('INVALID_QUERY');
  const url = new URL('https://www.mojeek.com/search');
  url.search = new URLSearchParams({
    api_key: config.mojekKey,
    q,
    s: String((page - 1) * 10 + 1),
    t: '10',
    fmt: 'json',
    safe: safeSearch === 'off' ? '0' : '1',
  }).toString();
  let response;
  try {
    response = await fetcher(url, {
      headers: { Accept: 'application/json' },
      signal: AbortSignal.timeout(timeoutMs(config)),
      redirect: 'error',
    });
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  if (!response.ok) {
    throw fail(response.status === 429 ? 'RATE_LIMITED' : 'SEARCH_UNAVAILABLE', response.status === 429 ? 429 : 503);
  }
  let body;
  try {
    body = await readSearchJSON(response);
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }
  const payload = body.response;
  if (!payload || typeof payload.status !== 'string' || !Array.isArray(payload.results)) {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }
  if (!payload.status.startsWith('OK')) {
    // Documented example: "ERROR: Daily Limit Reached".
    if (payload.status.includes('Daily Limit')) throw fail('RATE_LIMITED', 429);
    throw fail('SEARCH_UNAVAILABLE', 503);
  }
  const seen = new Set();
  const hits = [];
  for (const raw of payload.results) {
    const hit = normalizeHit({ title: raw.title, url: raw.url, description: raw.desc, source: 'mojeek-api' }, 'mojeek-api');
    if (hit && !seen.has(hit.url)) {
      seen.add(hit.url);
      hits.push(hit);
    }
  }
  const total = Number.isInteger(payload.head?.results) ? payload.head.results : null;
  const returned = (page - 1) * 10 + hits.length;
  return {
    query: q,
    total,
    hits,
    hasMore: total !== null ? returned < total : hits.length === 10,
    source: 'mojeek-api',
  };
}

// Adapter table (RV-01). external=true adapters require key+approval and get
// retry + breaker + quota guards; owned adapters stay fail-closed without
// retry (a local failure must surface, never be masked by retries).
export const ADAPTERS = {
  opensearch: { source: 'opensearch-index', pageLimit: 100, external: false, fetchPage: fetchOpenSearchPage },
  local: { source: 'opensearch-index', pageLimit: 100, external: false, fetchPage: fetchOpenSearchPage },
  brave: { source: 'brave-api', pageLimit: 10, external: true, fetchPage: fetchBravePage },
  naver: { source: 'naver-api', pageLimit: 100, external: true, fetchPage: fetchNaverPage },
  // hybrid pageLimit stays 100: the ≤10 cap applies only when the external
  // leg is configured (enforced inside fetchHybridPage), preserving the
  // legacy degraded-local range.
  hybrid: { source: 'hybrid-rrf', pageLimit: 100, external: true, fetchPage: fetchHybridPage },
  mojeek: { source: 'mojeek-api', pageLimit: 100, external: true, fetchPage: fetchMojeekPage },
  serper: { source: 'serper-api', pageLimit: 10, external: true, fetchPage: unverifiedFetch('serper') },
  youcom: { source: 'youcom-api', pageLimit: 10, external: true, fetchPage: unverifiedFetch('youcom') },
};

export function resolveAdapter(name) {
  const adapter = ADAPTERS[name];
  if (!adapter) throw fail('INVALID_PROVIDER', 400);
  return adapter;
}

// Per-provider quota keys generalize the former brave-only
// 'brave-daily'/'brave-per-second' buckets (names preserved for brave).
export function quotaKeys(provider) {
  return { daily: `${provider}-daily`, perSecond: `${provider}-per-second` };
}

export function quotaLimits(config, provider) {
  if (provider === 'brave') return { daily: config.braveDailyLimit, perSecond: config.braveRps };
  if (provider === 'naver') return { daily: config.naverDailyLimit, perSecond: config.naverRps };
  if (provider === 'mojeek') return { daily: config.mojekDailyLimit, perSecond: config.mojekRps };
  // No 'hybrid' case: hybrid's external leg consumes the Brave account budget
  // directly (server.mjs calls guardQuota('brave')), so one global quota
  // covers both brave and hybrid modes.
  return null;
}

// Execute one provider page with resilience guards. Returns {result, outcome}
// where outcome is INTERNAL telemetry (attempts, fallback, breaker, latency)
// and must never be spread into HTTP responses (see server.mjs).
export async function executeProvider(name, ctx, options = {}) {
  const adapter = resolveAdapter(name);
  assertProviderAvailable(name);
  if (!Number.isInteger(ctx.page) || ctx.page < 1 || ctx.page > adapter.pageLimit) throw fail('INVALID_QUERY');
  const started = Date.now();
  const outcome = { provider: name, attempts: 0, retried: false, breakerOpened: false, failureClass: null, latencyMs: 0 };
  const sleep = options.sleep || defaultSleep;
  const run = () => adapter.fetchPage(ctx);
  try {
    let value;
    if (adapter.external && options.resilience !== false) {
      const breaker = options.breaker || breakerFor(name);
      const { value: v, opened, error } = await breaker.exec(() => withRetry(run, { sleep }));
      if (opened) {
        outcome.breakerOpened = true;
        outcome.failureClass = 'circuit_open';
        throw error;
      }
      value = v.value;
      outcome.attempts = v.attempts;
      outcome.retried = v.attempts > 1;
    } else {
      value = await run();
      outcome.attempts = 1;
    }
    outcome.latencyMs = Date.now() - started;
    if (options.onOutcome) options.onOutcome(outcome);
    return { result: value, outcome };
  } catch (error) {
    outcome.latencyMs = Date.now() - started;
    if (!outcome.failureClass) {
      const status = Number(error?.status || 0);
      outcome.failureClass = status === 429 ? 'rate_limited' : status >= 500 || status === 0 ? 'upstream_error' : 'rejected';
    }
    if (options.onOutcome) options.onOutcome(outcome);
    throw error;
  }
}
