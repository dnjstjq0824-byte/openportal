import { fail } from './mail.mjs';

export async function readSearchJSON(response) {
  if(!response.headers.get('content-type')?.includes('application/json'))throw fail('SEARCH_UNAVAILABLE',502);
  let length=0;const chunks=[];
  for await(const chunk of response.body||[]){length+=chunk.length;if(length>2*1024*1024)throw fail('SEARCH_UNAVAILABLE',502);chunks.push(chunk);}
  try{const body=JSON.parse(Buffer.concat(chunks).toString('utf8'));if(!body||typeof body!=='object'||Array.isArray(body))throw Error();return body;}catch{throw fail('SEARCH_UNAVAILABLE',502);}
}

// Canonicalize URL for deduplication and comparison
export function normalizeURL(raw) {
  try {
    const u = new URL(raw);
    let pathname = u.pathname;
    if (pathname.endsWith('/')) {
      pathname = pathname.slice(0, -1);
    }
    return `${u.protocol}//${u.hostname.toLowerCase()}${u.port ? ':' + u.port : ''}${pathname}${u.search}`;
  } catch {
    return String(raw || '').trim().toLowerCase();
  }
}

// Reciprocal Rank Fusion (RRF) rank merger (SE-11)
// score(d) = sum(1 / (k + rank_i(d)))
export function rrfMerge(localHits = [], externalHits = [], k = 60) {
  const map = new Map();
  const seenExternal = new Set();

  // 1. Process local hits
  localHits.forEach((hit, idx) => {
    const rank = idx + 1;
    const key = normalizeURL(hit.url);
    if (map.has(key)) return;
    const score = 1.0 / (k + rank);
    map.set(key, {
      ...hit,
      rrf_score: score,
      local_rank: rank,
      source: hit.source || 'opensearch',
    });
  });

  // 2. Process external hits
  externalHits.forEach((hit, idx) => {
    const rank = idx + 1;
    const key = normalizeURL(hit.url);
    if (seenExternal.has(key)) return;
    seenExternal.add(key);
    const score = 1.0 / (k + rank);
    if (map.has(key)) {
      const existing = map.get(key);
      existing.rrf_score += score;
      existing.external_rank = rank;
      // Retain local metadata (title/desc) if available, or fill if empty
      if (!existing.title && hit.title) existing.title = hit.title;
      if (!existing.description && hit.description) existing.description = hit.description;
    } else {
      map.set(key, {
        ...hit,
        rrf_score: score,
        external_rank: rank,
        source: hit.source || 'external-api',
      });
    }
  });

  // 3. Sort by descending RRF score
  const merged = Array.from(map.values()).sort((a, b) => b.rrf_score - a.rrf_score);
  return merged;
}

// Track A: Brave Search API integration
export async function braveSearch(config, q, page = 1, options = {}) {
  const fetcher = options.fetcher || fetch;
  const safeSearch = options.safeSearch || 'moderate';
  if (!config.braveApproved || !config.braveKey) throw fail('SEARCH_NOT_CONFIGURED', 503);
  if (!Number.isInteger(page) || page < 1 || page > 10) throw fail('INVALID_QUERY');

  const braveSafe = ['strict', 'moderate', 'off'].includes(safeSearch) ? safeSearch : 'moderate';
  const url = new URL('https://api.search.brave.com/res/v1/web/search');
  url.search = new URLSearchParams({
    q,
    count: '10',
    offset: String(page - 1),
    safesearch: braveSafe,
    text_decorations: 'false',
    result_filter: 'web',
  }).toString();

  let response;
  try {
    response = await fetcher(url, {
      headers: {
        Accept: 'application/json',
        'X-Subscription-Token': config.braveKey,
      },
      signal: AbortSignal.timeout(8000),
      redirect: 'error',
    });
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }

  if (!response.ok) {
    throw fail(response.status === 429 ? 'RATE_LIMITED' : 'SEARCH_UNAVAILABLE', response.status === 429 ? 429 : 503);
  }

  let body;
  try {
    body = await readSearchJSON(response);
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }

  if (typeof body.query?.original !== 'string' || (body.web && !Array.isArray(body.web.results))) {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }

  const seen = new Set();
  const hits = (body.web?.results || [])
    .filter((x) => {
      try {
        if (typeof x.title !== 'string' || typeof x.description !== 'string') return false;
        const u = new URL(x.url);
        if (!['https:', 'http:'].includes(u.protocol) || u.username || u.password || seen.has(x.url)) return false;
        seen.add(x.url);
        return true;
      } catch {
        return false;
      }
    })
    .map((x) => ({
      title: x.title,
      url: x.url,
      description: x.description,
      source: 'brave-api',
    }));

  return {
    query: q,
    total: null,
    hits,
    hasMore: page < 10 && body.query.more_results_available === true,
    source: 'brave-api',
  };
}

// The public Bing Search API was retired on 2025-08-11. Preserve the export
// for configuration compatibility, but never call the discontinued endpoint.
export async function bingSearch() { throw fail('SEARCH_PROVIDER_RETIRED', 503); }
// Track C: Hybrid search with Reciprocal Rank Fusion
export async function hybridSearch(config, q, page = 1, options = {}) {
  const fetcher = options.fetcher || fetch;
  const safeSearch = options.safeSearch || 'moderate';

  // 1. Prepare local search promise
  const localTarget = new URL('/api/v1/search', config.searchURL);
  localTarget.search = new URLSearchParams({
    q,
    page: String(page),
    safe_search: safeSearch,
  }).toString();

  const localPromise = fetcher(localTarget, {
    headers: { Accept: 'application/json' },
    signal: AbortSignal.timeout(8000),
    redirect: 'error',
  })
    .then(async (res) => {
      if (!res.ok) throw new Error(`Local search HTTP ${res.status}`);
      const data = await readSearchJSON(res);
      if (!Array.isArray(data.hits)) throw new Error('Invalid local hits');
      return data;
    })
    .catch((err) => {
      console.warn(JSON.stringify({event:'search_source_unavailable',source:'local'}));
      return null;
    });

  // 2. Prepare external search promise (Brave or Bing depending on config)
  let externalPromise;
  if (config.braveKey && config.braveApproved) {
    externalPromise = braveSearch(config, q, page, { fetcher, safeSearch }).catch((err) => {
      console.warn(JSON.stringify({event:'search_source_unavailable',source:'brave'}));
      return null;
    });
  } else {
    externalPromise = Promise.resolve(null);
  }

  const [localRes, externalRes] = await Promise.all([localPromise, externalPromise]);

  if (!localRes && !externalRes) {
    throw fail('SEARCH_UNAVAILABLE', 503);
  }

  const localHits = (localRes?.hits || []).map((h) => ({ ...h, source: 'opensearch-index' }));
  const externalHits = externalRes?.hits || [];

  const mergedHits = rrfMerge(localHits, externalHits, 60);
  // Each provider returns one page. The overlap and the combined corpus size
  // are unknown; a local count cannot describe the union of both providers.
  const externalRequested = Boolean(config.braveKey && config.braveApproved);
  const total = externalRequested ? null : localRes?.total ?? null;
  const hasMore = Boolean((!externalRequested || page < 10) && ((localRes && (localRes.has_more ?? (Number.isInteger(localRes.total) && page * 10 < localRes.total))) || externalRes?.hasMore));

  return {
    query: q,
    total,
    hits: mergedHits,
    hasMore,
    degraded: !localRes || (externalRequested && !externalRes),
    source: 'hybrid-rrf',
    did_you_mean: localRes?.corrected_query || undefined,
  };
}
