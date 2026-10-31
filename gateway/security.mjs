import { createHash, randomBytes, scrypt as callbackScrypt, timingSafeEqual, createCipheriv, createDecipheriv } from 'node:crypto';
import { promisify } from 'node:util';

const scrypt = promisify(callbackScrypt);
export const randomToken = () => randomBytes(32).toString('base64url');
export const digest = value => createHash('sha256').update(value).digest('hex');
export function equal(a, b) {
  return typeof a === 'string' && typeof b === 'string' && Buffer.byteLength(a) === Buffer.byteLength(b) && timingSafeEqual(Buffer.from(a), Buffer.from(b));
}
export async function hashPassword(password) {
  if (typeof password !== 'string' || password.length < 12 || Buffer.byteLength(password) > 1024) throw Object.assign(new Error('PASSWORD_POLICY'), { status: 400 });
  const salt = randomBytes(16).toString('hex');
  const key = await scrypt(password, salt, 64, { N: 32768, r: 8, p: 1, maxmem: 64 * 1024 * 1024 });
  return `scrypt$32768$${salt}$${key.toString('hex')}`;
}
export async function verifyPassword(password, encoded) {
  const parts = String(encoded).split('$');
  if (parts.length !== 4 || parts[0] !== 'scrypt' || parts[1] !== '32768' || typeof password !== 'string' || Buffer.byteLength(password) > 1024) return false;
  const key = await scrypt(password, parts[2], 64, { N: 32768, r: 8, p: 1, maxmem: 64 * 1024 * 1024 });
  return equal(key.toString('hex'), parts[3]);
}
export function safeReturn(value, locale = 'en') {
  if (typeof value !== 'string' || /[\\\r\n]/.test(value)) return `/${locale}/mail`;
  try {
    const url = new URL(value, 'https://internal.invalid');
    if (url.origin !== 'https://internal.invalid' || !/^\/(ko|th|en)\/(mail|account)(\/|$)/.test(url.pathname)) return `/${locale}/mail`;
    return url.pathname + url.search;
  } catch { return `/${locale}/mail`; }
}
export function seal(value, key) {
  const iv = randomBytes(12); const cipher = createCipheriv('aes-256-gcm', key, iv);
  const encrypted = Buffer.concat([cipher.update(value, 'utf8'), cipher.final()]);
  return [iv, cipher.getAuthTag(), encrypted].map(x => x.toString('base64url')).join('.');
}
export function unseal(value, key) {
  const [iv, tag, encrypted] = value.split('.').map(x => Buffer.from(x, 'base64url'));
  const cipher = createDecipheriv('aes-256-gcm', key, iv); cipher.setAuthTag(tag);
  return Buffer.concat([cipher.update(encrypted), cipher.final()]).toString('utf8');
}
