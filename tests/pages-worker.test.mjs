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

test('edge vertical search serves typed structured results without 404 or 503', async () => {
  const reqPages = (path) => new Request(`https://openportal.pages.dev${path}`, { method: 'GET' });

  for (const vert of ['image', 'video', 'news', 'map', 'shopping', 'books']) {
    const res = await worker.fetch(reqPages(`/api/v1/vertical/search?vertical=${vert}&q=`), {});
    assert.equal(res.status, 200, `vertical ${vert} must return 200`);
    const body = await res.json();
    assert.equal(body.vertical, vert);
    assert.ok(Array.isArray(body.data));
    assert.ok(body.total_hits > 0);
  }

  // Filtered vertical search
  const filterRes = await worker.fetch(reqPages('/api/v1/vertical/search?vertical=books&q=Information'), {});
  assert.equal(filterRes.status, 200);
  const filterBody = await filterRes.json();
  assert.ok(filterBody.total_hits >= 1);
  assert.match(filterBody.data[0].title, /Information Retrieval/);
});

test('edge authentication lifecycle handles full user journey on pages.dev', async () => {
  const postPages = (path, body) => new Request(`https://openportal.pages.dev${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  const testEmail = `user_${Date.now()}@openscanner.internal`;
  const testPassword = 'Password1234!';

  // 1. Register
  const regRes = await worker.fetch(postPages('/api/v1/auth/register', {
    email: testEmail,
    password: testPassword,
    display_name: 'Edge Tester',
  }), {});
  assert.equal(regRes.status, 201);
  const regBody = await regRes.json();
  assert.equal(regBody.status, 'pending_verification');
  assert.ok(regBody.code);

  // 2. Attempt login before email verification -> fails closed
  const preLoginRes = await worker.fetch(postPages('/api/v1/auth/login', {
    email: testEmail,
    password: testPassword,
  }), {});
  assert.equal(preLoginRes.status, 400);

  // 3. Verify Email PIN
  const verifyRes = await worker.fetch(postPages('/api/v1/auth/email/verify', {
    email: testEmail,
    code: regBody.code,
  }), {});
  assert.equal(verifyRes.status, 200);
  const verifyBody = await verifyRes.json();
  assert.equal(verifyBody.status, 'verified');

  // 4. Authenticate Login
  const loginRes = await worker.fetch(postPages('/api/v1/auth/login', {
    email: testEmail,
    password: testPassword,
  }), {});
  assert.equal(loginRes.status, 200);
  const loginBody = await loginRes.json();
  assert.equal(loginBody.status, 'authenticated');
  assert.ok(loginBody.token);
  assert.equal(loginBody.email, testEmail);

  // 5. Password Reset Request & Confirm
  const resetReqRes = await worker.fetch(postPages('/api/v1/auth/password/reset-request', {
    email: testEmail,
  }), {});
  assert.equal(resetReqRes.status, 200);
  const resetReqBody = await resetReqRes.json();
  assert.ok(resetReqBody.token);

  const resetConfirmRes = await worker.fetch(postPages('/api/v1/auth/password/reset-confirm', {
    email: testEmail,
    token: resetReqBody.token,
    new_password: 'NewPassword999!',
  }), {});
  assert.equal(resetConfirmRes.status, 200);

  // 6. Login with new password
  const newLoginRes = await worker.fetch(postPages('/api/v1/auth/login', {
    email: testEmail,
    password: 'NewPassword999!',
  }), {});
  assert.equal(newLoginRes.status, 200);
});

test('edge superplatform endpoints serve apps, webmail, and devstudio', async () => {
  const getPages = (path) => new Request(`https://openportal.pages.dev${path}`, { method: 'GET' });

  // 1. Apps
  const appsRes = await worker.fetch(getPages('/api/v1/superplatform/apps'), {});
  assert.equal(appsRes.status, 200);
  const appsBody = await appsRes.json();
  assert.ok(appsBody.apps.length >= 8);

  // 2. Mail
  const mailRes = await worker.fetch(getPages('/api/v1/superplatform/mail?email=tester@openscanner.internal'), {});
  assert.equal(mailRes.status, 200);
  const mailBody = await mailRes.json();
  assert.ok(mailBody.messages.length > 0);

  // 3. Developer
  const devRes = await worker.fetch(getPages('/api/v1/superplatform/developer'), {});
  assert.equal(devRes.status, 200);
  const devBody = await devRes.json();
  assert.equal(devBody.version, 'v2.0.0');
  assert.ok(devBody.api_key.startsWith('sk_live_'));
});
