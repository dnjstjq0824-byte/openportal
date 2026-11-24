import { createHash } from 'node:crypto';
import { fail } from './mail.mjs';

// Minimal exposure/click telemetry (Phase 3, §6.3 online-experiment substrate).
//
// Privacy contract (TARGET-P2): query TEXT is never recorded — only its
// SHA-256 hex (query_hash). Clicks carry a URL hash, never the URL.
// Experiment ids are random per-request tokens, anonymous by construction.
// Team-draft interleaving assignment stays future work (049/G020); this
// module is the recording substrate it will feed.
export function hashQuery(query) {
  return createHash('sha256').update(String(query || ''), 'utf8').digest('hex');
}

export function hashURL(url) {
  return createHash('sha256').update(String(url || ''), 'utf8').digest('hex');
}

function isToken(value, max = 64) {
  return typeof value === 'string' && value.length >= 1 && value.length <= max && /^[A-Za-z0-9_-]+$/.test(value);
}

export function validateClickEvent(raw) {
  if (!raw || typeof raw !== 'object') throw fail('INVALID_INPUT');
  const { request_id: requestId, exp: experimentId, rank, url_hash: urlHash } = raw;
  if (!isToken(requestId) || !isToken(experimentId)) throw fail('INVALID_INPUT');
  if (!Number.isInteger(rank) || rank < 1 || rank > 100) throw fail('INVALID_INPUT');
  if (typeof urlHash !== 'string' || !/^[0-9a-f]{64}$/.test(urlHash)) throw fail('INVALID_INPUT');
  return { requestId, experimentId, rank, urlHash };
}

export function createTelemetry({ sink } = {}) {
  const emit = sink || (event => console.log(JSON.stringify(event)));
  return {
    impression({ requestId, experimentId, provider, queryHash, hits, degraded, latencyMs }) {
      if (!isToken(requestId, 128) || !isToken(experimentId)) throw fail('INVALID_INPUT');
      if (typeof provider !== 'string' || !provider) throw fail('INVALID_INPUT');
      if (typeof queryHash !== 'string' || !/^[0-9a-f]{64}$/.test(queryHash)) throw fail('INVALID_INPUT');
      emit({
        ts: new Date().toISOString(),
        event: 'search_impression',
        request_id: requestId,
        exp: experimentId,
        provider,
        query_hash: queryHash,
        hits: Number.isInteger(hits) ? hits : 0,
        degraded: degraded === true,
        latency_ms: Number.isInteger(latencyMs) ? latencyMs : 0,
      });
    },
    click({ requestId, experimentId, rank, urlHash }) {
      const valid = validateClickEvent({ request_id: requestId, exp: experimentId, rank, url_hash: urlHash });
      emit({
        ts: new Date().toISOString(),
        event: 'search_click',
        request_id: valid.requestId,
        exp: valid.experimentId,
        rank: valid.rank,
        url_hash: valid.urlHash,
      });
    },
  };
}
