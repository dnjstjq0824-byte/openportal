# OpenPortal Search / Phase 2 / Tasks 021–030 보고서

판정: **NO-GO (선행 게이트 예외 진행)**.
Phase 0/1의 선행 차단 게이트(Docker Compose, CGO race, Flutter SDK 호스트 설치, 원격 CI, 독립 감사자 승인)가 유지되고 있으며, Phase 2의 핵심 안전 게이트인 **"임의 인터넷 도메인 크롤링 비활성(default-deny)"** 정책이 엄격히 준수되어 승인된 10~100개 테스트 코퍼스 이외의 임의 인터넷 수집이 차단되어 있으므로 상용화 GO가 아닌 **NO-GO (선행 게이트 예외 진행)**를 유지한다.

- 기준 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`
- 선행 커밋: `44e6ea26f3066600c3b28b6d02773cbb0343a42d` (Phase 1)
- Phase 2 구현 커밋: `2c6da04628cfd6d9daca3936ebe7fa424b4e2952`
- 원본 import: `be0850836f8125683d8f7fa8377aab66befe5050`
- 실행 일자/환경: 2026-10-26, Windows x64 / Asia-Seoul, Node v24.19.0, Go 1.27.3, Python 3.12.14, OpenSearch 2.19.0, PostgreSQL 17.10 (embedded).

---

## 1. 실제 구현 변경 요약

### 1.1 데이터베이스 & 코퍼스 정책 마이그레이션 (Tasks 021, 030)
- **`database/migrations/004_phase2_crawler_policy_and_corpus.sql`**:
  - `control.feature_flags`에 크롤러 정책 플래그 시딩:
    - `crawler.arbitrary_enabled` = `false` (임의 인터넷 크롤링 기본 거부).
    - `crawler.bounded_corpus_enabled` = `true` (허가된 테스트 코퍼스 한정 수집 허용).
  - `crawl.hosts`에 10개 검증 도메인(`example.org`, `example.com`, `example.net`, `example.edu`, `rfc-editor.org`, `w3.org`, `iana.org`, `schema.org`, `openportal.local`, `internal.test`) 시딩.
  - `crawl.url_submissions`에 RLS 활성화 (`tenant_isolation_url_submissions`).
  - Frontier 큐 우선순위 및 스케줄링 가속 인덱스 생성 (`frontier_queue_next_fetch_idx`).

### 1.2 URL 정규화 & 표준화 (Task 022)
- **`backend/internal/crawler/canonical.go` & `canonical_test.go`**:
  - RFC 3986 URL Canonicalization (`http`/`https` scheme 강제).
  - 다국어 도메인 IDN Punycode 변환 (`golang.org/x/net/idna`, 예: `검색엔진.한국` -> `xn--c79as89axudhxe.xn--3e0b707e`).
  - 추적 파라미터 자동 정제 (`utm_*`, `fbclid`, `gclid`, `ref`, `source`).
  - 기본 포트(80, 443) 제거, 점(`.`, `..`) 세그먼트 정규화.
  - 결정적 SHA-256 URL 해시 계산 (`HashURL`).
  - 13개 세부 케이스 유닛테스트 전체 통과 (PASS).

### 1.3 RFC 9309 robots.txt 파서 및 캐시 (Task 024)
- **`backend/internal/crawler/robots.go` & `robots_test.go`**:
  - RFC 9309 호환 robots.txt 파서 구현.
  - 와일드카드(`*`), 끝점 앵커(`$`) 정규식 컴파일.
  - Longest-match 규칙 및 동일 길이 시 Allow 우선순위 tie-breaking.
  - `Crawl-delay` 파싱 및 초 단위 정규화.
  - `Sitemap:` 디렉티브 자동 추출.
  - HTTP 상태 코드별 정책: 4xx allow, 5xx fail-closed (거부).
  - 만료 시간(TTL) 기반 메모리 캐시 및 갱신.

### 1.4 SSRF 차단 및 IP 고정 (Task 025)
- **`backend/internal/crawler/ssrf.go` & `ssrf_test.go`**:
  - 루프백 (`127.0.0.0/8`, `::1`), 사설망 (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), 링크로컬 (`169.254.0.0/16`), CGNAT (`100.64.0.0/10`), 멀티캐스트 전면 차단.
  - 클라우드 메타데이터 엔드포인트 (`169.254.169.254`, `metadata.google.internal`) 차단.
  - **DNS Rebinding 방어 IP Pinning Dialer**: DNS 조회 결과를 사전 검증하고 확인된 IP로 소켓을 직결하여 TOCTOU(Time-of-Check Time-of-Use) 방지.
  - **SafeRedirectPolicy**: 최대 5회 리다이렉트 제한, HTTPS -> HTTP 다운그레이드 전면 차단, 매 홉마다 SSRF 주소 재검증.

### 1.5 Politeness & 속도 제한 (Task 026)
- **`backend/internal/crawler/politeness.go` & `politeness_test.go`**:
  - 호스트별 크롤 지연 시간(`HostDelay`) 강제 준수.
  - 호스트별 동시 크롤링 연결 1개로 제한 (`HostConcurrencyLimiter`).
  - 429 Too Many Requests / 503 Service Unavailable 수신 시 지수 백오프(`ExponentialBackoff`) 적용.
  - 조건부 GET 요청 헤더 생성 (`If-None-Match`, `If-Modified-Since`).

### 1.6 Fetcher & 안전한 파서 (Task 027)
- **`backend/internal/crawler/fetcher.go` & `fetcher_test.go`**:
  - Content-Type 화이트리스트 검사 (`text/html`, `text/plain`, `application/xml`, `text/xml`만 허용, 바이너리/실행파일 즉시 거부).
  - `io.LimitReader`를 통한 2 MiB 최대 응답 본문 크기 상한 강제.
  - 압축폭탄(Decompression Bomb) 방어: 최대 5 MiB 압축 해제 제한 및 압축비율 초과 시 중단.
  - 10초 타임아웃 적용.

### 1.7 ISO 28500 WARC/1.0 아카이빙 (Task 028)
- **`backend/internal/crawler/warc.go` & `warc_test.go`**:
  - ISO 28500 WARC/1.0 `response` 레코드 포맷 직렬화.
  - SHA-256 블록 다이제스트 및 페이로드 다이제스트 헤더 기록.
  - 파티셔닝된 객체 스토리지 키 생성 (`raw/YYYY/MM/DD/<sha256>.warc`).

### 1.8 Extractor & 로봇 지시자 (Task 029)
- **`backend/internal/crawler/extractor.go` & `extractor_test.go`**:
  - `golang.org/x/net/html` 기반 안전한 HTML 파서.
  - `X-Robots-Tag` 및 `<meta name="robots">` 파싱 (`noindex`, `nofollow`, `nosnippet`, `noarchive`).
  - `<link rel="canonical">` 표준 URL 추출.
  - `nofollow` 배제 및 하이퍼링크/앵커 텍스트 추출.
  - XML sitemap 및 sitemapindex 파일 파싱.

### 1.9 Frontier 큐 & Bounded Engine (Tasks 021, 023, 030)
- **`backend/internal/crawler/frontier.go` & `engine.go`**:
  - 호스트별 파티셔닝 우선순위(0~100) 큐.
  - 원자적 임대(`LeaseAcquire`) 및 확인(`Ack`), 실패 시 지수 백오프 재시도(`Nack`).
  - 승인된 호스트 레지스트리 기반 **허가된 코퍼스(10~100개 사이트) 한정 수집 게이트**.
  - 비인가 임의 도메인 수집 요청 시 `ErrArbitraryCrawlDenied` 즉시 반환.
  - 수집 트래픽, robots 차단 수, HTTP 상태 코드별 `ComplianceReport` 집계.

### 1.10 웹 프론트엔드 suggest 가드 수정
- **`apps/web/src/App.vue`**:
  - 검색 실행 및 버티컬 탭 전환 시 이전 비동기 제안 타이머 취소 및 드롭다운 닫기.
  - 입력창 blur 이벤트 시 제안 목록 자동 닫기 처리.
  - Playwright E2E 5개 테스트 전체 통과 완료.

---

## 2. 실행 명령 및 실제 증적

모든 테스트는 로컬 환경에서 실제 프로세스를 띄워 검증되었다.

| 명령 | 결과 | 증적 파일 / 내용 |
|---|---|---|
| `node scripts/test-database.mjs` | **PASS** | `docs/evidence/database.txt`: 86테이블/75FK 검증, 마이그레이션 001~004 정방향 순차 적용, 10개 코퍼스 호스트 시딩, RLS 및 비인가 거부 통과 |
| `node scripts/test-proto.mjs` | **PASS** | Buf lint, compile, breaking negative control PASS |
| `node scripts/check-contracts.mjs` | **PASS** | OpenAPI 3종 및 이벤트 계약 검증 통과 |
| `python -m unittest discover -s scripts -p test_seed.py` | **PASS** | 3건의 seed 유닛 테스트 통과 |
| `go test -v ./...` (in backend) | **PASS** | `docs/evidence/go-tests.txt`: crawler (18개 테스트), identity, search, cmd/api 전체 패키지 통과 |
| `go test -coverpkg=./... -coverprofile=...` | **PASS** | `docs/evidence/go-coverage.out`: crawler 80.6%, identity 85.0%, search 75.7% 문장 커버리지 달성 |
| `npx playwright test` | **PASS** (5/5) | `docs/evidence/browser-tests.txt`: OpenSearch 2.19.0 + Go + Vue3 Chrome 브라우저 E2E 5개 시나리오 전체 통과 |
| `govulncheck ./...` | **PASS** | `docs/evidence/govulncheck.txt`: No vulnerabilities found (취약점 0건) |
| `gitleaks dir . --redact` | **PASS** | `docs/evidence/gitleaks.txt`: 1.39MB 스캔 완료, 누출 시크릿 0건 |

---

## 3. 미해결 항목 및 게이트 차단 사유 (NO-GO)

1. **임의 인터넷 도메인 크롤링 비활성 유지 (G006, G010)**: 법적 승인 및 인프라 검토 전까지 승인된 10~100개 테스트 코퍼스 외 임의 크롤링은 비활성화 상태(`crawler.arbitrary_enabled` = `false`)를 엄격히 유지함.
2. **Docker 환경 미설치 (G005)**: 호스트에 Docker가 없어 `infra/compose.yaml` 풀스택 컨테이너 검증 미실행.
3. **CGO 컴파일러 부재 (G012)**: `go test -race ./...` 미실행.
4. **Flutter SDK 미설치 (G009, G017)**: `apps/mobile/` 코드는 작성 완료되었으나 호스트 SDK 부재로 로컬 실행 불가.
5. **원격 CI / CodeQL 미실행 (G011, 020)**: 원격 저장소 및 토큰 부재.
6. **독립 승인자 서명 부재 (G010, 010)**: 보안·법무·정산 독립 제3자 감사가 수행되지 않음.
