# OpenPortal Search / Phase 1 / Tasks 011–020 보고서

판정: **NO-GO (선행 게이트 예외 진행)**.
Phase 0의 외부 승인 및 CI 부재로 인한 NO-GO 상태 하에서, 독립적으로 진행 가능한 Phase 1(Tasks 011~018)의 핵심 포털 화면·Gateway·Passkey/MFA 인증·BOLA RLS·검색설정·콘솔 접근 경계를 구현하고 로컬 런타임에서 검증을 완료했다.
그러나 호스트 환경의 Flutter SDK 미설치(019) 및 원격 통합 CI/Lighthouse 미연동(020), 그리고 Phase 0의 미해결 게이트(Docker Compose, CGO race, 독립 감사자 승인)가 남아있으므로 상용화 승인이나 Phase 1 완전 종결(GO)을 선언하지 않는다.

- 기준 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`
- 선행 커밋: `ba2c1a5badf11b5fe3a7fbe390227a3d4067db01`
- Phase 1 구현 커밋: `44e6ea26f3066600c3b28b6d02773cbb0343a42d`
- 원본 import: `be0850836f8125683d8f7fa8377aab66befe5050`
- 실행 일자/환경: 2026-10-24, Windows x64 / Asia-Seoul, Node v24.11.1, Go 1.27.3, Python 3.12.14, OpenSearch 2.19.0, PostgreSQL 17.10 (embedded).

---

## 1. 실제 구현 변경 요약

### 1.1 데이터베이스 & 보안 격리 (Tasks 013, 015, 016, 017)
- **`database/migrations/003_phase1_identity_and_portal_flags.sql`**:
  - `identity.preferences`에 `recent_searches_opt_in`, `search_correction_opt_in`, `safe_search`, `theme`, `language` 컬럼 및 제약조건 추가.
  - `identity.auth_challenges`, `identity.recovery_codes`, `identity.recent_searches` 테이블 신설.
  - **PostgreSQL Row Level Security (RLS)** 강제 활성화:
    - `identity.preferences`, `identity.sessions`, `identity.recovery_codes`, `identity.recent_searches`, `identity.organization_members`에 `current_setting('app.current_account_id', true)` 기반 격리 정책 적용.
  - `control.feature_flags`에 검색 vertical 플래그 시딩: `vertical.web` (활성), `vertical.image`, `vertical.news`, `vertical.video`, `vertical.map`, `vertical.shopping` (비활성, Phase 5 게이트 설명 명시).
  - `query.rewrites`에 오타 및 한영전환 교정 매핑 시딩.
- **`scripts/test-database.mjs`**:
  - 마이그레이션 `001..003` 정방향 순차 적용 및 멱등성 검증.
  - `control.feature_flags` 비활성 버티컬 설정 검증.
  - 비인가 계정 접근 거부 및 BOLA RLS 교차 계정 접근 차단 검증.
  - DDL 롤백 및 체크섬 변조 방지 검증 (PASS).

### 1.2 Go 백엔드 & Gateway (Tasks 012, 013, 014, 015, 016, 018)
- **`backend/internal/search/normalize.go` & `normalize_test.go`**:
  - 쿼리 NFC 정규화, 연속 공백 축소, 1~200자 길이 제한 (`NormalizeQuery`).
  - 한영 오타 교정 제안 로직 (`SuggestCorrection`).
  - 기본 vertical 탭 구성 테이블 (`DefaultVerticalTabs`).
- **`backend/internal/search/proto_bridge.go` & `proto_bridge_test.go`**:
  - REST 검색 요청/응답과 Protobuf v1 (`SearchRequest`, `SearchResponse`, `SuggestResponse`) 간 무손실 양방향 변환.
  - Organic 검색 결과에 대해 `Sponsored=false` 및 `IsOrganic=true` 강제 격리 검증.
- **`backend/internal/identity/identity.go` & `identity_test.go`**:
  - Passkey WebAuthn / OIDC 챌린지 발급 및 검증.
  - SHA-256 해시 기반 세션 토큰(`ops_...`) 발급 및 검증.
  - 단회성 계정 복구코드(`OP-XXXX-XXXX`) 발급 및 복구 시 기존 세션 일괄 폐기.
  - RFC 6238 기반 TOTP MFA 시크릿 발급, TOTP 토큰 계산 및 활성화.
  - 사용자 환경설정(SafeSearch, 테마, 언어, opt-in) 및 opt-in 기반 최근 검색어 기록/삭제.
  - 세션, 환경설정, 최근검색, 조직 사이트 전반에 걸친 BOLA 격리 방어.
- **`backend/cmd/api/main.go` & `http_test.go`**:
  - Gateway 미들웨어: `X-Request-Id` 및 `X-Correlation-Id` 전파, 추적성 보장.
  - 콘솔 오리진 경계 검증: `admin.*` 및 `partner.*` 호스트로부터의 요청은 즉시 `403 CONSOLE_BOUNDARY_DENIED`로 기본 거부.
  - 버티컬 파라미터 검증: 비활성 버티컬 요청 시 `403 VERTICAL_DISABLED` 에러 반환.
  - `/api/v1/portal/config`, `/api/v1/auth/*`, `/api/v1/accounts/{id}/*`, `/api/v1/organizations/{id}/*` 엔드포인트 라우팅 및 BOLA 검증.

### 1.3 웹 포털 프론트엔드 (Tasks 011, 012, 013, 017, 018)
- **`apps/web/src/App.vue` & `style.css`**:
  - 접근성(a11y): Skip link (`#main-content`), ARIA 랜드마크(`role="search"`, `role="listbox"`, `role="option"`, `aria-selected`), WCAG 2.1 AA 대비.
  - 오타 교정 UI (`data-testid="query-correction"`): 오타 감지 시 원클릭 교정 링크 제공.
  - 버티컬 탭: feature flag 기반 연동. 비활성 탭 클릭 시 Phase 5 게이트 안내 모달 표시.
  - 최근 검색어: opt-in 토글, 칩 버튼 클릭 검색, 개별/전체 삭제 지원.
  - 검색 설정 모달: safeSearch (strict/moderate/off), 다크/라이트 테마, 한국어/영어 i18n, 최근검색 opt-in 토글.
  - 계정 및 보안 모달: 패스키 인증 시작, 세션 토큰 표시, 단회성 복구코드 발급, TOTP MFA 시크릿 발급, 세션 안전 폐기.
  - 접근 경계 전용 화면: `/admin`, `/partner` 접근 시 전용 접근 거부 안내 화면 (`console-denied-view`).
  - 404 전용 화면: 잘못된 경로 접근 시 전용 오류 화면 (`not-found-view`).

### 1.4 모바일 클라이언트 (Task 019)
- **`apps/mobile/lib/main.dart` & `test/widget_test.dart`**:
  - Flutter 3.29 호환 검색 화면, `PopScope`를 활용한 뒤로가기 가로채기(검색 결과 중일 때 결과 닫기), 텍스트 클립보드 복사/공유, `Semantics` 접근성 트리, 오타 교정 탭 및 검색 히스토리 구현.
  - 가상 검색 클라이언트를 통한 위젯 테스트 작성 완료.

### 1.5 API 계약 및 명세 (Task 014)
- **`docs/openapi/search.v1.yaml`**, **`identity.v1.yaml`**, **`portal.v1.yaml`**: 도메인별 분리된 OpenAPI 3.0 명세 작성.
- **`scripts/check-contracts.mjs`**: 도메인별 OpenAPI 스펙 유효성 및 이벤트 브레이킹 감지 자동화.

---

## 2. 실행 명령 및 실제 증적

모든 테스트는 로컬 환경에서 실제 프로세스를 띄워 검증되었다.

| 명령 | 결과 | 증적 파일 / 내용 |
|---|---|---|
| `node scripts/test-database.mjs` | **PASS** | `docs/evidence/database.txt`: 86테이블/75FK 검증, 마이그레이션 001~003 정방향 적용, RLS BOLA 격리 및 비인가 거부 통과 |
| `node scripts/test-proto.mjs` | **PASS** | Buf lint, compile, breaking negative control PASS |
| `node scripts/check-contracts.mjs` | **PASS** | `docs/evidence/contracts.txt`: OpenAPI 3종 및 이벤트 계약 검증 통과 |
| `python -m unittest discover -s scripts -p test_seed.py` | **PASS** | 3건의 seed 유닛 테스트 통과 |
| `go test -v -count=1 ./...` (in backend) | **PASS** | `docs/evidence/go-tests.txt`: HTTPBoundary, Auth/MFA/Recovery/BOLA, ProtoBridge, Search, Config 전체 통과 |
| `go test -count=1 -coverprofile=...` | **PASS** | `docs/evidence/go-coverage.out`: cmd/api 66.6%, identity 85.0%, search 75.7%, config 100.0% |
| `npx playwright test` | **PASS** (5/5) | `docs/evidence/browser-tests.txt`, `docs/evidence/search-browser.png`: 한국어/영문 검색, 빈 결과, 503 오류 인젝션, indexable=false 제외, Phase 1 포털 전체 통합 E2E 5건 모두 통과 |
| `govulncheck ./...` | **PASS** | 취약점 0건 |
| `npm audit` / `npm audit --prefix apps/web` | **PASS** | 취약점 0건 (moderate 이상 zero 취약점 준수) |
| `gitleaks dir . --redact` | **PASS** | 누출 시크릿 0건 |

---

## 3. 미해결 항목 및 게이트 차단 사유 (NO-GO)

1. **Docker 환경 미설치 (G005)**: `infra/compose.yaml` 기반 풀스택 컨테이너화 검증 불가.
2. **CGO 컴파일러 부재 (G012)**: `go test -race ./...` 미실행.
3. **Flutter SDK 미설치 (G009, G017)**: `apps/mobile/` 코드는 작성 완료되었으나 `flutter test` 로컬 실행 불가.
4. **원격 CI / CodeQL 미실행 (G011, 020)**: 원격 저장소 및 토큰 부재.
5. **독립 승인자 서명 부재 (G010, 010)**: 보안·법무·정산 독립 제3자 감사가 수행되지 않음.
6. **외부 접근 차단 유지**: 임의 인터넷 사이트 크롤링, 상용 결제 연동, 공개 클라우드 배포는 엄격히 비활성화 상태를 유지함.

---

## 4. 결론 및 다음 단계

Phase 1의 핵심 기능(웹 포털, Gateway, 인증/MFA, BOLA RLS, 접근 경계)은 검증되었으나 게이트 제약에 따라 Phase 1 판정은 **NO-GO**를 유지한다.
다음 세션에서는 Phase 0/1의 선행 게이트 조건을 기록으로 남긴 상태에서, 독립 크롤링 서브시스템(Phase 2 Tasks 021~030)의 URL frontier, RFC9309 robots.txt 파서, SSRF/DNS rebinding 방어 격리 로직의 개발을 순차적으로 이어서 수행해야 한다.
