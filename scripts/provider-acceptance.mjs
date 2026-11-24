// Provider acceptance runner (RV-06, implements the V-06 design).
// Compares providers on a fixed Korean-coverage query matrix through a
// RUNNING gateway (/api/v2/search, page=1, safe_search=moderate).
//
// The script holds NO provider credentials: the gateway under test owns keys.
// Exit codes: 0 = matrix PASS (or dry-run plan printed), 1 = matrix FAIL,
// 2 = misconfigured (bad args, unreachable base URL).
//
// Usage:
//   node scripts/provider-acceptance.mjs --provider naver            # dry-run plan (no network)
//   node scripts/provider-acceptance.mjs --provider naver --live --base-url http://127.0.0.1:8090

import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { normalizeURL } from '../gateway/providers.mjs';

// V-06-1 representative query sets (10 per group, each <= 200 chars per the
// backend query limit). Fixed inputs: adding/removing queries changes the
// matrix, so the evidence file records the exact queries executed.
export const QUERY_GROUPS = {
  domestic: [
    '주민등록등본 발급', '서울 지하철 노선도', '내일 날씨 서울', '국세청 홈택스 연말정산',
    '여권 재발급 준비물', '고속버스 예매', '전기요금 조회', '주민센터 점심시간',
    '우체국 택배 조회', '자동차 검사 예약',
  ],
  news: [
    '오늘 주요 뉴스', '국회 본회의 결과', '코스피 오늘 종가', '프로야구 오늘 경기 결과',
    '서울 오늘 날씨 속보', '지하철 지연 증명서', '주유소 기름값 오늘', '로또 당첨 번호',
    '코로나 확진자 현황', '태풍 경로 예보',
  ],
  shopping: [
    '무선 이어폰 추천', '김치냉장고 가격비교', '노트북 최저가', '공기청정기 추천',
    '러닝화 세일', '커피머신 가격', '겨울 패딩 추천', '로봇청소기 비교',
    '모니터 27인치 가격', '캠핑 텐트 추천',
  ],
  mixed: [
    'OpenSearch 한글 형태소', 'postgres RLS example', 'vue-i18n 한국어 설정',
    'golang http timeout', 'Nori tokenizer compound', 'redis rate limiter pattern',
    'playwright trace viewer', 'protobuf breaking change', 'embedded postgres port',
    'vite build slow',
  ],
};

// V-06-2 acceptance thresholds.
export const THRESHOLDS = {
  domesticKrRatio: 0.6, // domestic group top-10 Korean-host ratio (.kr heuristic)
  zeroRate: 0.05, // whole matrix zero-result share
  koTitleRatio: 0.8, // hangul-majority titles for Korean groups (heuristic)
  enTitleRatio: 0.7, // ascii-majority titles for mixed group (heuristic)
  maxDuplicates: 0, // same normalized URL twice in one top-10
  p95Ms: 3000, // gateway 8s timeout envelope
};

function percentile(sorted, p) {
  if (!sorted.length) return 0;
  return sorted[Math.min(sorted.length - 1, Math.ceil((p / 100) * sorted.length) - 1)];
}

function hangulRatio(text) {
  const chars = [...String(text || '')].filter(c => c.trim());
  if (!chars.length) return 0;
  return chars.filter(c => /[\uAC00-\uD7AF\u1100-\u11FF\u3130-\u318F]/.test(c)).length / chars.length;
}

function asciiRatio(text) {
  const chars = [...String(text || '')].filter(c => c.trim());
  if (!chars.length) return 0;
  return chars.filter(c => c.codePointAt(0) < 128).length / chars.length;
}

// Pure evaluation: results = [{query, group, hits: [{title, url}], latencyMs}].
// Returns {pass, checks: [{name, actual, threshold, pass}]}.
export function evaluateRun(results) {
  const checks = [];
  const zero = results.filter(r => r.hits.length === 0).length;
  const zeroRate = results.length ? zero / results.length : 1;
  checks.push({ name: 'zero_rate', actual: zeroRate, threshold: THRESHOLDS.zeroRate, pass: zeroRate <= THRESHOLDS.zeroRate });

  let duplicates = 0;
  for (const r of results) {
    const seen = new Set();
    for (const h of r.hits.slice(0, 10)) {
      const key = normalizeURL(h.url);
      if (seen.has(key)) duplicates++;
      seen.add(key);
    }
  }
  checks.push({ name: 'duplicates', actual: duplicates, threshold: THRESHOLDS.maxDuplicates, pass: duplicates <= THRESHOLDS.maxDuplicates });

  const domestic = results.filter(r => r.group === 'domestic');
  const domesticHits = domestic.flatMap(r => r.hits.slice(0, 10));
  const krHosts = domesticHits.filter(h => {
    try {
      return new URL(h.url).hostname.toLowerCase().endsWith('.kr');
    } catch {
      return false;
    }
  }).length;
  const domesticKrRatio = domesticHits.length ? krHosts / domesticHits.length : 0;
  checks.push({ name: 'domestic_kr_ratio', actual: domesticKrRatio, threshold: THRESHOLDS.domesticKrRatio, pass: domesticKrRatio >= THRESHOLDS.domesticKrRatio });

  const koGroups = results.filter(r => ['domestic', 'news', 'shopping'].includes(r.group));
  const koTitles = koGroups.flatMap(r => r.hits.slice(0, 10).map(h => h.title));
  const koRatio = koTitles.length ? koTitles.filter(t => hangulRatio(t) >= 0.3).length / koTitles.length : 0;
  checks.push({ name: 'ko_title_ratio', actual: koRatio, threshold: THRESHOLDS.koTitleRatio, pass: koRatio >= THRESHOLDS.koTitleRatio });

  const mixedTitles = results.filter(r => r.group === 'mixed').flatMap(r => r.hits.slice(0, 10).map(h => h.title));
  const enRatio = mixedTitles.length ? mixedTitles.filter(t => asciiRatio(t) >= 0.5).length / mixedTitles.length : 0;
  checks.push({ name: 'en_title_ratio', actual: enRatio, threshold: THRESHOLDS.enTitleRatio, pass: enRatio >= THRESHOLDS.enTitleRatio });

  const latencies = results.map(r => r.latencyMs).sort((a, b) => a - b);
  const p95 = percentile(latencies, 95);
  checks.push({ name: 'p95_ms', actual: p95, threshold: THRESHOLDS.p95Ms, pass: p95 <= THRESHOLDS.p95Ms });

  return { pass: checks.every(c => c.pass), checks };
}

export function parseArgs(argv) {
  const args = { provider: null, live: false, baseUrl: null, outDir: 'work' };
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--provider') args.provider = argv[++i];
    else if (argv[i] === '--live') args.live = true;
    else if (argv[i] === '--base-url') args.baseUrl = argv[++i];
    else if (argv[i] === '--out') args.outDir = argv[++i];
    else if (argv[i] === '--dry-run') args.live = false;
    else throw new Error(`unknown argument: ${argv[i]}`);
  }
  return args;
}

function printPlan(provider) {
  const groups = Object.entries(QUERY_GROUPS);
  const total = groups.reduce((n, [, qs]) => n + qs.length, 0);
  const lines = [
    `provider acceptance plan (RV-06 / V-06): ${provider}`,
    `queries: ${total} (${groups.map(([g, qs]) => `${g}=${qs.length}`).join(', ')})`,
    `request: GET /api/v2/search?q=...&page=1&safe_search=moderate`,
    `thresholds: ${JSON.stringify(THRESHOLDS)}`,
    'heuristic limits: .kr suffix is a narrow domestic proxy (top hostnames are',
    'recorded for human judgment); title-language ratios are script heuristics.',
    'run with --live --base-url <gateway> to execute against a running gateway.',
  ];
  return lines.join('\n');
}

async function runLive(provider, baseUrl, outDir) {
  const { writeFileSync, mkdirSync } = await import('node:fs');
  const results = [];
  for (const [group, queries] of Object.entries(QUERY_GROUPS)) {
    for (const query of queries) {
      const target = new URL('/api/v2/search', baseUrl);
      target.search = new URLSearchParams({ q: query, page: '1', safe_search: 'moderate' }).toString();
      const started = Date.now();
      let payload;
      try {
        const response = await fetch(target, { signal: AbortSignal.timeout(10000) });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        payload = await response.json();
      } catch (error) {
        return { fatal: `query failed [${group}] ${query}: ${error.message}` };
      }
      const hits = Array.isArray(payload.hits) ? payload.hits.slice(0, 10).map(h => ({ title: h.title, url: h.url })) : [];
      results.push({ query, group, hits, latencyMs: Date.now() - started, source: payload.source, total: payload.total });
    }
  }
  const evaluation = evaluateRun(results);
  mkdirSync(outDir, { recursive: true });
  const stamp = new Date().toISOString().replace(/[:.]/g, '-');
  const evidencePath = `${outDir}/provider-acceptance-${provider}-${stamp}.json`;
  const topHosts = {};
  for (const r of results) {
    for (const h of r.hits) {
      try {
        const host = new URL(h.url).hostname.toLowerCase();
        topHosts[host] = (topHosts[host] || 0) + 1;
      } catch { /* ignore */ }
    }
  }
  writeFileSync(evidencePath, JSON.stringify({ provider, baseUrl, at: new Date().toISOString(), thresholds: THRESHOLDS, evaluation, topHosts, results }, null, 2));
  return { evaluation, evidencePath };
}

async function main(argv) {
  let args;
  try {
    args = parseArgs(argv);
  } catch (error) {
    console.error(`misconfigured: ${error.message}`);
    process.exitCode = 2;
    return;
  }
  if (!args.provider) {
    console.error('misconfigured: --provider <name> is required');
    process.exitCode = 2;
    return;
  }
  if (!args.live) {
    console.log(printPlan(args.provider));
    return;
  }
  if (!args.baseUrl) {
    console.error('misconfigured: --live requires --base-url <gateway>');
    process.exitCode = 2;
    return;
  }
  try {
    await fetch(new URL('/healthz', args.baseUrl), { signal: AbortSignal.timeout(5000) });
  } catch {
    console.error(`misconfigured: gateway unreachable at ${args.baseUrl}`);
    process.exitCode = 2;
    return;
  }
  const run = await runLive(args.provider, args.baseUrl, args.outDir);
  if (run.fatal) {
    console.error(`failed: ${run.fatal}`);
    process.exitCode = 1;
    return;
  }
  console.log(JSON.stringify({ provider: args.provider, pass: run.evaluation.pass, checks: run.evaluation.checks, evidence: run.evidencePath }, null, 2));
  process.exitCode = run.evaluation.pass ? 0 : 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) await main(process.argv.slice(2));
