import { test } from 'node:test';
import assert from 'node:assert/strict';
import { QUERY_GROUPS, THRESHOLDS, evaluateRun, parseArgs } from '../scripts/provider-acceptance.mjs';

test('query matrix is fixed: 4 groups x 10 queries, each within the 200-char limit', () => {
  assert.deepEqual(Object.keys(QUERY_GROUPS).sort(), ['domestic', 'mixed', 'news', 'shopping']);
  for (const [group, queries] of Object.entries(QUERY_GROUPS)) {
    assert.equal(queries.length, 10, group);
    for (const q of queries) {
      assert.ok(q.trim().length > 0 && [...q].length <= 200, `${group}: ${q}`);
    }
  }
});

test('thresholds match the V-06 acceptance criteria', () => {
  assert.deepEqual(THRESHOLDS, {
    domesticKrRatio: 0.6,
    zeroRate: 0.05,
    koTitleRatio: 0.8,
    enTitleRatio: 0.7,
    maxDuplicates: 0,
    p95Ms: 3000,
  });
});

function hit(title, url) {
  return { title, url };
}

test('evaluateRun passes a clean matrix', () => {
  const results = [];
  for (const [group, queries] of Object.entries(QUERY_GROUPS)) {
    for (const query of queries) {
      const hits = group === 'mixed'
        ? Array.from({ length: 10 }, (_, i) => hit(`postgres RLS example guide ${i}`, `https://example.org/m${query.length}-${i}`))
        : Array.from({ length: 10 }, (_, i) => hit(`주민등록등본 발급 안내 ${i}번 문서`, `https://www.gov.kr/s-${query.length}-${i}`));
      results.push({ query, group, hits, latencyMs: 120 });
    }
  }
  const evaluation = evaluateRun(results);
  assert.equal(evaluation.pass, true);
  assert.equal(evaluation.checks.length, 6);
});

test('evaluateRun fails zero-rate, duplicates, and latency violations', () => {
  const results = [];
  for (const [group, queries] of Object.entries(QUERY_GROUPS)) {
    for (const query of queries) {
      results.push({ query, group, hits: [], latencyMs: 5000 });
    }
  }
  // One duplicated top-10.
  results[0].hits = [hit('같은 문서 제목', 'https://example.org/same'), hit('같은 문서 제목', 'https://example.org/same/')];
  const evaluation = evaluateRun(results);
  assert.equal(evaluation.pass, false);
  const byName = Object.fromEntries(evaluation.checks.map(c => [c.name, c]));
  assert.equal(byName.zero_rate.pass, false);
  assert.equal(byName.duplicates.pass, false);
  assert.equal(byName.duplicates.actual, 1);
  assert.equal(byName.p95_ms.pass, false);
});

test('parseArgs rejects unknown flags', () => {
  assert.throws(() => parseArgs(['--bogus']), /unknown argument/);
  assert.deepEqual(parseArgs(['--provider', 'naver']), { provider: 'naver', live: false, baseUrl: null, outDir: 'work' });
  assert.deepEqual(parseArgs(['--provider', 'naver', '--live', '--base-url', 'http://127.0.0.1:8090']).live, true);
});
