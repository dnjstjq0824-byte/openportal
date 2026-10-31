export class APIError extends Error { constructor(public code: string, public status: number) { super(code); } }
let csrf = '';
export async function session() {
  const result = await request('/session'); csrf = result.csrf; return result.user;
}
export async function request(path: string, options: RequestInit = {}): Promise<any> {
  const response = await fetch(`/api/v2${path}`, { credentials:'same-origin', ...options, headers:{ ...(options.body && typeof options.body === 'string' ? { 'Content-Type':'application/json' } : {}), ...(options.method && options.method !== 'GET' ? { 'X-CSRF-Token':csrf } : {}), ...options.headers }, signal:options.signal || AbortSignal.timeout(15000) });
  let body; try { body=await response.json(); } catch { throw new APIError('fallback',response.status); }
  if (!response.ok) throw new APIError(body.code || 'fallback', response.status);
  return body;
}
export const mutate = (path: string, body: unknown, method='POST') => request(path,{method,body:JSON.stringify(body)});
