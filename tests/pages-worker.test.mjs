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
