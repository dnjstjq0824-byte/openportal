import { fail } from './mail.mjs';

// Canonicalize URL for deduplication and comparison
export function normalizeURL(raw) {
  try {
    const u = new URL(raw);
    let pathname = u.pathname;
    if (pathname.endsWith('/')) {
      pathname = pathname.slice(0, -1);
    }
    return `${u.protocol}//${u.hostname.toLowerCase()}${u.port ? ':' + u.port : ''}${pathname}`;
  } catch {
    return String(raw || '').trim().toLowerCase();
  }
}

// Reciprocal Rank Fusion (RRF) rank merger (SE-11)
// score(d) = sum(1 / (k + rank_i(d)))
export function rrfMerge(localHits = [], externalHits = [], k = 60) {
  const map = new Map();

  // 1. Process local hits
  localHits.forEach((hit, idx) => {
    const rank = idx + 1;
    const key = normalizeURL(hit.url);
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
  if (page < 1 || page > 10) throw fail('INVALID_QUERY');

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
    body = await response.json();
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
        if (!['https:', 'http:'].includes(u.protocol) || seen.has(x.url)) return false;
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

// Track A: Bing Web Search API integration
export async function bingSearch(config, q, page = 1, options = {}) {
  const fetcher = options.fetcher || fetch;
  const safeSearch = options.safeSearch || 'moderate';
  if (!config.bingApproved || !config.bingKey) throw fail('SEARCH_NOT_CONFIGURED', 503);
  if (page < 1 || page > 10) throw fail('INVALID_QUERY');

  const bingSafeMap = { strict: 'Strict', moderate: 'Moderate', off: 'Off' };
  const bingSafe = bingSafeMap[safeSearch] || 'Moderate';

  const url = new URL('https://api.bing.microsoft.com/v7.0/search');
  url.search = new URLSearchParams({
    q,
    count: '10',
    offset: String((page - 1) * 10),
    safeSearch: bingSafe,
    responseFilter: 'Webpages',
  }).toString();

  let response;
  try {
    response = await fetcher(url, {
      headers: {
        Accept: 'application/json',
        'Ocp-Apim-Subscription-Key': config.bingKey,
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
    body = await response.json();
  } catch {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }

  const rawPages = body.webPages?.value;
  if (rawPages && !Array.isArray(rawPages)) {
    throw fail('SEARCH_UNAVAILABLE', 502);
  }

  const seen = new Set();
  const hits = (rawPages || [])
    .filter((x) => {
      try {
        if (typeof x.name !== 'string' || typeof x.snippet !== 'string') return false;
        const u = new URL(x.url);
        if (!['https:', 'http:'].includes(u.protocol) || seen.has(x.url)) return false;
        seen.add(x.url);
        return true;
      } catch {
        return false;
      }
    })
    .map((x) => ({
      title: x.name,
      url: x.url,
      description: x.snippet,
      source: 'bing-api',
    }));

  const totalEst = typeof body.webPages?.totalEstimatedMatches === 'number' ? body.webPages.totalEstimatedMatches : null;
  return {
    query: q,
    total: totalEst,
    hits,
    hasMore: totalEst !== null ? page * 10 < totalEst && page < 10 : hits.length === 10 && page < 10,
    source: 'bing-api',
  };
}

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
      const data = await res.json();
      if (!Array.isArray(data.hits)) throw new Error('Invalid local hits');
      return data;
    })
    .catch((err) => {
      console.warn('[HYBRID_SEARCH] Local backend error:', err.message);
      return null;
    });

  // 2. Prepare external search promise (Brave or Bing depending on config)
  let externalPromise;
  if (config.braveKey && config.braveApproved) {
    externalPromise = braveSearch(config, q, page, { fetcher, safeSearch }).catch((err) => {
      console.warn('[HYBRID_SEARCH] Brave API error:', err.message);
      return null;
    });
  } else if (config.bingKey && config.bingApproved) {
    externalPromise = bingSearch(config, q, page, { fetcher, safeSearch }).catch((err) => {
      console.warn('[HYBRID_SEARCH] Bing API error:', err.message);
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
  const total = localRes?.total ?? externalRes?.total ?? null;
  const hasMore = Boolean(localRes?.has_more ?? externalRes?.hasMore ?? (total !== null && page * 10 < total));

  return {
    query: q,
    total,
    hits: mergedHits,
    hasMore,
    source: 'hybrid-rrf',
    did_you_mean: localRes?.corrected_query || undefined,
  };
}
