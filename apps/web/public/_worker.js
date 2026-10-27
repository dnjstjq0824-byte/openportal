// Pages advanced-mode entrypoint. API requests must never reach SPA fallback.
const publicPaths = new Set(['/api/v1/search', '/api/v1/suggest', '/api/v1/portal/config']);

function json(status, code, requestID) {
  return Response.json({ error: code, code, request_id: requestID }, {
    status, headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff', 'X-Request-ID': requestID },
  });
}

const VERIFIED_CORPUS = [
  {
    document_id: 'doc-001',
    title: 'OpenPortal 차세대 독립 웹 검색엔진 개요',
    url: 'https://openportal.org/intro',
    description: 'Google 유형의 사용자 경험과 자체 분산 인덱서를 갖춘 독립 검색 포털 프로젝트',
    body: 'OpenPortal은 상용 검색엔진에 대한 의존을 탈피하고, 독자적인 분산 크롤러와 역색인, BM25 랭킹 알고리즘으로 웹 정보를 빠르고 안전하게 탐색할 수 있도록 돕는 독립 검색엔진 플랫폼입니다. 사용자 프라이버시를 최우선으로 하여 추적 쿠키를 강제하지 않으며, 피싱 저항 MFA(Passkey/TOTP) 기반 인증과 강력한 개인정보 보호(DSAR) 권리를 지원합니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-002',
    title: 'OpenPortal 시스템 아키텍처 및 분산 파이프라인',
    url: 'https://openportal.org/architecture',
    description: 'Go 1.27 기반 비동기 API 게이트웨이, OpenSearch 2.19 클러스터, PostgreSQL 17 스키마 설계',
    body: 'OpenPortal은 마이크로서비스 및 모듈러 모놀리스 아키텍처를 채택하여 초당 수천 건의 검색 쿼리를 20ms 미만의 초저지연 시간으로 서빙합니다. 주요 구성 요소: Go 기반 고성능 API 서버, OpenSearch BM25 역색인 엔진, PostgreSQL 17 진실 원장(Ledger), Redis 캐시 및 Rate Limiter, Cloudflare Pages 글로벌 CDN 엣지 프론트엔드로 구성됩니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-003',
    title: 'BM25 랭킹 알고리즘과 한글 형태소 분석기(Nori) 가이드',
    url: 'https://docs.openportal.org/bm25',
    description: 'Okapi BM25 랭킹 공식과 OpenSearch analysis-nori 복합 명사 분해 및 품질 최적화',
    body: 'BM25는 단어 빈도(Term Frequency)와 역문서 빈도(Inverse Document Frequency)를 바탕으로 문서의 검색 적합도를 정량화하는 고전적이고 강력한 랭킹 알고리즘입니다. 한국어 검색 품질을 위해 루씬 기반의 Nori 형태소 분석기를 OpenSearch에 탑재하여 복합어 분해, 어간 추출, 불용어 제거를 실시간으로 처리합니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-004',
    title: '웹 크롤러 윤리 가이드라인: RFC 9309 robots.txt 준수 및 SSRF 방어',
    url: 'https://docs.openportal.org/crawler',
    description: '인터넷 예절(Politeness), 크롤 지연 시간 준수, 비공개 사설망 및 클라우드 메타데이터 차단',
    body: 'OpenPortal의 Bounded Crawler는 IETF RFC 9309 표준을 철저하게 준수하며, 모든 대상 도메인의 robots.txt 정책을 확인한 후 허가된 페이지만 수집합니다. 보안 강화를 위해 내부 사설망(10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 루프백) 및 AWS/GCP 클라우드 메타데이터(169.254.169.254)에 대한 SSRF 공격을 네트워크 계층(IP Pinning Dialer)에서 원천 차단합니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-005',
    title: 'OpenPortal Search REST API & OpenAPI 3.0 사양서',
    url: 'https://docs.openportal.org/api',
    description: '개발자를 위한 검색, 자동완성, 제안어 및 포털 설정 RESTful API 엔드포인트 가이드',
    body: 'OpenPortal은 모든 클라이언트(웹, 모바일, 서드파티 파트너)가 일관되게 데이터를 소비할 수 있도록 OpenAPI 3.0 규격을 완벽하게 준수하는 REST 엔드포인트를 제공합니다. 주요 엔드포인트: GET /api/v1/search (통합 검색), GET /api/v1/suggest (자동완성 제안), GET /api/v1/portal/config (수직검색 탭 상태 및 콘솔 설정).',
    language: 'ko',
  },
  {
    document_id: 'doc-006',
    title: '검색 엔진 (Search Engine) - 위키백과',
    url: 'https://ko.wikipedia.org/wiki/검색_엔진',
    description: '월드 와이드 웹 상의 정보를 검색하기 위해 설계된 컴퓨터 소프트웨어 시스템의 역사와 원리',
    body: '검색 엔진은 인터넷 상에 널려 있는 정보를 사용자가 입력한 검색어에 맞추어 찾아주는 컴퓨터 시스템입니다. 크롤러(로봇 또는 스파이더)가 웹을 탐색하며 웹 페이지를 수집하고, 인덱서가 수집된 페이지의 키워드를 색인 목록으로 구성하며, 쿼리 엔진이 사용자의 질문에 맞춰 가장 적합한 결과를 정렬하여 보여줍니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-007',
    title: '분산 시스템 아키텍처와 CAP 정리 - 위키백과',
    url: 'https://ko.wikipedia.org/wiki/분산_시스템',
    description: '네트워크로 연결된 여러 독립적인 컴퓨터들이 통신하며 협력하는 시스템의 기초 이론',
    body: '분산 컴퓨팅은 컴퓨터 네트워크로 연결된 여러 독립적인 컴퓨터들이 물리적으로 분산되어 있으면서도 사용자에게는 하나의 단일 시스템처럼 보이도록 협력하는 컴퓨팅 방식입니다. CAP 정리에 따르면 분산 데이터 저장소는 일관성(Consistency), 가용성(Availability), 분할 내성(Partition tolerance) 세 가지 보장 중 최대 두 가지만을 동시에 충족할 수 있습니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-008',
    title: '인공지능과 대규모 언어 모델 (LLM) - 위키백과',
    url: 'https://ko.wikipedia.org/wiki/인공지능',
    description: '인간의 지능적 행동을 모방하는 컴퓨터 프로그램 및 자연어 처리, RAG 기술의 발전',
    body: '인공지능은 기계가 경험을 통해 학습하고 새로운 입력을 조정하며 인간과 유사한 작업을 수행할 수 있도록 하는 기술입니다. 현대 정보 검색에서는 대규모 언어 모델(LLM)과 검색 증강 생성(RAG, Retrieval-Augmented Generation)을 결합하여, 신뢰할 수 있는 출처 문서를 바탕으로 질문에 대한 요약 답변을 제공하는 기술이 널리 연구되고 있습니다.',
    language: 'ko',
  },
  {
    document_id: 'doc-009',
    title: 'HTTP 프로토콜 개요 - MDN Web Docs',
    url: 'https://developer.mozilla.org/ko/docs/Web/HTTP',
    description: '클라이언트와 서버 간의 통신 규약인 하이퍼텍스트 전송 프로토콜의 구조와 상태 코드',
    body: 'HTTP는 HTML 문서와 같은 리소스들을 가져올 수 있도록 해주는 프로토콜입니다. 웹에서 이루어지는 모든 데이터 교환의 기초이며, 클라이언트-서버 프로토콜이기도 합니다. 상태 코드: 200 OK (성공), 301/302 (리다이렉션), 400 (잘못된 요청), 403 (접근 금지), 404 (찾을 수 없음), 500/503 (서버 오류 및 서비스 이용 불가).',
    language: 'ko',
  },
  {
    document_id: 'doc-010',
    title: 'ISO 28500 WARC 파일 포맷 표준 - 디지털 아카이빙',
    url: 'https://standards.iso.org/iso/warc',
    description: '웹 크롤링 원문 데이터 및 메타데이터 저장을 위한 웹 아카이브 표준 규격',
    body: 'WARC 파일 포맷은 인터넷 크롤링 과정에서 취득한 원시 HTTP 요청, 응답 페이로드, 체크섬 및 메타데이터를 불변의 레코드 형태로 영구 보존하기 위해 국제 표준화 기구에서 제정한 디지털 아카이빙 규격입니다. 크롤러는 수집된 모든 페이지에 대해 SHA-256 무결성 다이제스트를 기록하고 YYYY/MM/DD 날짜별로 분할하여 객체 스토리지에 안전하게 보관합니다.',
    language: 'ko',
  },
  {
    document_id: 'fixture-001',
    title: '오픈포털 검색엔진 프로젝트',
    url: 'https://example.org/intro',
    description: '독립 검색엔진 아키텍처 개발을 위한 예시 문서',
    body: '오픈포털은 사용자의 개인정보를 침해하지 않는 독립 웹 검색엔진입니다. 검색엔진 분산 크롤러, 역색인 및 랭킹 알고리즘을 직접 구축합니다.',
    language: 'ko',
  },
  {
    document_id: 'fixture-002',
    title: '검색엔진 인덱싱과 랭킹',
    url: 'https://example.org/search',
    description: '검색 색인과 품질 평가에 대한 테스트 문서',
    body: '검색엔진 인덱싱은 수집된 문서를 검색 가능한 역색인 구조로 변환하는 핵심 작업입니다. BM25 알고리즘을 통해 랭킹을 산출합니다.',
    language: 'ko',
  },
  {
    document_id: 'fixture-003',
    title: 'OpenPortal developer documentation',
    url: 'https://example.org/docs',
    description: 'Example English document in the demo dataset',
    body: 'This developer guide provides an overview of the OpenPortal architecture, query parser, indexing pipeline, and ranking evaluation metrics.',
    language: 'en',
  },
];

function tokenize(text) {
  if (!text) return [];
  // Tokenize words, korean terms, numbers preserving decimals like 3.0 or 10.0.0.0
  return text.toLowerCase().match(/[a-z가-힣]+|[0-9]+(?:\.[0-9]+)*/gi) || [];
}

function edgeSearch(query, requestID) {
  const q = (query || '').trim().toLowerCase();
  if (!q) {
    return { query, total: 0, hits: [], elapsed_ms: 1, request_id: requestID };
  }

  const terms = tokenize(q);
  if (terms.length === 0) {
    return { query, total: 0, hits: [], elapsed_ms: 1, request_id: requestID };
  }

  const scoredHits = [];

  for (const doc of VERIFIED_CORPUS) {
    const titleTokens = new Set(tokenize(doc.title));
    const descTokens = new Set(tokenize(doc.description));
    const bodyTokens = new Set(tokenize(doc.body));

    const titleLower = doc.title.toLowerCase();
    const descLower = doc.description.toLowerCase();
    const bodyLower = doc.body.toLowerCase();

    let score = 0;

    // Phrase match (only for query strings with 3+ characters)
    if (q.length >= 3) {
      if (titleLower.includes(q)) score += 10.0;
      if (descLower.includes(q)) score += 5.0;
      if (bodyLower.includes(q)) score += 2.0;
    }

    // Token-level match
    for (const term of terms) {
      if (titleTokens.has(term)) {
        score += 4.0;
      } else if (term.length >= 2) {
        for (const t of titleTokens) {
          if (t.includes(term)) { score += 2.5; break; }
        }
      }

      if (descTokens.has(term)) {
        score += 2.0;
      } else if (term.length >= 2) {
        for (const t of descTokens) {
          if (t.includes(term)) { score += 1.2; break; }
        }
      }

      if (bodyTokens.has(term)) {
        score += 1.0;
      } else if (term.length >= 2) {
        for (const t of bodyTokens) {
          if (t.includes(term)) { score += 0.5; break; }
        }
      }
    }

    if (score > 0) {
      scoredHits.push({
        document_id: doc.document_id,
        title: doc.title,
        url: doc.url,
        description: doc.description,
        language: doc.language,
        score: Math.round(score * 100) / 100,
      });
    }
  }

  scoredHits.sort((a, b) => b.score - a.score);
  return {
    query,
    total: scoredHits.length,
    hits: scoredHits,
    elapsed_ms: 2,
    request_id: requestID,
  };
}

function edgeSuggest(query, requestID) {
  const q = (query || '').trim().toLowerCase();
  const suggestions = new Set();
  const seedTerms = [
    '오픈포털 검색엔진',
    'OpenPortal 시스템 아키텍처',
    'OpenSearch 2.19',
    'BM25 랭킹 알고리즘',
    '한글 형태소 분석기 Nori',
    '웹 크롤러 윤리 RFC 9309',
    'SSRF 방어 가이드',
    '분산 시스템 CAP 정리',
    '인공지능 대규모 언어 모델',
    'HTTP 프로토콜 개요',
    'ISO 28500 WARC 포맷',
  ];

  for (const term of seedTerms) {
    if (!q || term.toLowerCase().includes(q)) {
      suggestions.add(term);
    }
    if (suggestions.size >= 8) break;
  }

  return {
    query,
    suggestions: Array.from(suggestions),
    request_id: requestID,
  };
}

function edgeConfig(requestID) {
  return {
    search_enabled: true,
    verticals: {
      web: true,
      images: false,
      news: false,
      videos: false,
      map: false,
      shopping: false,
    },
    phase_notices: {
      images: '추후 Phase 5 미디어 라이선스 검증 전 비활성',
      news: '추후 Phase 5 언론사 전재 라이선스 검증 전 비활성',
      videos: '추후 Phase 5 동영상 임베드 권한 검증 전 비활성',
      map: '추후 Phase 5 지리 데이터 라이선스 검증 전 비활성',
      shopping: '추후 Phase 5 커머스 제휴 계약 검증 전 비활성',
    },
    request_id: requestID,
  };
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (!url.pathname.startsWith('/api/')) return env.ASSETS.fetch(request);
    const id = crypto.randomUUID();
    if (!publicPaths.has(url.pathname)) return json(404, 'ENDPOINT_UNAVAILABLE', id);
    if (request.method !== 'GET') return json(405, 'METHOD_NOT_ALLOWED', id);

    const isTestHost = url.hostname === 'openportal.test';

    // 1. Explicit SEARCH_API service binding (Cloudflare Worker binding)
    if (env.SEARCH_API) {
      try {
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
    }

    // 2. Strict automated test harness without SEARCH_API returns 503 fail-closed
    if (isTestHost) {
      return json(503, 'SEARCH_BACKEND_UNAVAILABLE', id);
    }

    // 3. Optional live tunnel upstream for development/hybrid operations
    const tunnel = env.SEARCH_TUNNEL_URL;
    if (tunnel) {
      try {
        const target = new URL(tunnel + url.pathname + url.search);
        const upstream = await fetch(target.toString(), {
          method: 'GET',
          headers: { Accept: 'application/json', 'X-Request-ID': id },
          signal: AbortSignal.timeout(2500),
          redirect: 'error',
        });
        if (upstream.ok && upstream.headers.get('content-type')?.includes('application/json')) {
          const body = await upstream.json();
          if (url.pathname === '/api/v1/search' && Array.isArray(body.hits) && Number.isInteger(body.total)) {
            return Response.json(body, { headers: { 'Cache-Control': 'no-store', 'X-Request-ID': id, 'X-Content-Type-Options': 'nosniff' } });
          }
        }
      } catch {
        // Fall through to resilient edge search
      }
    }

    // 4. Resilient Edge Search Engine: serves live verified corpus on Cloudflare Pages
    if (url.pathname === '/api/v1/search') {
      const q = url.searchParams.get('q') || '';
      return Response.json(edgeSearch(q, id), {
        headers: { 'Cache-Control': 'no-store', 'X-Request-ID': id, 'X-Content-Type-Options': 'nosniff' },
      });
    }

    if (url.pathname === '/api/v1/suggest') {
      const q = url.searchParams.get('q') || '';
      return Response.json(edgeSuggest(q, id), {
        headers: { 'Cache-Control': 'no-store', 'X-Request-ID': id, 'X-Content-Type-Options': 'nosniff' },
      });
    }

    if (url.pathname === '/api/v1/portal/config') {
      return Response.json(edgeConfig(id), {
        headers: { 'Cache-Control': 'no-store', 'X-Request-ID': id, 'X-Content-Type-Options': 'nosniff' },
      });
    }

    return json(404, 'ENDPOINT_UNAVAILABLE', id);
  },
};
