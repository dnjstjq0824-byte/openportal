import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { startStack } from './helpers/platform-stack.mjs';

// C-9: search-result reports persist per account. Anonymous submission is
// rejected (same bar as the moderation endpoints); invalid categories and
// non-http(s) URLs fail closed; reporters can list their own reports.
let stack, base;
const origin = 'http://127.0.0.1:5173';
const jars = new Map();
function jar(name) {
  if (!jars.has(name)) jars.set(name, { cookies: '', csrf: '' });
  return jars.get(name);
}
async function api(name, path, body, method = body ? 'POST' : 'GET') {
  const j = jar(name);
  const response = await fetch(base + '/api/v2' + path, { method, headers: { cookie: j.cookies, origin, 'x-csrf-token': j.csrf, 'content-type': 'application/json' }, body: body ? JSON.stringify(body) : undefined });
  for (const cookie of response.headers.getSetCookie()) {
    const part = cookie.split(';')[0];
    const cname = part.split('=')[0];
    j.cookies = j.cookies.split('; ').filter(x => x && !x.startsWith(cname + '=')).concat(part).join('; ');
  }
  return { status: response.status, data: await response.json() };
}
const profile = username => ({ givenName: 'Rep', familyName: 'Orter', birthDate: '1990-01-01', gender: 'unspecified', username, password: randomBytes(18).toString('base64url'), locale: 'ko', agree: true });
const report = overrides => ({ document_id: 'doc-001', url: 'https://example.org/spam', category: 'spam', details: 'keyword stuffing', ...overrides });

before(async () => {
  stack = await startStack({ gatewayPort: 0, pgPort: 55446 });
  base = `http://127.0.0.1:${stack.gateway.server.address().port}`;
  for (const name of ['a', 'b', 'anon']) {
    const session = await api(name, '/session');
    jar(name).csrf = session.data.csrf;
  }
  assert.equal((await api('a', '/auth/signup', profile(`repa.${randomBytes(4).toString('hex')}`))).status, 201);
  assert.equal((await api('b', '/auth/signup', profile(`repb.${randomBytes(4).toString('hex')}`))).status, 201);
});
after(async () => { if (stack) await stack.close(); });

test('anonymous report is rejected', async () => {
  const res = await api('anon', '/report', report());
  assert.equal(res.status, 401);
  assert.equal(res.data.code, 'SESSION_REQUIRED');
});

test('invalid report input fails closed', async () => {
  for (const bad of [
    report({ category: 'nope' }),
    report({ url: 'javascript:alert(1)' }),
    report({ url: 'ftp://example.org/x' }),
    report({ document_id: '' }),
    report({ url: '' }),
  ]) {
    const res = await api('a', '/report', bad);
    assert.equal(res.status, 400, JSON.stringify(bad));
  }
});

test('report persists and lists per account', async () => {
  const created = await api('a', '/report', report({ document_id: 'doc-007', category: 'malware' }));
  assert.equal(created.status, 201);
  assert.ok(created.data.id);
  assert.equal(created.data.status, 'pending');

  const row = (await stack.pool.query('select account_id,document_id,url,category,details,status from webapp.reports where id=$1', [created.data.id])).rows[0];
  assert.equal(row.document_id, 'doc-007');
  assert.equal(row.category, 'malware');
  assert.equal(row.status, 'pending');

  const mine = await api('a', '/report');
  assert.equal(mine.status, 200);
  assert.ok(mine.data.reports.some(r => r.id === created.data.id));

  const other = await api('b', '/report');
  assert.equal(other.status, 200);
  assert.ok(!other.data.reports.some(r => r.id === created.data.id));
});
