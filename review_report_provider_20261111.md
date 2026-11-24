# 검토 보고서 — OpenScan(OpenPortal) 검색 제공자 정정안 검토

- 발행: 2026-11-11 (개발 에이전트 검토분)
- 상위 지시: 작업지시 프롬프트 — 제공자 정정안 검토 전용 (2026-11-11, 기획·QA 총괄)
- 선행 문서: 작업지시_프롬프트_M0_집행.md (2026-10-31 승인분, 본 검토는 대체가 아닌 정정 검토)
- 방법: 코드 읽기 + 공식 문서 읽기 + 읽기 전용 확인. 코드 수정 0건.
- 표기 규칙: 직접 확인한 것만 판정 근거로 사용. 확인하지 못한 것은 "미확인"으로 표기. 사실과 소견을 구분 표기.

## 총괄 판정표

| 항목 | 제목 | 판정 | 한 줄 근거 |
|---|---|---|---|
| R-01 | 현 제공자 구조 실태 | 조건부 | 부품 교체 전제 부분 성립, Brave 전용 분기가 3개 파일에 산재 |
| R-02 | 네이버 오픈API 연동 | 조건부 | 기술 매핑 가능, 개정 약관 조건 수용 + 쇼핑 존속 확인 필요 |
| R-03 | Tavily 연동 | 불가 | 페이지네이션·버티컬·total이 없어 일반 검색 UI 1차 제공자 불가 |
| R-04 | 자체 인덱스 범위 | 조건부 | 크롤러 실구현 확인, allowlist·스키마 확장이 전제 |
| R-05 | 종전 승인 조건 유효성 | 조건부 | SE-00·C-12 해소 확인, SE-02 실물 미확인 |
| R-06 | 개발 착수 영향 산정 | 가능 | 수정 약 19개·신규 2~4개, 50개 미만으로 분리 불필요 |
| R-07 | 리스크·누락 | 조건부 | 착수 전 해소 조건 3건 존재 |

정정안 전체에 대한 종합 소견: 1차 제공자를 "네이버(국내)+Tavily(글로벌)" 무료 조합으로 교체한다는 기획 결정 중
네이버 축은 조건부 성립, Tavily 축은 일반 검색 UI 용도로 불성립이다. 글로벌 축은 대안(Exa·Serper·Brave 유지
중 택1) 재선정이 필요하다. 아래 항목별 판정을 따른다.

---

## R-01. 현 제공자 구조 실태 — 판정: 조건부 (부분 가능)

### 실물 구조

검색 제공자 선택은 환경변수 1개로 분기된다. 기본값은 자체 인덱스다.

- `gateway/config.mjs:12` — `SEARCH_PROVIDER` 기본값 `'opensearch'`
- `gateway/config.mjs:16-17` — `'bing'` 지정 시 부팅 거부(퇴역 반영), 허용값은
  `opensearch`·`local`·`brave`·`hybrid` 4종
- `gateway/server.mjs:347-380` — `/api/v2/search` 분기: `brave` 단독, `hybrid`(자체+Brave RRF),
  그 외는 `SEARCH_BACKEND_URL` 자체 백엔드
- `gateway/search.mjs:74-147` — `braveSearch()` (Brave `/res/v1/web/search` 호출, 키+승인 없으면
  `SEARCH_NOT_CONFIGURED` 503)
- `gateway/search.mjs:149-151` — `bingSearch()`는 퇴역 스텁(호출 시 `SEARCH_PROVIDER_RETIRED` 503)
- `gateway/search.mjs:153-217` — `hybridSearch()` (자체+외부 병렬 호출 후 `rrfMerge` 병합)
- `.env.example:60-75` — 제공자 설정 문서화

### Brave 연동 현 상태: 코드 활성, 기본 비활성

- 사실: Brave 호출 코드는 살아 있다. 그러나 기본 제공자는 `opensearch`이며, Brave 경로를 타려면
  `BRAVE_SEARCH_API_KEY`와 `BRAVE_API_TERMS_APPROVED=true`가 모두 필요하다
  (`gateway/config.mjs:22-32`, `gateway/server.mjs:348`).
- 사실: 키·승인 미설정 시 `brave` 모드는 503, `hybrid` 모드는 Brave 없이 자체 결과로만 동작한다
  (`gateway/search.mjs:183-190`).

### 제공자 추상화 존재 여부: 없음 (Brave 하드코딩 결합)

- 사실: `braveSearch`·`braveKey`·`braveApproved`·`brave-daily`·`brave-per-second`라는 Brave 전용
  식별자가 `gateway/config.mjs:22-32`, `gateway/search.mjs:74-190`, `gateway/server.mjs:347-365`
  3개 파일에 산재해 있다. 제공자 인터페이스·팩토리·레지스트리 같은 추상화는 없다.
- 사실: UI도 제공자 문자열에 직접 분기한다. `apps/web/src/views/SearchView.vue:33-40`은
  `source`가 `brave-api`·`bing-api`·`hybrid-rrf`인지로 배지·안내를 가른다.
- 소견: 그럼에도 교체 접합면(seam)은 좁다. 외부 제공자의 출력 계약은
  `{title, url, description, source}` 4개 필드로 단일화되어 있고(`gateway/search.mjs:133-138`),
  UI는 `title`·`url`·`description`·`site_name`만으로 렌더링한다
  (`apps/web/src/stores/search.ts:5-12`, `apps/web/src/views/SearchView.vue:232-246`).
  새 제공자는 (1) `*Search()` 함수 1개, (2) config 승인·한도 항목, (3) server 분기 1개,
  (4) UI source 라벨의 4점 접합으로 추가 가능하다.

### 판정: 조건부 (부분 가능)

"제공자는 교체 가능한 부품"이라는 전제는 부분 성립한다. 출력 계약은 범용이나 제공자 추상화가
없어 교체 시 Brave 전용 분기를 함께 손봐야 한다. 정정안 개발 시 제공자 추상화 도입(어댑터 분리)을
선행 조건으로 권고한다.

---

## R-02. 네이버 오픈API 연동 가능성 — 판정: 조건부

### R-02-1. 버티컬 매핑표

OpenScan 현 버티컬 구성 근거: `backend/internal/search/normalize.go:62-69`
(`web`만 활성, `image`·`news`·`video`·`map`·`shopping`은 Phase 5 게이트로 비활성).
`backend/cmd/api/handlers_search.go:38-48`은 `vertical=web` 외 요청에 501을 반환한다.

| OpenScan 버티컬 | 상태 | 네이버 카테고리 | 정합성 |
|---|---|---|---|
| web | 활성 | webkr(웹문서) | 적합 |
| news | 게이트 | news(뉴스) | 적합 |
| image | 게이트 | image(이미지) | 적합 |
| shopping | 게이트 | shop(쇼핑) | 미확인 (아래 주1) |
| map | 게이트 | local(지역) | 부분 (좌표→지도 렌더러 별도 필요) |
| video | 게이트 | 없음 | 불가 (네이버에 동영상 검색 API 없음) |
| (없음) | — | blog(블로그) | 신규 버티컬로만 수용 가능 |
| (없음) | — | cafearticle·kin·encyc | 신규 버티컬로만 수용 가능 |

- 주1 (사실+미확인): 네이버 쇼핑 검색 API가 2026-07-31에 종료되었다는 단일 출처 주장이 있다
  (scrapebadger, 2026-10). 책 검색 API 종료(2026-07-31)는 약관 부칙 근거로 별도 확인된다
  (canuseit). 쇼핑 카테고리 존속 여부는 공식 문서로 재확인이 필요하므로 미확인으로 표기한다.
- 호출 규격 (사실, developers.naver.com 검색 API 문서, 확인일 2026-11-11):
  `display` 1~100(기본 10), `start` 1~1000, `sort` sim(관련도)/date(날짜순).
  현 UI 페이지네이션(페이지당 10, 최대 100페이지, `SearchView.vue:249-259`)과 정합한다.
- 한도 (사실, 공식 문서): 검색 API 일 25,000회, 클라이언트 ID별 합산, 헤더 인증
  (`X-Naver-Client-Id`·`X-Naver-Client-Secret`), 앱 등록 시 휴대폰 인증 필요.

### R-02-2. 응답 필드 대조표

현 UI 표시 필드 근거: `apps/web/src/stores/search.ts:5-12`
(`title`·`url`·`description` 필수, `site_name`·`source`·`document_id` 선택),
렌더링은 제목·도메인 크럼브·사이트명·요약 스니펫(`SearchView.vue:232-246`). 썸네일 표시는 없다.

| 네이버 카테고리 | 네이버 응답 필드 | UI 매핑 | 부족·가공 필요 |
|---|---|---|---|
| webkr | title·link·description | title·url·description 직접 대응 | `title`·`description`의 `<b>` 하이라이트 태그 제거 필요 |
| news | title·originallink·link·description·pubDate | link=네이버뉴스, originallink=원문 | 둘 중 표시 URL 정책 결정 필요, pubDate 표시 UI 없음(추가 시 소규모) |
| blog | title·link·description·bloggername·bloggerlink·postdate | bloggername→site_name 가능 | postdate는 YYYYMMDD 일자 정밀도뿐(시각 없음) |
| image | title·link·thumbnail·sizeheight·sizewidth | thumbnail 표시 UI 없음 | 이미지 버티컬 UI 신규 필요, 현 web UI에 그대로 투입 불가 |
| shop | title·link·image·lprice·hprice·mallName·productId | 가격·몰명 표시 UI 없음 | 쇼핑 버티컬 UI 신규 필요 (`ProductOffer` 구조와 별도 매핑) |
| local | title·link·category·telephone·address·roadAddress·mapx·mapy | 주소·전화 표시 UI 없음 | 지도 렌더러·POI UI 신규 필요 |

- 공통 (사실): 네이버 응답에는 `total`·`start`·`display`가 포함되어 현 `total: null` 문제(R-05)를
  우회할 수 있다. `document_id`는 게이트웨이가 생성하면 된다.
- 공통 (소견): web·news·blog는 현 web UI에 투입 가능하고, image·shop·local은 버티컬 UI 신규가
  필요하다. 즉 네이버 연동만으로 버티컬이 열리는 것이 아니라 버티컬별 UI 작업이 동반된다.

### R-02-3. 약관 검토

- 사실 (2차 출처, digitaltoday 2026-09 보도·europesays 영문 보도, 확인일 2026-11-11):
  네이버 개발자센터 검색 API 개정 약관이 2026-09-07경 시행되었고, (1) 검색 결과를 보여주는
  목적으로만 이용, (2) API 데이터의 AI 입력·모델 학습·개선·평가 금지, (3) 제3자 제공·판매 금지,
  (4) 결과 표시 페이지 광고 부착 금지, (5) API 이용 별도 수익 금지라는 5개 제한을 포함한다.
  약관 원문은 본 검토에서 직접 열람하지 못했으므로, 조항 번호·문구는 미확인으로 표기하고
  착수 전 원문 대조를 조건으로 건다.
- 종전 승인 조건(캐싱 금지)과의 충돌 여부 (소견): 충돌하지 않고 방향이 일치한다. 개정 약관은
  표시 목적 외 이용을 금지하는 취지이므로, 결과 저장·캐싱을 하지 않는 종전 조건을 유지하는 쪽이
  약관 친화적이다. 단 "표시 목적의 일시 캐시" 허용 여부는 약관 원문 미확인 사항이다.
- 상업적 이용·제휴 신청 요건 (소견): 개발자센터 검색 API는 비상업·표시 목적 원칙으로 보이며,
  상업 서비스 단계에서는 별도 제휴·계약이 필요할 가능성이 크다. M0 단계(개발·검증 규모, 대외
  수익 없음)는 제휴 대상이 아닌 것으로 보이나, 네이버 측 확인 없이 단정할 수 없으므로
  미확인(착수 전 확인 필요)으로 표기한다.

### 판정: 조건부

기술 연동은 가능하나 (1) 개정 약관 5개 제한의 수용, (2) 쇼핑 카테고리 존속의 공식 확인,
(3) 약관 원문 대조(캐시 허용 범위·M0 적용 여부)를 조건으로 건다.

---

## R-03. Tavily 연동 가능성 — 판정: 불가

### R-03-1. 일반 검색 UI 적합성 (응답 형식 기준)

- 사실 (공식 문서 docs.tavily.com/documentation/api-reference/endpoint/search, 확인일 2026-11-11):
  `POST /search`는 `{query, answer?, images?, results[{title, url, content, score,
  published_date?, favicon?}], response_time, follow_up_questions?}`를 반환한다.
  요청 파라미터는 `query`·`search_depth`·`chunks_per_source`·`max_results`(최대 20)·`topic`
  (`general`·`news`·`finance`)·`include_answer`·`include_images`·`include_published_date` 등이며,
  `page`·`offset`·`start` 같은 페이지네이션 파라미터는 스키마에 없다(공식 OpenAPI 전문 대조).
- 판정 근거 (사실 대조):
  1. 페이지네이션 불가 — 현 UI는 최대 100페이지 이동(`SearchView.vue:249-259`)을 전제로 하나,
     Tavily는 최대 20개 단일 목록만 반환하고 다음 페이지 요청 방법이 없다.
  2. 이미지·뉴스 버티컬 대체 불가 — `topic=news`는 있으나 이미지 버티컬 상당 기능은 없고,
     `include_images`는 쿼리 관련 이미지 첨부 수준이다.
  3. `total` 카운트 없음 — 결과 건수 표시(`serp.about`)와 페이지 계산이 불가능하다.
  4. `content`는 RAG용 청크(기본 약 500자×최대 3개)라 스니펫 정규화 가공이 별도 필요하다.
- 결론: Tavily를 일반 검색 UI(결과 목록·페이지네이션·이미지/뉴스 버티컬)에 그대로 쓸 수 없다는
  판정이다. AI 요약·RAG 부가 기능 용도라면 별도 검토가 필요하나 본 항목 범위 밖이다.

### R-03-2. 무료 티어 한도와 크레딧 소모 규칙

- 사실 (공식 문서 크레딧 규격 + 2026-10~11월 2차 출처 교차 확인, 확인일 2026-11-11):
  무료 Researcher 플랜은 월 1,000 크레딧, 신용카드 불필요.
  소모 규칙은 `basic`·`fast`·`ultra-fast` 검색당 1 크레딧, `advanced` 검색당 2 크레딧이다.
- 소견: 월 1,000회는 개발·검증용으로도 빠듯하고(일 약 33회), 상용 트래픽의 1차 제공자로는
  용량이 성립하지 않는다.

### R-03-3. 대안 소견 (3줄 이내, 범위 확대 아님)

1. Serper.dev — Google SERP를 JSON(제목·링크·스니펫)으로 반환해 현 UI 계약과 가장 가깝고,
   일회성 2,500회 무료 후 $1/1천회 수준이다(2차 출처 기준, 착수 전 공식 확인 필요).
2. Exa — 신경망/AI 검색 API로 월 $10 크레딧 무료분이 있고 $4~7/1천회 수준이나, RAG 성향이라
   일반 UI 적합성은 별도 검증이 필요하다(2차 출처 기준).
3. Brave 유지 — 월 $5 크레딧(약 1,000회)은 Tavily 무료분과 동급이라 유료 전환 후보 강등보다
   현 연동 유지가 공수가 적다(카드 등록 필요).

### 판정: 불가

Tavily는 일반 검색 UI의 1차 제공자로 부적합하다. 정정안의 글로벌 축은 재선정이 필요하다.

---

## R-04. 자체 인덱스 범위 — 판정: 조건부

### R-04-1. SE-01 crawler 현 상태 재확인

- 종전 판정("실 fetch 미구현")은 현 코드에 부적합하다. 실측 결과 실 fetch가 구현되어 있다.
  - 사실: `backend/internal/crawler/fetcher.go:89-194`는 실제 HTTP GET(SSRF 가드·크기 제한·
    gzip bomb 가드 포함)을 수행한다.
  - 사실: `backend/internal/crawler/engine.go:105` 이하 `CrawlURL`은 allowlist 게이트→
    robots.txt(RFC 9309)→politeness→실 fetch 파이프라인이다.
  - 사실: `backend/cmd/crawler/main.go:266-313` `crawlAndIndex`는 실 수신 HTML을
    `IngestCrawlResult`로 OpenSearch에 색인하고, 빈·비색인 응답은 fail-closed로 버린다.
- 동작 전제 (사실): `CRAWL_ALLOWED_HOSTS` 미설정 시 부팅 거부(`main.go:87-90`), HTTPS 443 전용
  (`main.go:59-74`), 리다이렉트 자동 추적 미지원(`engine.go:109` `RejectRedirects=true`).
  즉 코드는 실동작하나 운영자 allowlist 없이는 1건도 수집하지 않는다.
- 시뮬레이션 색인 금지 상태 (소견): 크롤러 경로에 가짜·고정 색인 코드는 발견되지 않았다.
  단 전체 색인 경로의 전수 감사는 본 검토 범위를 초과하므로 "금지 상태 유지"는 소견으로 표기한다.

### R-04-2. OLMOA·getfortrip 색인 작업 산정 (산정만, 구현 금지)

- 미확인: `OLMOA`·`olmoa`·`getfortrip`·`fortrip` 문자열은 코드베이스 전역 검색(빌드 산출물 제외)에서
  0건이다. 상품 API 스펙·인증·호출 규격을 확인할 수 없어 상세 산정은 불가하다.
- 사실 (스키마 근거): 현 색인 `documents_v2`는 `dynamic: strict`이며 웹문서 필드만 허용한다
  (`infra/opensearch/documents_v2.json:54-123`). 가격·이미지·재고 같은 상품 필드는 수용 불가다.
- 소견 (산정): 상품 색인에는 (a) 스키마 확장 + 재색인 또는 상품 전용 인덱스 신설, (b) 상품 API
  수집 배치(신규), (c) 가격·재고 갱신 주기 설계, (d) 쇼핑 버티컬 검색·표시 연동이 필요하다.
  64개는 수동 검증 규모, 1,000개는 배치·스케줄러가 필요한 규모다.

### R-04-3. Common Crawl 한국어 초기 인덱스 소견 (단정 금지)

- 사실 (2차 출처 교차, 확인일 2026-11-11): Common Crawl은 월 1회 약 22억 페이지 규모의
  크롤을 WARC·WAT·WET 형식으로 AWS S3에 공개하고(registry.opendata.aws/commoncrawl),
  Columnar Index(Parquet)로 URL·도메인·언어 필터 조회가 가능하다. 언어 분포는 영어 편중이다
  (예: CC-MAIN-2026-43에서 영어 42.6%).
- 소견: 한국어 초기 인덱스는 (1) Columnar Index에서 한국어·한국 도메인 필터로 후보 선정,
  (2) WET(정제 텍스트) 선별 다운로드, (3) 현 파이프라인(추출→색인) 투입 순으로 실현 가능하다.
  전체 크롤(수백 TB)을 통째로 처리할 필요는 없다. 예상 기간은 필터 조건·목표 문서 수에 좌우되어
  단정할 수 없으며, 파이프라인 기구축 전제 시 수 주~수 개월 범위의 별도 산정이 필요하다.
- 소견: Common Crawl 데이터에도 원문 저작권·robots·삭제 요청 처리 등 권리 경계가 그대로
  적용되므로, 승인된 호스트 중심으로 선별 수집하는 원칙이 필요하다.

### 판정: 조건부

크롤러 실물은 동작 가능하나 allowlist 운영이 전제이고, 상품·CC 색인은 스키마·수집 작업이
선행되어야 한다. OLMOA·getfortrip 상세는 미확인이다.

---

## R-05. 종전 승인 조건의 현 유효성 — 판정: 조건부

### R-05-1. SE-00 조건 실물 재확인

- `continue-on-error: true` (사실): 현 워크플로우 2개(`.github/workflows/phase0.yml`,
  `.github/workflows/deploy-pages.yml`) 전수 확인 결과 존재하지 않는다. 제거 대상 없음(이미 해소).
- `1,280,450` 문자열 (사실): 실행 코드에 존재하지 않는다. 잔류는 2종뿐이다.
  1. `docs/archive/.../OPENSCANNER_FINAL_GO_NO_GO.md:2` 2건 — 폐기 선언문 안의 과거 언급(실행 영향 없음).
  2. `scripts/verify-acceptance.mjs:51` — 금지 문자열 검출용(등장 시 실패시키는 감사 코드, 잔재가 아님).

### R-05-2. C-12 (`total: null`) 현 코드 상태

- 사실: `total: null`은 하이브리드 설계상 정상 스펙으로 정착되어 있다.
  `gateway/search.mjs:202-205`는 외부 제공자 요청 시 합집합 크기를 알 수 없어 `total: null`을
  반환하고, `gateway/server.mjs:372`·`apps/web/src/stores/search.ts:65`·
  `apps/web/public/_worker.js:47`은 `null`을 허용 검증하며,
  `SearchView.vue:190`은 `null`일 때 건수 미상 표시로 분기한다.
- 소견: C-12는 "문제"가 아니라 해결된 설계다. 네이버 연동 시 네이버 응답의 `total`을 전달하면
  단일 제공자 모드에서는 건수 표시가 복원된다.

### R-05-3. SE-02 fail-fast 매트릭스의 제공자 교체 설계 소견 (구현 금지)

- 미확인: `SE-02`·`fail-fast 매트릭스` 명칭의 실물은 코드베이스에서 발견되지 않았다
  (전역 검색 0건). M0 집행 문서 원문이 작업 공간에 없어 대조 불가하다.
- 소견 (설계 방향): 네이버+Tavily 교체 시 fail-fast 행렬은 아래 축으로 재구성되어야 한다.
  (1) 제공자별 필수 env 존재(`NAVER_CLIENT_ID`·`SECRET`, `TAVILY_API_KEY`) × 승인 플래그,
  (2) production 부팅 거부 규칙(현 `gateway/config.mjs:7,31` 패턴 확장),
  (3) 한도 초과 시 degraded 폴백(현 `server.mjs:356-362` Brave 예산 패턴을 제공자 공통으로 일반화),
  (4) 버티컬별 제공자 가용 행렬(web/news/image/shop × naver/tavily/자체).

### 판정: 조건부

SE-00·C-12는 해소 확인, SE-02는 실물 미확인으로 M0 원문 확보 후 대조가 조건이다.

---

## R-06. 개발 착수 시 영향 범위 산정 — 판정: 가능

산정만 하며 파일을 건드리지 않았다. 규모 표기: 소(~20줄)·중(~100줄)·대(100줄 이상).

| # | 파일 | 예상 변경 내용 | 규모 |
|---|---|---|---|
| 1 | gateway/config.mjs | 네이버·신규 제공자 env+승인+한도, provider 목록 교체 | 중 |
| 2 | gateway/search.mjs | naverSearch 신규 + hybrid 분기 교체, Brave 분기 정리 | 대 |
| 3 | gateway/server.mjs | /api/v2/search 분기·레이트리밋·응답 정규화 | 중 |
| 4 | .env.example | 신규 env 문서화 | 소 |
| 5 | apps/web/src/stores/search.ts | source·필드(발행일 등) 확장 | 소 |
| 6 | apps/web/src/views/SearchView.vue | source 라벨·버티컬 탭·페이지 상한 | 중 |
| 7 | apps/web/src/i18n.ts | 버티컬 라벨·게이트 문구 | 소 |
| 8 | apps/web/public/_worker.js | 업스트림 검증 케이스 추가 | 소 |
| 9 | backend/cmd/api/handlers_search.go | vertical 501 해제·라우팅 | 중 |
| 10 | backend/cmd/api/handlers_vertical.go | 버티컬 응답 연동 | 중 |
| 11 | backend/internal/search/normalize.go | DefaultVerticalTabs 활성화 | 소 |
| 12 | backend/internal/vertical/* | 네이버 매핑 어댑터(이미지·쇼핑·지역) | 대 |
| 13 | infra/opensearch/documents_v2.json | 상품 필드 스키마 또는 전용 인덱스 | 중 |
| 14 | backend/cmd/crawler/main.go | 생태계 시드·allowlist 운영 반영 | 소 |
| 15 | tests/gateway-search.test.mjs | 제공자 모의 교체 | 중 |
| 16 | tests/platform.integration.test.mjs | 예산·폴백 테스트 교체 | 중 |
| 17 | backend/cmd/api/http_test.go | vertical 501 테스트 수정 | 중 |
| 18 | tests/pages-worker.test.mjs | 계약 테스트 추가 | 소 |
| 19 | docs/API_CONTRACT.md 등 | 제공자 계약 문서 갱신 | 소 |

신규 파일 산정: 제공자 어댑터 분리 시 `gateway/providers/naver.mjs` 등 1~2개,
매핑 테스트 1~2개, 수집 배치(상품) 1개 내외. 합계 신규 2~4개.

산정 총합: 수정 약 19개 + 신규 2~4개로 50개를 넘지 않으므로 작업 분리안은 제안하지 않는다.
단 Tavily 축이 불가 판정이므로 글로벌 제공자 재선정 후 산정을 갱신해야 한다.

### 판정: 가능

---

## R-07. 리스크·누락 — 판정: 조건부

사실과 소견을 구분해 보고한다.

1. 사실: 네이버 개정 약관은 결과 표시 페이지 광고 부착과 API 이용 별도 수익을 금지한다
   (R-02-3 근거). 소견: OpenScan 수익 모델이 검색 결과 페이지 광고를 전제한다면 정면 충돌이다.
   수익화 위치(결과 페이지 외) 확정 없이는 상용 전환이 불가하다.
2. 사실: 네이버 검색 API의 NAVER API HUB(네이버클라우드) 이관·유료 전환 추진이 보도되고 있다.
   소견: "무료 조합" 전제는 언제든 흔들릴 수 있어 제공자 교체 비용을 낮추는 추상화(R-01)가
   리스크 완화책이다.
3. 사실: 네이버는 국내·한국어 커버리지 중심이다. 소견: 글로벌 쿼리·영문 커버리지는 네이버로
   해결되지 않으므로 글로벌 축 공백(R-03 불가)이 서비스 정의에 직접 영향을 준다.
4. 사실: 무료 티어(네이버 일 25,000회·Tavily 월 1,000회)는 상업 트래픽 규모가 아니다.
   소견: M0 검증 규모에는 충분하나, 공개 전환 시점의 유료 전환 비용을 별도 산정해야 한다.
5. 미확인: 네이버 쇼핑·책 카테고리 축소 흐름이 webkr·news·blog로 확대될지 알 수 없다.
   착수 전 카테고리별 존속 확인이 필요하다.
6. 소견: 네이버 `<b>` 하이라이트·원문/네이버링크 이중 URL·일자 정밀도 같은 응답 특성은
   표시 품질·정책 결정 사항으로, 연동 후 QA 항목에 올려야 한다.

착수 전 해소 조건 3건: (a) 네이버 약관 원문 대조, (b) 쇼핑 카테고리 존속 확인,
(c) 글로벌 제공자 재선정. 이를 조건으로 판정은 조건부다.

---

## 미해결 사항

- U-01 (미확인): SE-02 fail-fast 매트릭스 실물 — M0 집행 문서 원문이 작업 공간에 없어 대조 불가.
- U-02 (미확인): 네이버 쇼핑 검색 API 존속 여부 — 종료 주장이 단일 출처라 공식 확인 필요.
- U-03 (미확인): 네이버 약관 원문(조항 번호·캐시 허용 범위·M0 적용 여부) — 2차 출처 보도만 확인.
- U-04 (미확인): OLMOA 상품 API·getfortrip 상품 데이터 스펙 — 코드베이스 언급 0건.
- U-05 (미확인): Common Crawl 한국어 선별 규모·기간 — 별도 산정 필요.

## 파일 상태표 (본 검토 작업 기인)

| 구분 | 파일 | 비고 |
|---|---|---|
| 신규 | 검토보고_제공자정정_20261111.md | 본 보고서 1개 |
| 수정 | 없음 | 0개 |
| 삭제 | 없음 | 0개 |

git 상태 원문 (보고서 작성 직후, `git status --porcelain=v1 --untracked-files=all`):

```text
 M apps/web/src/App.vue
 M apps/web/src/MailView.vue
 D apps/web/src/ScanArt.vue
 M apps/web/src/components/auth/AuthShell.vue
 M apps/web/src/i18n.ts
 M apps/web/src/portal.css
 M apps/web/src/signup-copy.ts
 M apps/web/src/views/HomeView.vue
 M apps/web/src/views/auth/LoginView.vue
 M tests/foundation.e2e.spec.mjs
 M tests/platform.e2e.spec.mjs
?? 검토보고_제공자정정_20261111.md
```

주: 위 출력에 본 검토 이전부터 존재하던 수정·삭제 항목이 포함될 수 있다. 본 검토 작업이
변경한 파일은 신규 보고서 1개뿐이며, 그 외 항목은 작업 개시 전 상태와 동일함을 개시 시점
스냅샷과 대조해 확인한다. 빌드 산출물은 생성하지 않았다.

## 참조 (확인일 2026-11-11)

코드 근거:

- gateway/search.mjs, gateway/config.mjs, gateway/server.mjs, .env.example
- apps/web/src/stores/search.ts, apps/web/src/views/SearchView.vue, apps/web/src/i18n.ts,
  apps/web/public/_worker.js
- backend/cmd/api/handlers_search.go, backend/cmd/api/handlers_vertical.go,
  backend/internal/search/normalize.go, backend/internal/vertical/news.go,
  backend/internal/vertical/shopping.go, backend/internal/vertical/federation.go
- backend/cmd/crawler/main.go, backend/internal/crawler/fetcher.go,
  backend/internal/crawler/engine.go, infra/opensearch/documents_v2.json
- .github/workflows/phase0.yml, .github/workflows/deploy-pages.yml,
  scripts/verify-acceptance.mjs, scripts/dev-local.mjs

공식 문서:

- https://developers.naver.com/docs/serviceapi/search/blog/blog.md (일 25,000회·등록 절차)
- https://docs.tavily.com/documentation/api-reference/endpoint/search (응답 스키마·크레딧 규격)
- https://developers.google.com/custom-search/v1/overview (신규 종료·2027-01-01 종료)

2차 출처 (교차 확인용):

- digitaltoday 698331·europesays (네이버 약관 개정), scrapebadger·canuseit (쇼핑·책 카테고리),
  registry.opendata.aws/commoncrawl·motherduck·shifter (Common Crawl 규모),
  edenai·mrscraper·devtoollab·exa.ai/versus/serper (Brave·Exa·Serper 요금)

