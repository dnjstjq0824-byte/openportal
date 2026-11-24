import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createTelemetry, hashQuery, hashURL, validateClickEvent } from '../gateway/telemetry.mjs';
import { configuration } from '../gateway/config.mjs';
import { createGateway } from '../gateway/server.mjs';

test('query and URL hashes are deterministic SHA-256 hex', () => {
  assert.equal(hashQuery('검색엔진'), hashQuery('검색엔진'));
  assert.match(hashQuery('검색엔진'), /^[0-9a-f]{64}$/);
  assert.match(hashURL('https://example.org/'), /^[0-9a-f]{64}$/);
  assert.notEqual(hashQuery('a'), hashQuery('b'));
});

test('impression carries hashes only, never query text', () => {
  const events = [];
  const telemetry = createTelemetry({ sink: e => events.push(e) });
  telemetry.impression({ requestId: 'r1', experimentId: 'e1', provider: 'naver', queryHash: hashQuery('secret query'), hits: 3, degraded: false, latencyMs: 42 });
  assert.equal(events.length, 1);
  const body = JSON.stringify(events[0]);
  assert.ok(!body.includes('secret query'));
  assert.equal(events[0].event, 'search_impression');
  assert.equal(events[0].query_hash, hashQuery('secret query'));
});

test('click validation rejects raw URLs, bad ranks, and malformed ids', () => {
  const good = { request_id: 'r1', exp: 'e1', rank: 2, url_hash: hashURL('https://example.org/') };
  assert.deepEqual(validateClickEvent(good), { requestId: 'r1', experimentId: 'e1', rank: 2, urlHash: hashURL('https://example.org/') });
  assert.throws(() => validateClickEvent({ ...good, url_hash: 'https://example.org/' }), /INVALID_INPUT/);
  assert.throws(() => validateClickEvent({ ...good, rank: 0 }), /INVALID_INPUT/);
  assert.throws(() => validateClickEvent({ ...good, rank: 101 }), /INVALID_INPUT/);
  assert.throws(() => validateClickEvent({ ...good, exp: 'has space' }), /INVALID_INPUT/);
  assert.throws(() => validateClickEvent(null), /INVALID_INPUT/);
});

async function startTestGateway(events) {
  const searchFetch = async (target) => {
    if (String(target).includes('/api/v1/search')) {
      return Response.json({
        query: 'q', total: 1, sponsored_ads: [],
        hits: [{ title: 'T', url: 'https://example.org/', description: 'D', language: 'ko', score: 1 }],
      }, { headers: { 'Content-Type': 'application/json' } });
    }
    throw new Error('unexpected upstream: ' + String(target));
  };
  const pool = { query: async () => ({ rows: [{ attempts: 1 }] }) };
  const telemetry = createTelemetry({ sink: e => events.push(e) });
  const app = await createGateway(configuration({ SEARCH_PROVIDER: 'opensearch' }), { pool, searchFetch, telemetry });
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve));
  const port = app.server.address().port;
  return { app, base: `http://127.0.0.1:${port}` };
}

test('gateway search emits an impression and exposes provider metrics', async () => {
  const events = [];
  const { app, base } = await startTestGateway(events);
  try {
    const res = await fetch(`${base}/api/v2/search?q=${encodeURIComponent('노출 측정 질의')}&page=1`);
    assert.equal(res.status, 200);
    const impression = events.find(e => e.event === 'search_impression');
    assert.ok(impression);
    assert.equal(impression.provider, 'opensearch');
    assert.equal(impression.hits, 1);
    assert.equal(impression.query_hash, hashQuery('노출 측정 질의'));
    assert.ok(!JSON.stringify(impression).includes('노출 측정 질의'));
    const metrics = await (await fetch(`${base}/metrics`)).text();
    assert.match(metrics, /openportal_provider_calls_total\{provider="opensearch",outcome="ok"\} 1/);
    assert.match(metrics, /openportal_provider_latency_ms\{provider="opensearch",quantile="0\.95"\} \d+/);
    assert.match(metrics, /openportal_provider_retries_total 0/);
  } finally {
    await app.close();
  }
});

test('gateway ingests anonymous clicks and rejects raw URLs', async () => {
  const events = [];
  const { app, base } = await startTestGateway(events);
  try {
    const csrf = 'c'.repeat(43);
    const headers = { 'Content-Type': 'application/json', origin: 'http://127.0.0.1:5173', cookie: `op_csrf=${csrf}`, 'x-csrf-token': csrf };
    const ok = await fetch(`${base}/api/v2/events`, {
      method: 'POST', headers,
      body: JSON.stringify({ events: [{ type: 'click', request_id: 'r9', exp: 'e9', rank: 3, url_hash: hashURL('https://example.org/x') }] }),
    });
    assert.equal(ok.status, 202);
    assert.deepEqual(await ok.json(), { accepted: 1 });
    assert.equal(events.filter(e => e.event === 'search_click').length, 1);
    const raw = await fetch(`${base}/api/v2/events`, {
      method: 'POST', headers,
      body: JSON.stringify({ events: [{ type: 'click', request_id: 'r9', exp: 'e9', rank: 3, url_hash: 'https://example.org/x' }] }),
    });
    assert.equal(raw.status, 400);
    const badType = await fetch(`${base}/api/v2/events`, {
      method: 'POST', headers,
      body: JSON.stringify({ events: [{ type: 'purchase', request_id: 'r9', exp: 'e9', rank: 3, url_hash: hashURL('https://example.org/x') }] }),
    });
    assert.equal(badType.status, 400);
  } finally {
    await app.close();
  }
});
