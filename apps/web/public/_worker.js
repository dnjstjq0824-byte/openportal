// Service bindings point to operator-controlled services. Never proxy arbitrary
// client URLs, fall back to fixtures, or expose the prototype Go auth handlers.
function json(status, code, requestID) {
  return Response.json({ error: code, code, request_id: requestID }, {
    status, headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff', 'X-Request-ID': requestID },
  });
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if(url.pathname==='/sitemap.xml') {
      const entries=['ko','th','en'].map(locale=>`<url><loc>${url.origin}/${locale}</loc></url>`).join('');
      return new Response(`<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${entries}</urlset>`,{headers:{'Content-Type':'application/xml; charset=utf-8'}});
    }
    if(url.pathname==='/robots.txt') return new Response(`User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /*/search\nDisallow: /*/mail\nDisallow: /*/account\nDisallow: /*/login\nDisallow: /*/signup\nDisallow: /*/verify-email\nDisallow: /*/reset-password\nSitemap: ${url.origin}/sitemap.xml\n`,{headers:{'Content-Type':'text/plain; charset=utf-8'}});
    if (!url.pathname.startsWith('/api/')) {
      const response=await env.ASSETS.fetch(request);
      if(!response.headers.get('content-type')?.includes('text/html'))return response;
      const headers=new Headers(response.headers);headers.set('X-Robots-Tag',/^\/(ko|th|en)\/?$/.test(url.pathname)?'index,follow':'noindex,nofollow');headers.set('Referrer-Policy','no-referrer');
      return new Response(response.body,{status:response.status,headers});
    }
    const id = crypto.randomUUID();
    if(url.pathname.startsWith('/api/v2/')) {
      if(!env.WEB_GATEWAY&&!env.WEB_GATEWAY_ORIGIN)return json(503,'GATEWAY_UNAVAILABLE',id);
      try {
        const target=new URL(request.url);target.protocol='https:';target.host='gateway.internal';
        if(!env.WEB_GATEWAY){const origin=new URL(env.WEB_GATEWAY_ORIGIN);if(origin.protocol!=='https:'||origin.username||origin.password||origin.pathname!=='/'||origin.search||origin.hash||!env.GATEWAY_SHARED_SECRET)return json(503,'GATEWAY_UNAVAILABLE',id);target.host=origin.host;}
        const headers=new Headers(request.headers);headers.delete('authorization');headers.delete('x-forwarded-for');headers.delete('x-gateway-key');headers.set('X-Request-ID',id);headers.set('X-Gateway-Client-IP',request.headers.get('cf-connecting-ip')||'unknown');if(env.GATEWAY_SHARED_SECRET)headers.set('X-Gateway-Key',env.GATEWAY_SHARED_SECRET);
        const forwarded=new Request(target,{method:request.method,headers,body:['GET','HEAD'].includes(request.method)?undefined:request.body,duplex:'half',redirect:'error',signal:AbortSignal.timeout(20000)});
        const upstream=env.WEB_GATEWAY?await env.WEB_GATEWAY.fetch(forwarded):await fetch(forwarded);
        const output=new Headers(upstream.headers);output.set('Cache-Control','no-store');output.set('X-Content-Type-Options','nosniff');output.set('Referrer-Policy','no-referrer');
        return new Response(upstream.body,{status:upstream.status,headers:output});
      }catch{return json(502,'GATEWAY_UNAVAILABLE',id);}
    }
    if (!['/api/v1/search', '/api/v1/suggest'].includes(url.pathname)) return json(404, 'ENDPOINT_UNAVAILABLE', id);
    if (request.method !== 'GET') return json(405, 'METHOD_NOT_ALLOWED', id);
    if (!env.SEARCH_API) return json(503, 'SEARCH_BACKEND_UNAVAILABLE', id);
    try {
      const upstream = await env.SEARCH_API.fetch(new Request(`https://search.internal${url.pathname}${url.search}`, {
        method: 'GET', headers: { Accept: 'application/json', 'X-Request-ID': id },
        signal: AbortSignal.timeout(8000), redirect: 'error',
      }));
      if (!upstream.headers.get('content-type')?.includes('application/json')) return json(502, 'INVALID_UPSTREAM_RESPONSE', id);
      const body = await upstream.json();
      if (!upstream.ok) return json(upstream.status, upstream.status === 429 ? 'RATE_LIMITED' : 'SEARCH_REQUEST_FAILED', id);
      if (url.pathname === '/api/v1/search' && (!Array.isArray(body.hits) || !Number.isInteger(body.total) || typeof body.query !== 'string')) return json(502, 'INVALID_UPSTREAM_RESPONSE', id);
      return Response.json(body, { headers: { 'Cache-Control': 'no-store', 'X-Request-ID': id, 'X-Content-Type-Options': 'nosniff' } });
    } catch {
      return json(502, 'SEARCH_UPSTREAM_UNAVAILABLE', id);
    }
  },
};
