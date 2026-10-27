// Pages advanced-mode entrypoint. API requests must never reach SPA fallback.
const publicPaths = new Set(['/api/v1/search', '/api/v1/suggest', '/api/v1/portal/config']);
function json(status, code, requestID) {
  return Response.json({ error: code, code, request_id: requestID }, {
    status, headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff', 'X-Request-ID': requestID },
  });
}
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (!url.pathname.startsWith('/api/')) return env.ASSETS.fetch(request);
    const id = crypto.randomUUID();
    if (!publicPaths.has(url.pathname)) return json(404, 'ENDPOINT_UNAVAILABLE', id);
    if (request.method !== 'GET') return json(405, 'METHOD_NOT_ALLOWED', id);
    if (!env.SEARCH_API) return json(503, 'SEARCH_BACKEND_UNAVAILABLE', id);
    try {
      // A service binding is explicit deployment configuration, never a user URL.
      const upstream = await env.SEARCH_API.fetch(new Request(`https://search.internal${url.pathname}${url.search}`, {
        method: 'GET', headers: { Accept: 'application/json', 'X-Request-ID': id },
        signal: AbortSignal.timeout(8000), redirect: 'error',
      }));
      if (!upstream.headers.get('content-type')?.includes('application/json')) return json(502, 'INVALID_UPSTREAM_RESPONSE', id);
      const body = await upstream.json();
      if (upstream.status >= 400) return json(upstream.status, upstream.status === 429 ? 'RATE_LIMITED' : 'SEARCH_REQUEST_FAILED', id);
      if (url.pathname === '/api/v1/search' && (!Array.isArray(body.hits) || !Number.isInteger(body.total) || typeof body.query !== 'string')) {
        return json(502, 'INVALID_UPSTREAM_RESPONSE', id);
      }
      return Response.json(body, { headers: { 'Cache-Control': 'no-store', 'X-Request-ID': id, 'X-Content-Type-Options': 'nosniff' } });
    } catch {
      return json(502, 'SEARCH_UPSTREAM_UNAVAILABLE', id);
    }
  },
};
