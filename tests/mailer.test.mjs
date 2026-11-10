import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  encodeMimeWord,
  formatFromHeader,
  deriveNumericOtp,
  buildTransactionalEmail,
  createDualModeMailer
} from '../gateway/mailer.mjs';

test('encodeMimeWord encodes non-ASCII strings with RFC 2047 UTF-8 Base64 and keeps ASCII intact', () => {
  assert.equal(encodeMimeWord('OpenScanner'), 'OpenScanner');
  const encoded = encodeMimeWord('OpenScanner 보안팀');
  assert.match(encoded, /^=\?UTF-8\?B\?[A-Za-z0-9+/=]+\?=$/);
  const header = formatFromHeader({ fromName: 'OpenScanner 보안', fromEmail: 'no-reply@openscanner.com' });
  assert.match(header, /^=\?UTF-8\?B\?.+\?= <no-reply@openscanner\.com>$/);
});

test('deriveNumericOtp returns a deterministic 6-digit numeric PIN', () => {
  const otp1 = deriveNumericOtp('sample-token-123');
  const otp2 = deriveNumericOtp('sample-token-123');
  assert.equal(otp1, otp2);
  assert.match(otp1, /^\d{6}$/);
});

test('buildTransactionalEmail renders all 4 HTML + Text templates (verify, reset, welcome, security) with inline CSS and table layout', () => {
  for (const locale of ['ko', 'en', 'th']) {
    const verify = buildTransactionalEmail('verify', {
      locale,
      link: `https://openportal-a94.pages.dev/${locale}/verify-email#token=tok123`,
      token: 'tok123'
    });
    assert.match(verify.subject, /OpenScanner/);
    assert.match(verify.html, /<table role="presentation"/);
    assert.match(verify.html, /#1D4ED8/);
    assert.ok(verify.html.includes(verify.otpCode));
    assert.ok(verify.text.includes(`#token=tok123`));

    const reset = buildTransactionalEmail('reset', {
      locale,
      link: `https://openportal-a94.pages.dev/${locale}/reset-password#token=rst456`,
      token: 'rst456'
    });
    assert.match(reset.subject, /OpenScanner/);
    assert.match(reset.html, /<table role="presentation"/);
    assert.ok(reset.text.includes(`#token=rst456`));

    const welcome = buildTransactionalEmail('welcome', {
      locale,
      displayName: '홍길동',
      email: 'hong@openscanner.com'
    });
    assert.match(welcome.subject, /OpenScanner/);
    assert.match(welcome.html, /OpenScanner/);

    const security = buildTransactionalEmail('security', {
      locale,
      action: 'Password changed',
      timestamp: '2026-11-10T00:00:00Z'
    });
    assert.match(security.subject, /OpenScanner/);
    assert.ok(security.html.includes('2026-11-10T00:00:00Z'));
  }
});

test('createDualModeMailer supports preview fallback mode without SMTP_HOST and returns null when unconfigured in strict mode', async () => {
  assert.equal(createDualModeMailer({ smtpMode: 'auto', smtpHost: '', env: 'test' }), null);

  const previewMailer = createDualModeMailer({
    smtpMode: 'preview',
    smtpFromName: 'OpenScanner',
    smtpFromEmail: 'no-reply@openscanner.com',
    env: 'test'
  });
  assert.equal(previewMailer.mode, 'preview');
  const res = await previewMailer.sendMail({
    to: 'user@example.com',
    subject: '[OpenScanner] 이메일 인증을 완료해 주세요',
    text: 'OTP: 123456',
    html: '<p>OTP: 123456</p>'
  });
  assert.equal(res.preview, true);
  assert.equal(previewMailer.previews.length, 1);
  assert.equal(previewMailer.previews[0].to, 'user@example.com');
});
