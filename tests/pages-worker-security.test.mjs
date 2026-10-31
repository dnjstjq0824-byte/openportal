import { test } from 'node:test';
import assert from 'node:assert/strict';
import worker from '../apps/web/public/_worker.js';

test('production host never fabricates search or account responses', async () => {
  for (const [path, method, expected] of [
    ['/api/v1/search?q=test', 'GET', 503],
    ['/api/v1/auth/register', 'POST', 404],
    ['/api/v1/superplatform/mail?email=a@example.com', 'GET', 404],
    ['/api/v1/admin', 'GET', 404],
  ]) {
    const response = await worker.fetch(new Request(`https://openportal.pages.dev${path}`, { method }), {});
    assert.equal(response.status, expected, path);
    assert.match(response.headers.get('content-type'), /application\/json/);
  }
});

test('privileged endpoints never reach a configured search binding', async () => {
  let called = false;
  const env = { SEARCH_API: { fetch() { called = true; throw Error('unexpected'); } } };
  const response = await worker.fetch(new Request('https://openportal.pages.dev/api/v1/admin'), env);
  assert.equal(response.status, 404);
  assert.equal(called, false);
});
