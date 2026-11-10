import { fail } from './mail.mjs';

export function cookieMap(req) { return Object.fromEntries(String(req.headers.cookie || '').split(';').map(x => x.trim().split('=')).filter(x => x.length === 2)); }
export async function readBody(req, max = 600000) {
  let size = 0; const chunks = [];
  for await (const chunk of req) { size += chunk.length; if (size > max) throw fail('PAYLOAD_TOO_LARGE', 413); chunks.push(chunk); }
  return Buffer.concat(chunks);
}
export async function readJSON(req) {
  if (!String(req.headers['content-type']).startsWith('application/json')) throw fail('INVALID_INPUT', 415);
  try { const body = JSON.parse((await readBody(req)).toString()); if (!body || Array.isArray(body) || typeof body !== 'object') throw Error(); return body; }
  catch (error) { if (error.status) throw error; throw fail('INVALID_INPUT'); }
}
export const normalizeEmail = value => {
  const email = String(value || '').trim().toLowerCase();
  if (email.length > 254 || !/^[^\s<>@,]+@[^\s<>@,]+\.[^\s<>@,]+$/.test(email)) throw fail('INVALID_INPUT');
  return email;
};
export const selectLocale = value => ['ko','th','en'].includes(value) ? value : 'en';
