import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { startStack } from './helpers/platform-stack.mjs';

// Phase 4 (Part D): sensitive gateway actions append audit rows; denied admin
// attempts are recorded; no secrets ever land in audit meta.
let stack, base;
const origin = 'http://127.0.0.1:5173';
const jars = new Map();
function jar(name) {
  if (!jars.has(name)) jars.set(name, { cookies: '', csrf: '' });
  return jars.get(name);
}
async function api(name, path, body, method = body ? 'POST' : 'GET') {
  const j = jar(name);
  const response = await fetch(base + '/api/v2' + path, { method, headers: { cookie: j.cookies, origin, 'x-csrf-token': j.csrf, 'content-type': 'application/json', 'user-agent': 'audit-test/1.0' }, body: body ? JSON.stringify(body) : undefined });
  for (const cookie of response.headers.getSetCookie()) {
    const part = cookie.split(';')[0];
    const cname = part.split('=')[0];
    j.cookies = j.cookies.split('; ').filter(x => x && !x.startsWith(cname + '=')).concat(part).join('; ');
  }
  return { status: response.status, data: await response.json() };
}
const password = randomBytes(18).toString('base64url');
const username = `aud.${randomBytes(4).toString('hex')}`;
const profile = { givenName: 'Aud', familyName: 'It', birthDate: '1990-01-01', gender: 'unspecified', username, password, locale: 'ko', agree: true };

before(async () => {
  stack = await startStack({ gatewayPort: 0, pgPort: 55447 });
  base = `http://127.0.0.1:${stack.gateway.server.address().port}`;
  const session = await api('a', '/session');
  jar('a').csrf = session.data.csrf;
});
after(async () => { if (stack) await stack.close(); });

async function actions(accountId) {
  return (await stack.pool.query('select action,account_id,target_type,target_id,ip,user_agent,meta,request_id from webapp.audit_log where account_id=$1 or $1 is null order by occurred_at', [accountId])).rows;
}

test('auth lifecycle is audited', async () => {
  assert.equal((await api('a', '/auth/signup', profile)).status, 201);
  const acc = (await stack.pool.query('select id,email from webapp.accounts where username=$1', [username])).rows[0];
  const id = acc.id;

  assert.equal((await api('a', '/auth/login', { email: acc.email, password: 'wrong-password-is-wrong' })).status, 401);
  assert.equal((await api('a', '/auth/logout', {})).status, 200);

  const rows = await actions(id);
  const names = rows.map(r => r.action);
  for (const want of ['auth.signup', 'auth.login_failed', 'auth.logout']) {
    assert.ok(names.includes(want), `missing ${want} in ${names}`);
  }
  const login = rows.find(r => r.action === 'auth.logout');
  assert.equal(login.user_agent, 'audit-test/1.0');
  assert.ok(login.ip);
  assert.ok(login.request_id);
});

test('denied admin attempt and report are audited without secrets', async () => {
  const acc = (await stack.pool.query('select id,email from webapp.accounts where username=$1', [username])).rows[0];
  const id = acc.id;
  assert.equal((await api('a', '/auth/login', { email: acc.email, password })).status, 200);
  assert.equal((await api('a', '/admin/users')).status, 403);
  assert.equal((await api('a', '/report', { document_id: 'doc-1', url: 'https://example.org/x', category: 'spam' })).status, 201);
  assert.equal((await api('a', '/account', { displayName: 'Aud It 2', locale: 'ko' }, 'PATCH')).status, 200);

  const rows = await actions(id);
  const names = rows.map(r => r.action);
  for (const want of ['auth.login', 'admin.denied', 'report.created', 'account.updated']) {
    assert.ok(names.includes(want), `missing ${want} in ${names}`);
  }
  const denied = rows.find(r => r.action === 'admin.denied');
  assert.equal(denied.target_type, 'route');

  const metas = (await stack.pool.query("select meta::text as m from webapp.audit_log")).rows.map(r => r.m);
  for (const m of metas) {
    assert.ok(!/password|token_hash|secret|csrf/i.test(m), `secret leaked in audit meta: ${m}`);
  }
});
