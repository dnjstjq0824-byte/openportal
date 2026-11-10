import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { startStack } from './helpers/platform-stack.mjs';
import { digest } from '../gateway/security.mjs';

// D-5 regression: 6-digit OTP storage and verification are bound to the
// requesting account_id. Cross-account consumption fails closed without
// touching the victim row; exhausted/expired rows are deleted.
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
const profile = username => ({ givenName: 'Otp', familyName: 'Test', birthDate: '1990-01-01', gender: 'unspecified', username, password: randomBytes(18).toString('base64url'), locale: 'ko', agree: true });

before(async () => {
  stack = await startStack({ gatewayPort: 0, pgPort: 55445 });
  base = `http://127.0.0.1:${stack.gateway.server.address().port}`;
  for (const name of ['a', 'b', 'd', 'anon']) {
    const session = await api(name, '/session');
    jar(name).csrf = session.data.csrf;
  }
});
after(async () => { if (stack) await stack.close(); });

async function accountId(username) {
  return (await stack.pool.query('select id from webapp.accounts where username=$1', [username])).rows[0].id;
}

test('OTP verify is bound to the requesting account', async () => {
  const userA = `otpa.${randomBytes(4).toString('hex')}`;
  const userB = `otpb.${randomBytes(4).toString('hex')}`;
  assert.equal((await api('a', '/auth/signup', profile(userA))).status, 201);
  assert.equal((await api('b', '/auth/signup', profile(userB))).status, 201);
  const emailA = `${userA}@openscanner.test`;
  const emailB = `${userB}@openscanner.test`;
  const idA = await accountId(userA);

  // Victim row owned by A with a known OTP.
  const otpA = '111111';
  await stack.pool.query(
    `insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at) values($1,$2,'verify',now()+interval '30 minutes')`,
    [digest(`otp:${otpA}`), idA]);

  // 1. Cross-account consumption fails closed; victim row untouched.
  const cross = await api('anon', '/auth/verify', { token: otpA, email: emailB });
  assert.equal(cross.status, 400);
  assert.equal(cross.data.code, 'INVALID_TOKEN');
  const intact = await stack.pool.query('select account_id,attempts from webapp.action_tokens where token_hash=$1', [digest(`otp:${otpA}`)]);
  assert.equal(intact.rows.length, 1);
  assert.equal(intact.rows[0].account_id, idA);
  assert.equal(intact.rows[0].attempts, 0);

  // 2. OTP without account identification is rejected.
  const noAccount = await api('anon', '/auth/verify', { token: otpA });
  assert.equal(noAccount.status, 400);
  assert.equal(noAccount.data.code, 'ACCOUNT_REQUIRED');

  // 3. Correct account consumes the OTP.
  const ok = await api('anon', '/auth/verify', { token: otpA, email: emailA });
  assert.equal(ok.status, 200);
  assert.equal(ok.data.verified, true);
  assert.equal((await stack.pool.query('select 1 from webapp.action_tokens where token_hash=$1', [digest(`otp:${otpA}`)])).rows.length, 0);
});

test('exhausted and expired OTP rows are deleted', async () => {
  const userC = `otpc.${randomBytes(4).toString('hex')}`;
  assert.equal((await api('a', '/auth/signup', profile(userC))).status, 201);
  const emailC = `${userC}@openscanner.test`;
  const idC = await accountId(userC);

  await stack.pool.query(
    `insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at,attempts) values($1,$2,'verify',now()+interval '30 minutes',5)`,
    [digest('otp:222222'), idC]);
  const exhausted = await api('anon', '/auth/verify', { token: '222222', email: emailC });
  assert.equal(exhausted.status, 400);
  assert.equal((await stack.pool.query('select 1 from webapp.action_tokens where token_hash=$1', [digest('otp:222222')])).rows.length, 0);

  await stack.pool.query(
    `insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at) values($1,$2,'verify',now()-interval '1 minute')`,
    [digest('otp:333333'), idC]);
  const expired = await api('anon', '/auth/verify', { token: '333333', email: emailC });
  assert.equal(expired.status, 400);
  assert.equal((await stack.pool.query('select 1 from webapp.action_tokens where token_hash=$1', [digest('otp:333333')])).rows.length, 0);
});

test('live OTP roundtrip: resend issues account-bound code', async () => {
  const userD = `otpd.${randomBytes(4).toString('hex')}`;
  const recoveryD = `recovery.${randomBytes(4).toString('hex')}@example.test`;
  assert.equal((await api('d', '/auth/signup', { ...profile(userD), recoveryEmail: recoveryD })).status, 201);
  const emailD = `${userD}@openscanner.test`;
  assert.equal((await api('d', '/auth/resend', { email: recoveryD })).status, 202);
  const msg = [...stack.smtp.messages].reverse().find(x => x.to.toLowerCase() === recoveryD.toLowerCase() && x.raw.includes('verify-email'));
  assert.ok(msg, 'expected verify email via SMTP fixture');
  const otp = msg.raw.replace(/=\r\n/g, '').match(/OTP: (\d{6})/)?.[1];
  assert.ok(otp, 'expected 6-digit OTP in message text');
  const ok = await api('anon', '/auth/verify', { token: otp, email: emailD });
  assert.equal(ok.status, 200);
  assert.equal((await stack.pool.query('select recovery_email_verified_at is not null as v from webapp.accounts where username=$1', [userD])).rows[0].v, true);
});
