import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { startStack } from './helpers/platform-stack.mjs';

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
const profile = username => ({ givenName: 'Admin', familyName: 'Test', birthDate: '1990-01-01', gender: 'unspecified', username, password: randomBytes(18).toString('base64url'), locale: 'ko', agree: true });

before(async () => {
  stack = await startStack({ gatewayPort: 0, pgPort: 55444 });
  base = `http://127.0.0.1:${stack.gateway.server.address().port}`;
  for (const name of ['admin', 'user']) {
    const session = await api(name, '/session');
    jar(name).csrf = session.data.csrf;
  }
});
after(async () => { if (stack) await stack.close(); });

test('admin signups timeseries and role management', async () => {
  const adminU = `adm.${randomBytes(4).toString('hex')}`;
  const userU = `usr.${randomBytes(4).toString('hex')}`;
  assert.equal((await api('admin', '/auth/signup', profile(adminU))).status, 201);
  assert.equal((await api('user', '/auth/signup', profile(userU))).status, 201);
  await stack.pool.query('update webapp.accounts set is_admin=true where username=$1', [adminU]);

  const series = await api('admin', '/admin/signups?days=14');
  assert.equal(series.status, 200);
  assert.equal(series.data.days, 14);
  assert.equal(series.data.signups.length, 14);
  assert.ok(series.data.signups.reduce((n, r) => n + r.count, 0) >= 2);
  assert.match(series.data.signups[0].date, /^\d{4}-\d{2}-\d{2}$/);

  assert.equal((await api('user', '/admin/signups')).data.code, 'FORBIDDEN');
  assert.equal((await api('admin', '/admin/signups?days=0')).data.code, 'INVALID_QUERY');
  assert.equal((await api('admin', '/admin/signups?days=91')).data.code, 'INVALID_QUERY');

  const userEmail = `${userU}@openscanner.test`;
  assert.equal((await api('user', `/admin/accounts/${encodeURIComponent(userEmail)}/role`, { is_admin: true }, 'PATCH')).data.code, 'FORBIDDEN');
  const promote = await api('admin', `/admin/accounts/${encodeURIComponent(userEmail)}/role`, { is_admin: true }, 'PATCH');
  assert.equal(promote.status, 200);
  assert.equal(promote.data.is_admin, true);
  assert.equal((await stack.pool.query('select is_admin from webapp.accounts where username=$1', [userU])).rows[0].is_admin, true);

  const adminEmail = `${adminU}@openscanner.test`;
  const selfDemote = await api('admin', `/admin/accounts/${encodeURIComponent(adminEmail)}/role`, { is_admin: false }, 'PATCH');
  assert.equal(selfDemote.status, 403);
  assert.equal(selfDemote.data.code, 'SELF_DEMOTE_FORBIDDEN');

  assert.equal((await api('admin', '/admin/accounts/nobody@example.test/role', { is_admin: true }, 'PATCH')).status, 404);
  assert.equal((await api('admin', `/admin/accounts/${encodeURIComponent(userEmail)}/role`, { is_admin: 'yes' }, 'PATCH')).data.code, 'INVALID_INPUT');
});
