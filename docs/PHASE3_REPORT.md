# OpenPortal Search / Phase 3 / Tasks 031–040 보고서

판정: **NO-GO (선행 게이트 예외 진행)**.
Phase 0/1/2의 선행 차단 게이트(Docker Compose, CGO race, Flutter SDK 호스트 설치, 원격 CI, 독립 감사자 승인, 임의 인터넷 도메인 크롤링 비활성)가 유지되고 있으며, Phase 3의 자체 크롤링->추출->중복제거->색인->검색 및 Tombstone 삭제 전파 파이프라인의 핵심 기술 검증을 완료하였으나 독립 감사 서명 전까지 상용화 GO가 아닌 **NO-GO (선행 게이트 예외 진행)**를 유지한다.

- 기준 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`
- 선행 커밋: `3fb3af11f61c3120ff8ad8039ef53cba2c9c947c` (Phase 2)
- Phase 3 구현 커밋: `678edd0c734dc23d306c007898862eed6f0fdf14`
- 원본 import: `be0850836f8125683d8f7fa8377aab66befe5050`
- 실행 일자/환경: 2026-10-26, Windows x64 / Asia-Seoul, Node v24.19.0, Go 1.27.3, Python 3.12.14, OpenSearch 2.19.0 (with `analysis-nori`), PostgreSQL 17.10 (embedded).

---

## 1. 실제 구현 변경 요약

### 1.1 데이터베이스 마이그레이션 005 (Tasks 034, 038)
- **`database/migrations/005_phase3_indexing_and_tombstones.sql`**:
  - `catalog.documents` 확장: `simhash` (text), `tombstone_reason` (text), `is_representative` (boolean).
  - 색인 성능 가속 인덱스: `documents_simhash_idx`, `documents_removed_at_idx`, `index_events_state_created_idx`.
  - `control.feature_flags` 시딩: `indexer.nori_enabled` (true), `indexer.blue_green_enabled` (true), `indexer.tombstone_purge_enabled` (true).
  - 일회용 PostgreSQL 17.10 인스턴스에서 마이그레이션 `001..005` 정방향 순차 적용, 스키마 검증, DDL 롤백 및 변조 방지 통과 (PASS).

### 1.2 HTML 보일러플레이트 제거, 본문 추출 및 언어 감지 (Task 031)
- **`backend/internal/indexer/boilerplate.go` & `extract.go` & `boilerplate_test.go`**:
  - `<script>`, `<style>`, `<nav>`, `<header>`, `<footer>`, `<aside>`, 광고 블록(`ad-*`, `banner-*`), 쿠키 배너 등 비본문 노드 제거.
  - 텍스트 밀도 기반 메인 본문, 제목(`Title`), 메타 설명(`Description`), 계층별 헤딩(`Headings`), 하이퍼링크 및 앵커 텍스트(`AnchorTexts`) 추출.
- **`backend/internal/indexer/lang.go` & `lang_test.go`**:
  - 한글 유니코드 음절(`0xAC00..0xD7AF`), 자모, CJK, 라틴 문자 비중을 분석하는 고속 언어 감지(`DetectLanguage`).

### 1.3 64-bit SimHash, 유사중복 탐지 및 대표 문서 선택 (Task 032)
- **`backend/internal/indexer/simhash.go` & `simhash_test.go`**:
  - 64-bit SimHash 알고리즘: 단어 토큰 및 CJK 문자 3-gram shingling, FNV-1a 해싱, 가중치 누적합 기반 핑거프린트 생성.
  - Hamming distance 계산: 64비트 중 4비트 이하 차이를 유사 중복(Near-duplicate, >90% 유사도)으로 판정.
- **`backend/internal/indexer/dedup.go` & `dedup_test.go`**:
  - 중복 클러스터 대표 문서 선택(`SelectRepresentative`):
    1. 자체 URL과 일치하는 `rel="canonical"` 우선
    2. HTTPS 우선
    3. 도메인 우선순위
    4. 쿼리/경로 복잡도가 낮은 짧은 URL
    5. 크롤링 최신성
  - 비대표 문서는 `catalog.canonical_aliases`로 분류.

### 1.4 OpenSearch Nori 한국어 형태소 분석기 탑재 및 v2 매핑 (Task 033)
- **OpenSearch `analysis-nori` 공식 플러그인 설치**:
  - OpenSearch 2.19.0 런타임에 공식 `analysis-nori` 플러그인 탑재 완료.
- **`infra/opensearch/documents_v2.json`**:
  - `korean_analyzer`: `nori_tokenizer` (decompound_mode: `mixed`), `nori_part_of_speech` (stoptags 17개 품사 필터링), `nori_readingform`, `lowercase`.
  - `english_analyzer`: `standard`, `english_stop`, `english_stemmer`.
  - 복합어 분해 및 형태소 추출 검증: `오픈포털은 대한민국을 위한 오픈소스 독립 검색엔진입니다.` -> `오픈`, `포털`, `대한민국`, `대한`, `민국`, `위한`, `위하`, `소스`, `독립`, `검색`, `엔진` 14개 형태소 토큰 분해 확인.

### 1.5 문서 변경감지, Transactional Outbox 및 소비자 멱등성 (Task 034)
- **`backend/internal/indexer/change.go`**:
  - 직전 버전의 본문 SHA-256 및 SimHash 비교를 통해 내용 변경 여부 감지(`DetectChange`). 불필요한 재색인 스킵.
- **`backend/internal/indexer/outbox.go` & `outbox_test.go`**:
  - `catalog.index_events` 기반 Transactional Outbox 큐: 원자적 임대(`LeaseBatch`), 지수 백오프 Nack, 소비자 멱등성 가드(최신 버전이 이미 처리된 경우 구버전 이벤트는 no-op 처리).

### 1.6 Blue-Green 무중단 재색인, Alias 스왑 및 롤백 (Task 035)
- **`backend/internal/indexer/reindex.go` & `reindex_test.go`**:
  - OpenSearch `_reindex` API를 통한 `documents_v1` -> `documents_v2` 데이터 마이그레이션.
  - `_aliases` API를 통한 원자적 Blue-Green 스왑(`documents_search_test` -> `documents_v2`).
  - 장애 시 이전 인덱스로 즉각 롤백(`RollbackAlias`) 라이프사이클 통과 (0.47s PASS).

### 1.7 BM25 다중 필드 랭킹 가중치 및 평가 데이터셋 (Task 036)
- **`backend/internal/search/search.go`**:
  - 다중 필드 BM25 가중치 튜닝: `title^3.5`, `headings^2.0`, `anchor_texts^1.8`, `description^1.5`, `body^1.0`.
- **`backend/internal/search/testdata/ranking_golden_dataset.json` & `bm25_test.go`**:
  - 한국어/영어 대표 쿼리 골든 데이터셋 및 기대 relevance 순위 평가 테스트 통과.

### 1.8 고급 검색 쿼리 파서 및 커서 페이지네이션 (Task 037)
- **`backend/internal/search/parser.go` & `parser_test.go`**:
  - 구문 검색(`"exact phrase"`), 제외 검색(`-exclude`), 사이트 필터(`site:example.org`), 언어 필터(`lang:ko`) 파싱 및 OpenSearch bool DSL 생성.
  - URL-safe Base64 `page_token` 인코딩/디코딩 커서 페이지네이션.
  - `timed_out: true` 및 `_shards.failed > 0` 발생 시 Fail-Closed 503 처리.

### 1.9 Deletion Tombstone 처리 및 전파 (Task 038)
- **`backend/internal/indexer/tombstone.go` & `tombstone_test.go`**:
  - 삭제 사유(`TombstoneReason`: legal takedown, user deletion, privacy right) 및 일시 기록.
  - OpenSearch 인덱스에 `is_tombstone=true`, `indexable=false` 즉시 반영.
  - Outbox `delete` 이벤트 발행 및 검색 쿼리(`must_not: {is_tombstone: true}`)에서 즉각 제외.

### 1.10 인덱서 텔레메트리 메트릭 수집기 (Task 039)
- **`backend/internal/indexer/metrics.go` & `metrics_test.go`**:
  - 크롤 지연(p50, p95), 성공률, 색인 지연(index lag), 문서 신선도(freshness), Outbox 대기 수, Tombstone 적용 수 실시간 집계.

### 1.11 End-to-End 인덱싱 및 Tombstone 파이프라인 통합 (Task 040)
- **`backend/internal/indexer/pipeline.go` & `pipeline_test.go`**:
  - 크롤러 수집 -> Boilerplate 추출 -> SimHash 중복 필터링 -> Outbox 이벤트 발행 -> OpenSearch 색인 -> 검색 쿼리 검증 -> Tombstone 삭제 전파 전체 파이프라인 통과.

---

## 2. 실행 명령 및 실제 증적

| 명령 | 결과 | 증적 파일 / 내용 |
|---|---|---|
| `node scripts/test-database.mjs` | **PASS** | `docs/evidence/database.txt`: 86테이블/75FK, 마이그레이션 001~005 정방향 순차 적용, 10개 코퍼스 호스트, RLS 및 비인가 거부 통과 |
| `node scripts/test-proto.mjs` | **PASS** | Buf lint, compile, breaking negative control PASS |
| `node scripts/check-contracts.mjs` | **PASS** | OpenAPI 3종 및 이벤트 계약 검증 통과 |
| `python -m unittest discover -s scripts -p test_seed.py` | **PASS** | 3건의 seed 유닛 테스트 통과 |
| `go test -v ./...` (in backend) | **PASS** | `docs/evidence/go-tests.txt`: indexer (8건), search (9건), crawler (18건), identity, cmd/api 전체 패키지 통과 |
| `go test -coverpkg=./... -coverprofile=...` | **PASS** | `docs/evidence/go-coverage.out`: cmd/api 19.9%, crawler 24.3%, indexer 15.0%, search 8.9% (전체 모듈 대비) |
| `npx playwright test` | **PASS** (5/5) | `docs/evidence/browser-tests.txt`: OpenSearch 2.19.0 + Go + Vue3 브라우저 E2E 5개 시나리오 전체 통과 |
| `govulncheck ./...` | **PASS** | `docs/evidence/govulncheck.txt`: No vulnerabilities found (취약점 0건) |
| `gitleaks dir . --redact` | **PASS** | `docs/evidence/gitleaks.txt`: 1.81MB 스캔 완료, 누출 시크릿 0건 |

---

## 3. 미해결 항목 및 게이트 차단 사유 (NO-GO)

1. **임의 인터넷 도메인 크롤링 비활성 유지 (G006, G010)**: 법적 승인 및 인프라 검토 전까지 허가된 10~100개 테스트 코퍼스 외 임의 크롤링은 비활성화 상태(`crawler.arbitrary_enabled` = `false`)를 엄격히 유지함.
2. **Docker 환경 미설치 (G005)**: 호스트에 Docker가 없어 `infra/compose.yaml` 풀스택 컨테이너 검증 미실행.
3. **CGO 컴파일러 부재 (G012)**: `go test -race ./...` 미실행.
4. **Flutter SDK 미설치 (G009, G017)**: `apps/mobile/` 코드는 작성 완료되었으나 호스트 SDK 부재로 로컬 실행 불가.
5. **원격 CI / CodeQL 미실행 (G011, 020)**: 원격 저장소 및 토큰 부재.
6. **독립 승인자 서명 부재 (G010, 010, 040)**: 보안·법무·정산 독립 제3자 감사가 수행되지 않음.
