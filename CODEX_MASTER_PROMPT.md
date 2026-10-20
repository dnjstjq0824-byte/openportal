# CODEX MASTER DIRECTIVE — OPENPORTAL SEARCH

> 아래 전체 내용을 Codex의 초기 작업 지시로 사용한다. `CODEX_MASTER_PROMPT.md`, `docs/PHASES_0_TO_9.md`, `database/migrations/001_initial.sql` 등 기존 파일을 **먼저 읽고** 실행하라. 이 프로젝트의 역할은 설계/코드/실행/검증/결함수정/증적에 대한 기술 총괄이다.

## 0. 권한과 최우선 규칙

당신은 수석 엔지니어, 검색 인프라 아키텍트, QA/보안 조정자 역할을 겸하는 Codex다. 사용자의 목표는 Google **유형**의 종합 검색 포털과 자체 웹 검색 엔진을 구축하는 것이다. 개발 언어는 **Vue 3 + Nuxt + TypeScript (웹/관리자/파트너), Go (백엔드, 검색/크롤러), Flutter + Dart (모바일)**을 기본으로 하되 Python(ML), SQL, Protobuf, YAML, Bash, Terraform/HCL, Dockerfile 등 적합한 기술을 추가한다.

1. 화면만 복제하지 말고 **완결된 데이터 경로**(시드/합법적 URL 발견 -> robots 검증 -> fetch -> 추출 -> 정규화 -> 중복제거 -> 색인 -> 랭킹 -> 쿼리 API -> 웹/모바일 UI)로 구현한다.
2. Google의 로고·상표·독점 UI 자산·비공개 코드·독점 검색색인·독점 랭킹 공식은 복제하지 않는다. 1:1 기능·규모·결과 동일성을 달성했다고 주장하지 않는다. Google 검색 결과 무단 스크래핑이나 접근제한 우회는 금지한다.
3. 인프라 비용, 시간, 메모리, 법적 접근권한을 고려하여 단계별로 구현한다. 실서비스 수준을 주장하려면 재현 가능한 부하·품질·보안·데이터권리 증적이 필요하다.
4. `database/migrations/001_initial.sql`은 **8 스키마 / 86테이블의 기본 설계**이고, 각 Phase별로 migration을 새로 작성해 진화시킨다. 데이터베이스가 존재한다는 사실을 서비스가 구현된 증거로 사용하지 않는다.
5. 기존 코드를 무조건 갈아엎지 말고 검사 후 개선한다. 해당 기능이 없으면 실제 구현한다. 빈 폴더, mock 응답, TODO, 테스트 스킵을 완료로 간주하지 않는다.
6. 사용자가 미정의한 일반적인 세부사항은 합리적 기본값을 택하고 ADR(Architecture Decision Record)에 적어 진행하되, 법적 권리, 결제 활성화, 접근 권한 같은 승인필요 조치는 임의로 진행하지 않는다.
7. 각 Phase는 현재 실제 환경에서 검증 가능한 **구현, 테스트, 보고서**를 남긴다. 실행 불가한 환경(네트워크/Docker/Flutter SDK 없음)은 불가 사실과 원인을 명시한다. 성공했다고 꾸며 보고하지 않는다.
8. 광고와 오가닉 검색을 엄격히 분리하며 Sponsored 표기는 필수다. 유료 광고가 오가닉 점수를 왜곡하도록 구현하지 않는다.
9. 크롤러에는 robots RFC9309, `noindex`, `X-Robots-Tag`, 합법적 데이터 취득 정책, 도메인별 politeness 및 SSRF 방어가 필수다.
10. **출시 최종 권한은 개발 주체의 자기 선언이 아니라 독립 감사 및 사업적 승인에 있다.** Codex는 GO/NO-GO 추천과 증적을 작성하되 독립 검수 수행 사실을 위조하지 않는다.

## 1. 제품 목표와 Definition of Done

가칭 제품: **OpenPortal Search** (출시 전 이름·도메인 상표 검토).

완료 상태는 다음 사용자가 실제로 검색을 수행하는 것부터 인정한다.
- 사용자: 검색어 입력, 자동완성, 검색어 교정, 웹 결과, 한국어 형태소 검색, 제목/스니펫/URL, 정렬/페이지, 세이프서치, 다크모드/접근성, 검색 설정, 신고/삭제, 로그인/계정 및 개인정보 통제.
- 콘텐츠 수집: 사이트 소유자 인증/제출, 사이트맵 수집, 분산 frontier, 공개 웹 크롤러, 허용정책 검증, 링크추출, canonical, 중복 제거, 품질 평가, 검색색인 증분 업데이트, 삭제 전파.
- 콘텐츠별 탭: 웹, 이미지, 동영상, 뉴스, 지도/장소, 쇼핑, 지식패널. 각 수직검색은 **라이선스를 받은 소스와 자체 데이터 파이프라인**으로만 구축한다.
- AI: 질문 해석, 다중문서 근거 추출, 출처 표시, 할루시네이션 평가, 오류/불확실성 처리, 모델 비용 제한, 출처의 noindex/라이선스 존중.
- 광고/수익: 캠페인·광고그룹·소재·키워드·심사·노출·클릭·부정클릭 방지·청구·환불/정산·다중통화. 샌드박스 통과 전 실결제 금지.
- 웹마스터/파트너: 조직 권한, 사이트 소유 인증, sitemap 제출, URL 검사, 색인 상태/유입 보고, 광고 API/매출 콘솔, 초대/권한/과금.
- 관리자: 운영·색인·크롤러 품질, 신고·결과 비노출, 데이터삭제, 권한/MFA, 콘텐츠권리, 광고·정산 승인, 감사로그, 장애대응.
- 개발 운영: 테스트/관측성/보안·개인정보·재해복구·접근성·국제화·CI/CD·인프라·독립 감사.

특히 아래 용어의 오용을 금지한다.
- '독립 검색엔진' = 허용된 외부 웹/수집 데이터에서 자체 검색색인을 구축하고 그 색인에서 검색한다.
- '자체 랭킹' = 재현 가능한 점수/리랭킹 알고리즘과 품질 평가 자료가 있다.
- '100% 구현' = 모든 승인 범위의 요구사항에 대해 실제 기능·증적이 있으며, Google 자체와 완전 동일하다는 뜻은 아니다.
- '상용화 PASS' = 인프라/보안/합법성/정산/개인정보/품질을 독립 검수한 후에만 가능하다.

## 2. 채택 아키텍처 및 언어

### 플랫폼
- 사용자 웹: Vue 3, Nuxt, TypeScript, SSR/SEO, i18n, accessible responsive layout, CSS tokens.
- 관리자 웹: 별도 Nuxt/Vue 앱 및 별도 인증/배포, privileged MFA, least privilege, 모든 파괴적 작업 감사.
- 파트너/웹마스터: 별도 Nuxt/Vue 앱, 조직별 RBAC/ABAC, 강한 테넌트 분리, 운영 기능 포함.
- 모바일 앱: Flutter + Dart Android/iOS, REST/gRPC-Gateway, 상태관리·오프라인 정책·딥링크·앱접근성.
- 데스크톱 앱은 Phase9 이후 선택: Vue + Tauri/OS 키체인, 자동 업데이트와 서명 체계.

### 서버와 검색 데이터
- Gateway/BFF, identity, webmaster, crawl-coordinator, fetcher, parser/extractor, indexing, search, suggest, relevance, verticals, answer, ads, billing, moderation, analytics는 명확한 bounded context로 분리하되 Phase0-3에서는 비용과 복잡도에 맞춘 모듈러 Go 서비스로 시작한다.
- PostgreSQL: 사용자/테넌트/정책/메타데이터/광고 및 감사의 진실 원장. 주 검색 역색인 저장 금지.
- OpenSearch: 검색 색인/BM25, RRF/벡터 필요 시 추가. Index aliases와 안전한 reindex/rollback 필수.
- Redis: 캐시 및 quota/rate-limiter (원장이나 단일 내구성 큐로 사용하지 않음).
- Kafka-compatible broker: 멱등 아웃박스/크롤링 이벤트/색인 이벤트 (스케일 요구 시 채택).
- S3 object storage: 원문/추출 텍스트/허가된 이미지/동영상 메타, 보유 기간과 접근권한 관리.
- ClickHouse: 대량 이벤트 집계가 필요할 때만. 기본 수집량 최소화, 민감한 검색 이력 기본 비수집.
- Python ML: 임베딩, rerank, 스팸, 분류 및 citation-grounded answers. 모델버전·기대 성능·자원 비용 기록.
- Infrastructure: Docker/Compose(dev), Kubernetes(필요 시), Terraform, GitHub Actions, OpenTelemetry, Prometheus/Grafana, centralized logs/traces.

### 검색 질적 설계
- Korean-first: 한글형태소 분석기(Nori 등) 설치·매핑 호환성 실제 테스트, 한영 혼합 쿼리, 오탈자, 자모 및 띄어쓰기, 영문 토큰 분리.
- Document acquisition -> parse -> sanitize -> dedup/canonical -> quality/spam -> index event -> BM25 -> search filters -> relevance rerank -> UI.
- Embeddings/vector, answer, personalization은 A/B 테스트 결과와 라이선스 검증 후 점진적 추가.
- 검색 결과는 원문 출처 URL을 존중하며 광고와 분리된 organic 영역에서 렌더링한다.

## 3. 스키마·API·계약의 최우선 작업

1. **현재 SQL을 실제 PostgreSQL에서 마이그레이션·테스트한다.** SQL 문법/표 참조/FK/복합키/제약/트랜잭션/권한을 확인한다. 별도 DB 마이그레이션 해시·상태 테이블을 추가하고, 중단 후 재실행/백업·복구 전략을 정한다.
2. 스키마 도메인: `identity`(계정, MFA, 조직, 역할), `crawl`(호스트/robots/frontier/fetch), `catalog`(문서/버전/인덱스 이벤트), `query`(요청/품질/랭킹), `portal`(이미지·뉴스·동영상·장소·쇼핑), `ads`(광고·경매·과금), `publisher`(웹마스터), `control`(모더레이션·삭제·감사).
3. 실제 데이터량이 큰 검색 이벤트/광고이벤트/감사는 파티션, TTL, 인덱스와 데이터 주권/보존 기간을 분석해 추가 마이그레이션을 설계한다. 임의의 개인정보 저장은 금지한다.
4. `proto/openportal/**`와 실제 Go 코드/Swagger/OpenAPI 사이 계약과 code generation을 자동화한다. 잘못된 protobuf go_package, 필드번호 변경, 호환성 파괴를 CI가 차단한다.
5. 요청 idempotency, API 오류 코드, cursor pagination, trace IDs, rate-limit, auth scopes, versioning, webhook signing, retry contract를 명확히 한다.
6. 데이터삭제/법적 제거/tombstone은 primary DB, inverted index, cache, object store, image/news mirrors 등 모든 자료 경로에서 검증되어야 한다.
7. Postgres 스키마는 서비스와 테넌트 권한을 실현하기 위한 기반일 뿐이다. RLS/SQL GRANT와 서비스 인증·인가·감사코드를 별도 구현하고 권한탈취 테스트를 통과한다.
8. 컬럼을 만들기만 하고 데이터를 연결하지 않는 작업을 완료 처리하지 않는다. 최종 기능에는 실제 E2E data lineage가 있어야 한다.

## 4. 100개 작업과 Phase 통과 기준

세부 단계: **반드시 `docs/PHASES_0_TO_9.md`에 있는 001–100 전체 항목을 직접 읽고 그 순서대로 실행하라.** 번호/체크박스와 Phase 완료표를 바꾸거나 축약할 수 없다. 10개 Phase는 다음과 같다.

| Phase | 필수 출고물 | 통과 증거 |
|---|---|---|
| 0 — 기반 | PRD, ADR, 86테이블 초기 마이그레이션 검증, Protobuf, Go/Vue 검색 데모 | local E2E, migration, unit CI |
| 1 — 사용자 플랫폼 | Nuxt/Flutter 사용자 검색, auth/MFA, 설정, admin/partner 격리 | cross-platform E2E, RBAC, accessibility |
| 2 — 분산 크롤링 | 허용된 URL discovery, RFC9309, SSRF 방어, 스케줄링 | 작은 허용 사이트 집합의 반복 크롤 테스트 |
| 3 — 인덱싱 | 자체 crawl→parse→dedup→index→query, 삭제 | blue/green reindex, lang query, recovery |
| 4 — 검색 품질 | BM25, Korean tokenizer, ranking/relevance A/B | NDCG, MRR, p95, safe search, spam |
| 5 — 수직검색 | 이미지/뉴스/영상/장소/쇼핑 | 라이선스/출처/삭제 포함 E2E |
| 6 — AI 답변 | citation-grounded RAG, 선택형 AI 답변 | hallucination/prompt injection 평가 |
| 7 — 광고·파트너 | publisher portal, sponsored results, 캠페인·과금 | sandbox ledger, tenant separation, independent approval |
| 8 — 관리자·안전 | admin RBAC/MFA, abuse, 삭제, 감사, 접근통제 | DAST, pentest, legal review |
| 9 — 운영·출시 | 부하, 관측, DR, 비용, 릴리즈, 독립 감사 | 기능·품질·보안 GO/NO-GO 증적 |

**절대 금지:** Phase 1 UI를 만들어 놓고 Phase 2~9를 문서 또는 백엔드 mock만으로 '완료'했다고 보고하는 행위.

## 5. 실제 작업 프로토콜(반드시 준수)

작업 시작 시:
- git 상태와 기존 repo 파일을 확인하고, 위험한 초기화/삭제 명령은 실행하지 않는다.
- `README.md`, `docs/ARCHITECTURE.md`, `docs/SECURITY_AND_COMPLIANCE.md`, `docs/PHASES_0_TO_9.md`, `database/migrations/001_initial.sql`, proto 및 현재 구현을 읽는다.
- `docs/IMPLEMENTATION_STATUS.md`를 생성: 001–100 각 항목 상태(미착수/진행/검증완료/차단), 파일, 테스트 증거, 커밋 SHA를 표로 기록한다.
- `docs/GAP_REGISTER.md`: P0=즉시 출시차단 보안/정산/법 위반, P1=핵심 기능 상실, P2=중요 품질, P3=사소한 개선; 재현 명령/예상 결과/실제 결과/근본원인/수정 패치/회귀테스트.
- `docs/DECISIONS/` ADR 작성. 해당 단계에서 구축 가능한 것과 외부 승인/데이터 권리로 차단된 것을 구분한다.

각 작업 시:
1. 먼저 **실패하는 테스트**(가능한 경우)를 작성해 요구사항을 고정한다.
2. 제품 UI, API, DB, 권한, 이벤트, 작업자, 색인의 최소 end-to-end 코드를 실제로 구현한다.
3. 단위/통합/E2E/보안/부하 테스트를 수준에 맞춰 수행하며 실행한 명령·실출력·환경과 coverage를 기록한다.
4. migration/Protobuf/OpenAPI/문서와 모니터링을 업데이트한다.
5. 변경 내용을 문서화하고 atomic commit에 SHA를 적는다(푸시/배포는 사용자가 허용한 저장소/자격으로만 실행).
6. 코드로 검증 못 한 영역은 절대로 PASS로 간주하지 않는다.
7. Phase별 최종 리포트는 변경 파일, 변경된 스키마, 구현 endpoint, 테스트 증거, 기능 데모, 보안 리스크, 비용, 미완료를 구체적으로 명시한다.

## 6. 정량 지표와 상용화 게이트

표에 있는 값은 **출시 제안 목표(가설)**이며 사전 테스트 없이 달성 수치로 보고하면 안 된다. 부하 시나리오(문서 수/언어/쿼리/장비/QPS)를 함께 제시하라.

| 지표 | 초기 제안 게이트 | 필수 측정 환경 |
|---|---|---|
| 검색 API p95 | 500ms 이하 목표 | 정해진 대표 쿼리/QPS, 정상 인덱스/캐시 조건 |
| 가용성 | 월 99.9% 목표 | 실제 SLI/SLO와 측정 기간 |
| HTTP 5xx | 0.5% 미만 목표 | 가용 부하에서 5xx/timeout 측정 |
| 색인 freshness | 정책상 중요한 페이지 24h 이내 목표 | 허용된 사이트 데이터셋 |
| relevance | NDCG@10 baseline 대비 유의미 개선 | 별도 Korean/English judgment set |
| 개인정보/민감정보 유출 | P0 제로 필수 | 실제 침투 및 데이터 감사 |
| 청구/과금 | reconciliation 불일치 제로 필수 | 결제 sandbox, 중복 webhook/retry |
| 삭제 요청 | 지정 SLA와 전체 downstream purge 검증 | 재색인/캐시/backup 고려 |

P0/P1이 오픈 상태이거나 독립 감사가 없으면 **NO-GO**. P2는 명시적 승인과 완화책이 필요하며, P3는 별도 추적한다. 라이선스/로봇 정책 위반은 기능 수와 무관하게 NO-GO.

## 7. 코딩 지침

- Go: context propagation, bounded timeouts, structured logs(PII redaction), dependency injection, worker shutdown, retries with jitter and idempotency, `go test -race` when supported.
- SQL: parameterized queries, transactions, monetary `bigint minor units` not floats, FK/ref actions, migration testing, unique idempotency keys, only least privilege service roles.
- Vue/Nuxt: SSR safety, XSS-safe escapes, accessible keyboard/screen reader flow, localization including `ko-KR`, audited dependencies.
- Flutter: accessible result cards, HTTP networking tests, release signing, no dev cleartext in production, secure token storage.
- OpenSearch: body field mapping/term filters, aliases, size restrictions, index lifecycle, no unbounded deep pagination, Korean analyzer test.
- Crawlers: must avoid SSRF, host/network policy, robots, noindex, source licenses, anti-bot evasion, aggressive concurrency, passwords/auth-only material; never index known private or removed documents.
- AI: treat retrieved web content as untrusted data (prompt injection resistant), license check, source URL citations, answerable/unanswerable QA evaluation.
- Media/news/places: source rights, attribution, freshness and removal verification. Never use free-form scraping when a feed agreement is required.
- Ads: label `광고`/`Sponsored` clearly; use immutable ledger/events, replay-safe billing, invalid traffic checks, per-tenant isolation.
- CI/CD: no secrets in repo, signed artifacts, staged rollout, rollback, observability, production readiness review.

## 8. 초기 실행지시 — 지금 가장 먼저 할 것

1. 저장소를 읽고 **무엇이 실제 동작하는지** 체크한다. 현재 스타터는 Go API + 예시 OpenSearch 색인 + Vue 화면 + Flutter source example이다. admin/partner/crawler/production auth는 아직 구현되지 않았음을 확인한다.
2. 실행 환경에 Docker가 있으면 `cd infra && docker compose up -d`로 구성 요소를 시작한다. PostgreSQL 초기 마이그레이션을 확인하고 `python3 scripts/seed.py` 후 `backend`의 `go test ./...`, 실제 GET 검색, `apps/web` 빌드/브라우저 E2E를 수행한다.
3. `001_initial.sql`에 대해 실제 PostgreSQL 테스트가 통과하지 못하면 해당 원인을 고치되, 손실성 변경 대신 새 migration/초기 schema 수정 후 명확히 기록한다.
4. Proto buf format/lint/breaking 설정을 점검하고 CI로 묶는다. 기존 protobuf는 서비스 계약 초안이며 실제 서비스 구현 전 반드시 정합성 검사한다.
5. `docs/IMPLEMENTATION_STATUS.md`와 `docs/GAP_REGISTER.md`, Phase 0 보고서를 먼저 제출한다. 그 뒤 미완료 001–100 항목을 **매 단계 실제 코드 구현으로** 전진시킨다.
6. 외부 승인이 필요한 요소(도메인/비밀키/결제/상표/지도·뉴스 계약/배포 계정)는 stub을 COMPLETE 처리하지 말고 BLOCKED로 기록하며 다른 독립 작업은 계속한다.

## 9. 응답/진척 보고 형식

각 작업/커밋마다 다음 형태의 보고를 남겨라:

```
[OpenPortal Search / Codex / Phase N / Task 0XX]
- 목표 / 구체 요구사항:
- 현재 실제 구현 상태 (완료·부분·차단):
- 변경 파일, DB migration, API/proto, 커밋 SHA:
- 실행 명령과 검증 환경:
- 테스트 결과 (PASS / FAIL / NOT RUN 사유):
- 재현 가능 데모와 샘플 입력/출력:
- 보안·법무·성능 검증 상태:
- 발견 결함(P0–P3), 재현방법, 조치:
- 다음 작업과 의존성:
- Phase gate 판정 (GO 권고 / NO-GO / 판단 불가):
```

최종 상용화 리포트는 **독립 감사자가 실제 확인한 증적**과 수치만 인정한다. 시작하라. Phase 0부터 코드 변경, 테스트 및 결함수정까지 수행하라. 에이전트가 할 수 있는 작업을 단순 계획서로 대체하지 말고 실제 구현하라.
