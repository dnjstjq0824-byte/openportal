import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { startStack } from './helpers/platform-stack.mjs';

// D-4 regression: /api/v2/admin/* denies by default regardless of is_admin
// until MFA, privileged audit logging, and separate-domain isolation ship.
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

test('admin routes deny by default even for admins', async () => {
  const adminU = `adm.${randomBytes(4).toString('hex')}`;
  const userU = `usr.${randomBytes(4).toString('hex')}`;
  assert.equal((await api('admin', '/auth/signup', profile(adminU))).status, 201);
  assert.equal((await api('user', '/auth/signup', profile(userU))).status, 201);
  await stack.pool.query('update webapp.accounts set is_admin=true where username=$1', [adminU]);

  const userEmail = `${userU}@openscanner.test`;
  const probes = [
    ['/admin/overview', undefined, 'GET'],
    ['/admin/accounts', undefined, 'GET'],
    ['/admin/signups?days=14', undefined, 'GET'],
    [`/admin/accounts/${encodeURIComponent(userEmail)}/role`, { is_admin: true }, 'PATCH'],
  ];
  // Authenticated admins and non-admins alike: unconditional 403.
  for (const name of ['admin', 'user']) {
    for (const [path, body, method] of probes) {
      const res = await api(name, path, body, method);
      assert.equal(res.status, 403);
      assert.equal(res.data.code, 'ADMIN_DISABLED');
    }
  }
  // Role escalation must not have happened through the dead surface.
  assert.equal((await stack.pool.query('select is_admin from webapp.accounts where username=$1', [userU])).rows[0].is_admin, false);
});
