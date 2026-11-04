// Validation for hosted-mailbox signup (username@MAIL_DOMAIN). Pure functions; no I/O.
import { randomInt } from 'node:crypto';
import { fail } from './mail.mjs';

// RFC 2142 role mailboxes plus service/brand/operator names that must never be user-owned.
const RESERVED = new Set(['postmaster', 'hostmaster', 'webmaster', 'abuse', 'security', 'noc', 'admin', 'administrator', 'root', 'support', 'help', 'info', 'contact', 'mailer-daemon', 'noreply', 'no-reply', 'system', 'operator', 'staff', 'billing', 'privacy', 'legal', 'openscanner', 'openscan', 'official', 'service', 'team']);
export const MINIMUM_AGE = 14; // Korean PIPA: under-14 signup needs guardian consent, which is not implemented.

export function normalizeUsername(value) {
  return String(value ?? '').trim().toLowerCase();
}
/** Returns null when valid, otherwise 'format' | 'reserved'. */
export function usernameProblem(username) {
  if (!/^[a-z0-9][a-z0-9.]{4,28}[a-z0-9]$/.test(username) || username.includes('..')) return 'format';
  const bare = username.replace(/\./g, '');
  if (RESERVED.has(username) || RESERVED.has(bare) || [...RESERVED].some(word => word.length >= 5 && bare.startsWith(word))) return 'reserved';
  return null;
}
export function usernameCandidates(username, count = 3) {
  const base = username.replace(/[^a-z0-9.]/g, '').replace(/\.+$/, '').slice(0, 24) || 'user';
  const padded = base.length < 5 ? base.padEnd(5, '0') : base;
  return Array.from({ length: count * 3 }, () => `${padded}${randomInt(10, 10000)}`).filter(x => !usernameProblem(x));
}
export function normalizeDomain(value) {
  const domain = String(value || '').trim().toLowerCase();
  if (!/^(?=.{3,253}$)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(domain)) throw Error('MAIL_DOMAIN must be a valid domain name');
  return domain;
}
export function ageOn(birth, today = new Date()) {
  let age = today.getUTCFullYear() - birth.getUTCFullYear();
  const beforeBirthday = today.getUTCMonth() < birth.getUTCMonth() || (today.getUTCMonth() === birth.getUTCMonth() && today.getUTCDate() < birth.getUTCDate());
  return beforeBirthday ? age - 1 : age;
}
export function parseBirthDate(value, today = new Date()) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(value || ''));
  if (!match) throw fail('INVALID_INPUT');
  const date = new Date(Date.UTC(+match[1], +match[2] - 1, +match[3]));
  if (date.getUTCFullYear() !== +match[1] || date.getUTCMonth() !== +match[2] - 1 || date.getUTCDate() !== +match[3]) throw fail('INVALID_INPUT');
  const age = ageOn(date, today);
  if (age < 0 || age > 130) throw fail('INVALID_INPUT');
  if (age < MINIMUM_AGE) throw fail('AGE_REQUIREMENT', 403);
  return `${match[1]}-${match[2]}-${match[3]}`;
}
const text = (value, max) => String(value ?? '').normalize('NFC').replace(/[\u0000-\u001f\u007f<>]/g, '').trim().slice(0, max);
export function parseProfile(body, normalizeEmail, mailDomain) {
  const givenName = text(body.givenName, 40), familyName = text(body.familyName, 40);
  if (!givenName) throw fail('INVALID_INPUT');
  const gender = ['female', 'male', 'unspecified', 'custom'].includes(body.gender) ? body.gender : null;
  if (!gender) throw fail('INVALID_INPUT');
  const genderCustom = gender === 'custom' ? text(body.genderCustom, 40) : null;
  const pronoun = gender === 'custom' ? (['female', 'male', 'other'].includes(body.pronoun) ? body.pronoun : null) : null;
  if (gender === 'custom' && (!genderCustom || !pronoun)) throw fail('INVALID_INPUT');
  let recoveryEmail = null;
  if (body.recoveryEmail) {
    recoveryEmail = normalizeEmail(body.recoveryEmail);
    if (recoveryEmail.endsWith('@' + mailDomain)) throw fail('RECOVERY_EMAIL_SAME_DOMAIN');
  }
  let recoveryPhone = null;
  if (body.recoveryPhone) {
    recoveryPhone = String(body.recoveryPhone).replace(/[\s()-]/g, '');
    if (!/^\+[1-9]\d{6,14}$/.test(recoveryPhone)) throw fail('INVALID_PHONE');
  }
  return { givenName, familyName: familyName || null, birthDate: parseBirthDate(body.birthDate), gender, genderCustom, pronoun, recoveryEmail, recoveryPhone };
}
export function displayNameFor(profile, locale) {
  if (!profile.familyName) return profile.givenName;
  return locale === 'ko' ? `${profile.familyName}${profile.givenName}` : `${profile.givenName} ${profile.familyName}`;
}
