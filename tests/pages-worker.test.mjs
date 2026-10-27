import { test } from 'node:test';
import assert from 'node:assert/strict';
import worker from '../apps/web/public/_worker.js';

const request = (path='/api/v1/search?q=test', method='GET') => new Request(`https://openportal.test${path}`, {method});
test('missing backend returns JSON 503, never SPA HTML', async () => {
  const response = await worker.fetch(request(), {ASSETS: {fetch() { throw Error('API reached SPA'); }}});
  assert.equal(response.status, 503);
  assert.equal((await response.json()).code, 'SEARCH_BACKEND_UNAVAILABLE');
});
test('HTML, invalid JSON and network failures become sanitized JSON', async () => {
  for (const fetch of [async()=>new Response('<!doctype html>'), async()=>new Response('{', {headers:{'content-type':'application/json'}}), async()=>{throw Error('secret upstream');}]) {
    const response = await worker.fetch(request(), {SEARCH_API:{fetch}});
    assert.equal(response.status, 502);
    assert.match(response.headers.get('content-type'), /application\/json/);
    assert.doesNotMatch(await response.text(), /secret|doctype|SyntaxError/);
  }
});
test('search contract, query forwarding and request correlation', async () => {
  const response = await worker.fetch(request(), {SEARCH_API:{async fetch(req) {
    assert.equal(new URL(req.url).searchParams.get('q'), 'test');
    assert.ok(req.headers.get('x-request-id'));
    return Response.json({query:'test', total:0, hits:[]});
  }}});
  assert.equal(response.status, 200);
  assert.deepEqual((await response.json()).hits, []);
});
test('unsupported methods and privileged routes are denied before upstream', async () => {
  assert.equal((await worker.fetch(request('/api/v1/search','POST'), {})).status, 405);
  for (const path of ['/api/v1/auth/verify','/api/v1/admin','/api/v1/organizations/x']) {
    assert.equal((await worker.fetch(request(path), {})).status, 404);
  }
});
test('non-API requests retain asset routing', async () => {
  const response = await worker.fetch(request('/'), {ASSETS:{fetch:async()=>new Response('home')}});
  assert.equal(await response.text(), 'home');
});

test('edge search fallback on production pages.dev serves live verified corpus without 503', async () => {
  const reqPages = (path) => new Request(`https://openportal.pages.dev${path}`, { method: 'GET' });
  
  // Test search match
  const matchRes = await worker.fetch(reqPages('/api/v1/search?q=openportal'), {});
  assert.equal(matchRes.status, 200);
  const matchBody = await matchRes.json();
  assert.ok(matchBody.total > 0);
  assert.ok(matchBody.hits.length > 0);
  assert.equal(typeof matchBody.hits[0].title, 'string');
  assert.equal(typeof matchBody.hits[0].url, 'string');

  // Test zero results match (query '0') - must be 200 with hits: [], NEVER 503
  const zeroRes = await worker.fetch(reqPages('/api/v1/search?q=0'), {});
  assert.equal(zeroRes.status, 200);
  const zeroBody = await zeroRes.json();
  assert.equal(zeroBody.total, 0);
  assert.deepEqual(zeroBody.hits, []);

  // Test suggestions
  const sugRes = await worker.fetch(reqPages('/api/v1/suggest?q=검색'), {});
  assert.equal(sugRes.status, 200);
  const sugBody = await sugRes.json();
  assert.ok(Array.isArray(sugBody.suggestions));

  // Test portal config
  const cfgRes = await worker.fetch(reqPages('/api/v1/portal/config'), {});
  assert.equal(cfgRes.status, 200);
  const cfgBody = await cfgRes.json();
  assert.equal(cfgBody.search_enabled, true);
});
