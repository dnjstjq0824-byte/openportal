import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  ADAPTERS,
  CircuitBreaker,
  UNVERIFIED_PROVIDERS,
  assertProviderAvailable,
  executeProvider,
  normalizeHit,
  quotaKeys,
  quotaLimits,
  resetBreakers,
  resolveAdapter,
  stripTags,
  withRetry,
} from '../gateway/providers.mjs';

test('registry resolves every selectable provider and rejects unknown names', () => {
  for (const name of ['opensearch', 'local', 'brave', 'naver', 'hybrid', 'mojeek', 'serper', 'youcom']) {
    assert.ok(resolveAdapter(name).fetchPage, name);
  }
  assert.throws(() => resolveAdapter('tavily'), error => error.message === 'INVALID_PROVIDER');
  assert.throws(() => resolveAdapter('bing'), error => error.message === 'INVALID_PROVIDER');
  assert.throws(() => resolveAdapter('gse'), error => error.message === 'INVALID_PROVIDER');
});

test('unverified providers fail closed naming the missing confirmation', () => {
  assert.deepEqual(Object.keys(UNVERIFIED_PROVIDERS).sort(), ['serper', 'youcom']);
  for (const [name, missing] of Object.entries(UNVERIFIED_PROVIDERS)) {
    assert.throws(() => assertProviderAvailable(name), error => error.message.includes('SPEC_UNVERIFIED') && error.message.includes(missing));
  }
  assert.doesNotThrow(() => assertProviderAvailable('naver'));
  assert.doesNotThrow(() => assertProviderAvailable('brave'));
  assert.doesNotThrow(() => assertProviderAvailable('mojeek'));
});

test('normalizeHit enforces the canonical contract', () => {
  const good = normalizeHit({ title: 'T', url: 'https://example.org/a', description: 'D', site_name: 'Example', document_id: 'd1' }, 'naver-api');
  assert.deepEqual(good, { title: 'T', url: 'https://example.org/a', description: 'D', source: 'naver-api', site_name: 'Example', document_id: 'd1' });
  assert.equal(normalizeHit({ title: 'T', url: 'javascript:alert(1)', description: 'D' }, 'x'), null);
  assert.equal(normalizeHit({ title: 'T', url: 'https://user:pass@example.org/', description: 'D' }, 'x'), null);
  assert.equal(normalizeHit({ title: 42, url: 'https://example.org/', description: 'D' }, 'x'), null);
  assert.equal(normalizeHit(null, 'x'), null);
});

test('stripTags removes Naver highlight markup', () => {
  assert.equal(stripTags('<b>검색</b>엔진 <b>결과</b>'), '검색엔진 결과');
});

test('naver adapter maps webkr transport (fixture, not a live call)', async () => {
  resetBreakers();
  let seen;
  const fetcher = async (url, opts) => {
    seen = { url: String(url), headers: opts.headers };
    assert.equal(opts.headers['X-Naver-Client-Id'], 'test-id');
    assert.equal(opts.headers['X-Naver-Client-Secret'], 'test-secret');
    return Response.json({
      lastBuildDate: 'Tue, 24 Nov 2026 00:00:00 +0900',
      total: 21,
      start: 11,
      display: 10,
      items: [
        { title: '<b>검색</b>엔진', link: 'https://example.org/1', description: '결과 <b>설명</b>' },
        { title: 'Bad', link: 'ftp://example.org/x', description: 'rejected scheme' },
      ],
    }, { headers: { 'Content-Type': 'application/json' } });
  };
  const config = { naverClientId: 'test-id', naverClientSecret: 'test-secret', naverApproved: true };
  const { result, outcome } = await executeProvider('naver', { config, q: '검색엔진', page: 2, safeSearch: 'moderate', fetcher }, { sleep: () => Promise.resolve() });
  const params = new URL(seen.url).searchParams;
  assert.equal(new URL(seen.url).hostname, 'openapi.naver.com');
  assert.equal(params.get('display'), '10');
  assert.equal(params.get('start'), '11');
  assert.equal(params.get('sort'), 'sim');
  assert.equal(params.get('query'), '검색엔진');
  assert.equal(result.source, 'naver-api');
  assert.equal(result.total, 21);
  assert.equal(result.hasMore, true);
  assert.deepEqual(result.hits, [{ title: '검색엔진', url: 'https://example.org/1', description: '결과 설명', source: 'naver-api' }]);
  assert.equal(outcome.provider, 'naver');
  assert.equal(outcome.attempts, 1);
  assert.equal(outcome.failureClass, null);
});

test('naver adapter requires credentials x approval and enforces page 1..100', async () => {
  resetBreakers();
  const fetcher = async () => Response.json({ total: 0, items: [] }, { headers: { 'Content-Type': 'application/json' } });
  await assert.rejects(
    executeProvider('naver', { config: { naverClientId: 'id', naverClientSecret: 's', naverApproved: false }, q: 'q', page: 1, safeSearch: 'moderate', fetcher }),
    error => error.message === 'SEARCH_NOT_CONFIGURED'
  );
  await assert.rejects(
    executeProvider('naver', { config: { naverClientId: 'id', naverClientSecret: 's', naverApproved: true }, q: 'q', page: 101, safeSearch: 'moderate', fetcher }),
    error => error.message === 'INVALID_QUERY'
  );
});

test('withRetry retries 5xx/429/network and never 4xx', async () => {
  let calls = 0;
  const flaky = async () => {
    calls++;
    if (calls < 3) throw Object.assign(new Error('SEARCH_UNAVAILABLE'), { status: 503 });
    return 'ok';
  };
  const { value, attempts } = await withRetry(flaky, { sleep: () => Promise.resolve() });
  assert.equal(value, 'ok');
  assert.equal(attempts, 3);
  let badCalls = 0;
  await assert.rejects(
    withRetry(async () => { badCalls++; throw Object.assign(new Error('INVALID_QUERY'), { status: 400 }); }, { sleep: () => Promise.resolve() }),
    error => error.message === 'INVALID_QUERY'
  );
  assert.equal(badCalls, 1);
  let configCalls = 0;
  await assert.rejects(
    withRetry(async () => { configCalls++; throw Object.assign(new Error('SEARCH_NOT_CONFIGURED'), { status: 503 }); }, { sleep: () => Promise.resolve() }),
    error => error.message === 'SEARCH_NOT_CONFIGURED'
  );
  assert.equal(configCalls, 1);
});

test('circuit breaker opens after threshold and recovers on probe success', async () => {
  const breaker = new CircuitBreaker({ failureThreshold: 2, cooldownMs: 20 });
  const boom = async () => { throw Object.assign(new Error('SEARCH_UNAVAILABLE'), { status: 503 }); };
  await assert.rejects(breaker.exec(boom));
  const opened = await breaker.exec(boom);
  assert.equal(opened.opened, true);
  await assert.rejects(breaker.exec(async () => 'x'), error => error.message === 'PROVIDER_CIRCUIT_OPEN');
  await new Promise(resolve => setTimeout(resolve, 25));
  const recovered = await breaker.exec(async () => 'ok');
  assert.equal(recovered.value, 'ok');
  assert.equal(breaker.state, 'closed');
});

test('quota helpers generalize the brave buckets and keep brave names', () => {
  assert.deepEqual(quotaKeys('brave'), { daily: 'brave-daily', perSecond: 'brave-per-second' });
  assert.deepEqual(quotaKeys('naver'), { daily: 'naver-daily', perSecond: 'naver-per-second' });
  const config = { braveDailyLimit: 1000, braveRps: 1, naverDailyLimit: 25000, naverRps: 5, mojekDailyLimit: 1000, mojekRps: 1 };
  assert.deepEqual(quotaLimits(config, 'brave'), { daily: 1000, perSecond: 1 });
  assert.deepEqual(quotaLimits(config, 'naver'), { daily: 25000, perSecond: 5 });
  assert.deepEqual(quotaKeys('mojeek'), { daily: 'mojeek-daily', perSecond: 'mojeek-per-second' });
  assert.deepEqual(quotaLimits(config, 'mojeek'), { daily: 1000, perSecond: 1 });
  assert.equal(quotaLimits(config, 'hybrid'), null);
  assert.equal(quotaLimits(config, 'opensearch'), null);
});

test('executeProvider enforces adapter page limits and reports outcomes', async () => {
  resetBreakers();
  const fetcher = async () => Response.json({ query: { original: 'q' }, web: { results: [] } }, { headers: { 'Content-Type': 'application/json' } });
  await assert.rejects(
    executeProvider('brave', { config: { braveKey: 'k', braveApproved: true }, q: 'q', page: 11, safeSearch: 'moderate', fetcher }),
    error => error.message === 'INVALID_QUERY'
  );
  const outcomes = [];
  await executeProvider('brave', { config: { braveKey: 'k', braveApproved: true }, q: 'q', page: 1, safeSearch: 'moderate', fetcher }, { onOutcome: o => outcomes.push(o) });
  assert.equal(outcomes.length, 1);
  assert.equal(outcomes[0].provider, 'brave');
  assert.equal(typeof outcomes[0].latencyMs, 'number');
});

test('hybrid keeps the legacy degraded-local page range without credentials', async () => {
  resetBreakers();
  const fetcher = async (target) => {
    if (String(target).includes('/api/v1/search')) {
      return Response.json({ query: 'q', total: 500, hits: [], corrected_query: 'qq' }, { headers: { 'Content-Type': 'application/json' } });
    }
    throw new Error('no external leg without credentials');
  };
  const { result } = await executeProvider('hybrid', { config: { searchURL: 'http://127.0.0.1:8080' }, q: 'q', page: 50, safeSearch: 'moderate', fetcher }, { resilience: false });
  assert.equal(result.source, 'hybrid-rrf');
  assert.equal(result.total, 500);
  assert.equal(result.did_you_mean, 'qq');
});

test('RV-05: verified adapters never touch the vertical path or its cache', async () => {
  resetBreakers();
  // Any request toward /vertical/* fails the test: external results must not
  // flow into the prototype vertical cache (V-07-2).
  const guardFetcher = async (target, opts) => {
    const url = String(target);
    assert.ok(!url.includes('/vertical'), `provider must not call vertical path: ${url}`);
    if (url.includes('/api/v1/search')) {
      return Response.json({ query: 'q', total: 1, hits: [{ title: 'T', url: 'https://example.org/', description: 'D', language: 'ko', score: 1 }] }, { headers: { 'Content-Type': 'application/json' } });
    }
    if (url.includes('api.search.brave.com')) {
      return Response.json({ query: { original: 'q' }, web: { results: [{ title: 'B', url: 'https://example.org/b', description: 'D' }] } }, { headers: { 'Content-Type': 'application/json' } });
    }
    if (url.includes('openapi.naver.com')) {
      return Response.json({ total: 1, items: [{ title: 'N', link: 'https://example.org/n', description: 'D' }] }, { headers: { 'Content-Type': 'application/json' } });
    }
    if (url.includes('www.mojeek.com')) {
      return Response.json({ response: { status: 'OK', head: { results: 1 }, results: [{ title: 'M', url: 'https://example.org/m', desc: 'D' }] } }, { headers: { 'Content-Type': 'application/json' } });
    }
    throw new Error('unexpected upstream: ' + url);
  };
  const full = { searchURL: 'http://127.0.0.1:8080', braveKey: 'k', braveApproved: true, naverClientId: 'id', naverClientSecret: 's', naverApproved: true, mojekKey: 'k', mojekApproved: true };
  for (const name of ['opensearch', 'local', 'brave', 'naver', 'mojeek', 'hybrid']) {
    const { result } = await executeProvider(name, { config: full, q: 'q', page: 1, safeSearch: 'moderate', fetcher: guardFetcher }, { sleep: () => Promise.resolve() });
    assert.ok(Array.isArray(result.hits), name);
  }
});

test('hybrid fuses local+Brave only: naver/mojeek never enter RRF (terms: no reordering)', async () => {
  resetBreakers();
  // Naver terms forbid reordered display; Mojeek posture matches. Hybrid must
  // not call them even when their credentials are configured.
  const fetcher = async (target) => {
    const url = String(target);
    if (!url.includes('/api/v1/search')) throw new Error('hybrid must not call external leg: ' + url);
    return Response.json({ query: 'q', total: 2, hits: [{ title: 'T', url: 'https://example.org/', description: 'D', language: 'ko', score: 1 }] }, { headers: { 'Content-Type': 'application/json' } });
  };
  const config = {
    searchURL: 'http://127.0.0.1:8080',
    naverClientId: 'id', naverClientSecret: 's', naverApproved: true,
    mojekKey: 'k', mojekApproved: true,
  };
  const { result } = await executeProvider('hybrid', { config, q: 'q', page: 3, safeSearch: 'moderate', fetcher }, { sleep: () => Promise.resolve() });
  assert.equal(result.source, 'hybrid-rrf');
  assert.equal(result.total, 2);
  assert.equal(result.degraded, false);
});

test('adapter table documents the Phase 2 provider surface', () => {
  assert.deepEqual(Object.keys(ADAPTERS).sort(), ['brave', 'hybrid', 'local', 'mojeek', 'naver', 'opensearch', 'serper', 'youcom']);
  assert.equal(ADAPTERS.brave.pageLimit, 10);
  assert.equal(ADAPTERS.naver.pageLimit, 100);
  assert.equal(ADAPTERS.mojeek.pageLimit, 100);
  assert.equal(ADAPTERS.opensearch.external, false);
});

test('mojeek adapter maps the official wire shape (fixture, not a live call)', async () => {
  resetBreakers();
  let seen;
  const fetcher = async (url, opts) => {
    seen = { url: String(url) };
    return Response.json({
      response: {
        status: 'OK',
        head: { query: 'mojeek', results: 25, start: 11, return: 10 },
        results: [
          { url: 'https://example.org/1', title: 'First', desc: 'First description' },
          { url: 'https://example.org/2', title: 'Second', desc: 'Second description' },
        ],
      },
    }, { headers: { 'Content-Type': 'application/json' } });
  };
  const config = { mojekKey: 'test-key', mojekApproved: true };
  const { result } = await executeProvider('mojeek', { config, q: 'mojeek', page: 2, safeSearch: 'strict', fetcher }, { sleep: () => Promise.resolve() });
  const target = new URL(seen.url);
  assert.equal(target.hostname, 'www.mojeek.com');
  assert.equal(target.pathname, '/search');
  assert.equal(target.searchParams.get('api_key'), 'test-key');
  assert.equal(target.searchParams.get('q'), 'mojeek');
  assert.equal(target.searchParams.get('s'), '11');
  assert.equal(target.searchParams.get('t'), '10');
  assert.equal(target.searchParams.get('fmt'), 'json');
  assert.equal(target.searchParams.get('safe'), '1');
  assert.equal(result.source, 'mojeek-api');
  assert.equal(result.total, 25);
  assert.equal(result.hasMore, true);
  assert.deepEqual(result.hits, [
    { title: 'First', url: 'https://example.org/1', description: 'First description', source: 'mojeek-api' },
    { title: 'Second', url: 'https://example.org/2', description: 'Second description', source: 'mojeek-api' },
  ]);
});

test('mojeek adapter honors safe=off, credentials, and ERROR statuses', async () => {
  resetBreakers();
  const config = { mojekKey: 'k', mojekApproved: true };
  let safeParam;
  const safeFetcher = async (url) => {
    safeParam = new URL(String(url)).searchParams.get('safe');
    return Response.json({ response: { status: 'OK', head: { results: 0 }, results: [] } }, { headers: { 'Content-Type': 'application/json' } });
  };
  await executeProvider('mojeek', { config, q: 'q', page: 1, safeSearch: 'off', fetcher: safeFetcher }, { sleep: () => Promise.resolve() });
  assert.equal(safeParam, '0');
  await assert.rejects(
    executeProvider('mojeek', { config: { mojekKey: 'k', mojekApproved: false }, q: 'q', page: 1, safeSearch: 'moderate', fetcher: safeFetcher }),
    error => error.message === 'SEARCH_NOT_CONFIGURED'
  );
  const limitFetcher = async () => Response.json({ response: { status: 'ERROR: Daily Limit Reached', head: {}, results: [] } }, { headers: { 'Content-Type': 'application/json' } });
  await assert.rejects(
    executeProvider('mojeek', { config, q: 'q', page: 1, safeSearch: 'moderate', fetcher: limitFetcher }, { sleep: () => Promise.resolve() }),
    error => error.message === 'RATE_LIMITED'
  );
  const errorFetcher = async () => Response.json({ response: { status: 'ERROR: Bad API key', head: {}, results: [] } }, { headers: { 'Content-Type': 'application/json' } });
  await assert.rejects(
    executeProvider('mojeek', { config, q: 'q', page: 1, safeSearch: 'moderate', fetcher: errorFetcher }, { sleep: () => Promise.resolve() }),
    error => error.message === 'SEARCH_UNAVAILABLE'
  );
});

test('naver adapter maps official error statuses', async () => {
  resetBreakers();
  const config = { naverClientId: 'id', naverClientSecret: 's', naverApproved: true };
  const ctx = (status) => ({ config, q: 'q', page: 1, safeSearch: 'moderate', fetcher: async () => new Response('{}', { status, headers: { 'Content-Type': 'application/json' } }) });
  await assert.rejects(executeProvider('naver', ctx(400), { sleep: () => Promise.resolve() }), error => error.message === 'INVALID_QUERY');
  await assert.rejects(executeProvider('naver', ctx(403), { sleep: () => Promise.resolve() }), error => error.message === 'SEARCH_NOT_CONFIGURED');
  await assert.rejects(executeProvider('naver', ctx(429), { sleep: () => Promise.resolve() }), error => error.message === 'RATE_LIMITED');
});
