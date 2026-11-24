import {
  normalizeURL as normalizeURLImpl,
  readSearchJSON as readSearchJSONImpl,
  rrfMerge as rrfMergeImpl,
  ADAPTERS,
} from './providers.mjs';

// Backward-compatible facade over gateway/providers.mjs (RV-01). The registry
// owns all provider logic; these wrappers preserve the historical function
// signatures and response shapes relied on by server.mjs and the test suite.
// The retired Bing stub was removed in Phase 2: 'bing' is refused at boot in
// config.mjs, and no bingSearch export exists anymore.

export const normalizeURL = normalizeURLImpl;
export const readSearchJSON = readSearchJSONImpl;
export const rrfMerge = rrfMergeImpl;

// Track A: Brave Search API integration
export async function braveSearch(config, q, page = 1, options = {}) {
  const fetcher = options.fetcher || fetch;
  const safeSearch = options.safeSearch || 'moderate';
  return ADAPTERS.brave.fetchPage({ config, q, page, safeSearch, fetcher });
}

// Track C: Hybrid search with Reciprocal Rank Fusion
export async function hybridSearch(config, q, page = 1, options = {}) {
  const fetcher = options.fetcher || fetch;
  const safeSearch = options.safeSearch || 'moderate';
  return ADAPTERS.hybrid.fetchPage({ config, q, page, safeSearch, fetcher });
}
