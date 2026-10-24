# Phase 0 위협 모델·리스크 레지스터·독립 승인 패키지 — 010

## 1. 시스템 신뢰 경계 (Trust Boundaries)

1. **공개 클라이언트 ↔ API 게이트웨이 경계 (`Browser / Mobile App` → `Go Gateway / API`)**
   - 모든 쿼리 파라미터(`q`, `page`, `lang`, `tab`, 세션 쿠키/토큰, 헤더)는 비신뢰(untrusted) 입력으로 취급한다.
   - 바운드된 입력 길이(UTF-8 유효성 및 최대 200자), 페이지 상한(1~100), 허용된 열거형 값만 통과시키며, 잘못된 요청은 업스트림(OpenSearch/PostgreSQL)에 도달하기 전에 `400`/`405`로 즉시 차단한다.
2. **API 게이트웨이 ↔ 내부 데이터 플레인 경계 (`Go API` → `OpenSearch 2.19`)**
   - 로컬 개발 환경에서는 `127.0.0.1` 루프백 바인딩만 허용한다.
   - OpenSearch 응답은 2MiB 크기 제한(`io.LimitReader`), HTTP 200 확인, `timed_out == false`, `_shards.failed == 0`, `hits != nil` 검사를 통과해야 하며, 부분 실패나 손상된 JSON은 빈 성공으로 둔갑시키지 않고 `503`(Fail-Closed)으로 처리한다(G001).
3. **API / 마이그레이션 러너 ↔ 컨트롤 플레인 경계 (`Go API / migrate.mjs` → `PostgreSQL 17.10`)**
   - 마이그레이션은 세션 advisory lock(`709230001`), 파일 순서 프리픽스 및 SHA-256 체크섬 대조, 단일 트랜잭션(DDL + `public.schema_migrations`)으로 실행된다(G004).
   - `002_phase0_access_boundary.sql`로 8개 도메인 스키마의 `PUBLIC` 기본 권한을 전부 회수(REVOKE)했다.
4. **관리자·파트너·외부 크롤링 경계 (`Admin / Partner / Crawler`)**
   - 관리자·파트너 콘솔은 전용 도메인/포털 경계와 서버 측 인증·MFA·권한 검증이 완비되기 전까지 기본 거부(Deny-by-default)한다(G006).
   - 외부 인터넷 크롤러는 Phase 2(`021`~`030`)의 RFC 9309 `robots.txt`, SSRF/DNS rebinding/비공개 IP 핀닝, 응답 크기 제한 검증 전까지 비활성화한다(G007).

## 2. STRIDE / OWASP API Top 10 위협 분석 및 통제 현황

| 위협 분류 | 구체적 공격 시나리오 | 현재 구현된 기술 통제 및 검증 증적 | 잔여 리스크 및 후속 Phase 필수 게이트 |
|---|---|---|---|
| **쿼리 주입·자원 고갈 (API4 / DoS)** | 초대형 쿼리 문자열, 깊은 페이지네이션(`page=999999`), 느린 연결로 서버/색인 고갈 | UTF-8 200자/페이지 1~100 검증, HTTP 서버 타임아웃(`ReadHeaderTimeout 5s`, `Read/Write 10s`, `MaxHeaderBytes 1MiB`), 업스트림 4s 타임아웃 (`http_test.go` PASS) | 분산 Rate Limit, 테넌트별 쿼터(`059`, `079`, `088`), 실제 부하 테스트(`048`, `091`) |
| **업스트림 부분 장애 은폐 (Integrity)** | OpenSearch 샤드 일부 실패·타임아웃 시 빈 검색 결과(`total: 0`)를 정상처럼 반환 | `search.go`에서 `TimedOut`, `Shards.Failed > 0`, `Hits == nil`, trailing JSON 감지 시 즉시 에러(`503`) 반환 (`regression_test.go` PASS) | 멀티노드 샤드/레플리카 장애 주입 및 Blue-Green 재색인(`035`, `092`, `093`) |
| **저장형 XSS 및 위험 스킴 (`javascript:`/`data:`)** | 색인된 악성 URL이나 스크립트가 결과 링크로 렌더링되어 브라우저에서 실행 | Go API에서 `http://` 및 `https://` 외 URL 필터링, Vue/Nuxt 텍스트 이스케이프 및 `rel="noopener noreferrer"` 강제 | 크롤러 HTML 파서 살균(`027`, `031`) 및 CSP 헤더 고도화(`088`) |
| **권한 상승·BOLA·테넌트 침범 (API1 / API5)** | 일반 사용자가 `/admin`, `/partner` 또는 타 조직/계정 리소스에 접근하거나 조작 | 관리자/파트너 미인가 접근 차단 및 DB `PUBLIC` 권한 전체 회수(`test-database.mjs` `42501` 검증 PASS) | 세션·MFA·조직/객체 단위 BOLA 회귀 테스트(`015`, `016`, `018`, `081`, `086`) |
| **스키마·계약 무단 변조 (Tampering)** | 적용된 SQL 마이그레이션 파일 수정, Protobuf/OpenAPI 파괴적 변경 배포 | `migrate.mjs` SHA-256 변조 거부(`checksum mismatch` PASS), `test-proto.mjs`(`buf breaking` PASS), `check-contracts.mjs` 불변성 검증 | 운영 백업/복구 리허설(`094`) 및 스키마 하위호환 카나리 검증(`087`, `095`) |
| **개인정보·민감 로그 유출 (Privacy)** | 검색어 원문 무단 수집, 에러 로그에 DB/OpenSearch 자격증명 및 내부 URL 노출 | 검색 요청 원문 DB 비저장(최근검색은 명시적 opt-in 전 비활성), 에러 로그 일반화, 시크릿 스캔(`gitleaks` 0건) | 정보주체 권리 실행(`084`), 보존주기·전경로 파기 자동화(`038`, `085`) |
| **SSRF·무단 스크래핑·권리 침해 (Compliance)** | 크롤러가 내부 메타데이터 IP(`169.254.169.254`)를 호출하거나 `noindex`/저작권 위반 문서를 노출 | 외부 크롤러 미활성화, OpenSearch `indexable=false` 문서 검색/추천 완전 배제(`search.spec.mjs` 4번 테스트 PASS) | Phase 2 SSRF/DNS rebinding/robots 통합 테스트(`024`~`029`) 및 법무 라이선스 승인(`002`, `060`, `G010`) |
| **광고 오인·부정 과금 (Financial)** | 유료 광고가 일반 검색 결과에 섞이거나 중복 웹훅으로 이중 청구 발생 | 광고·결제 기능 비활성화, DB 금액 컬럼 `bigint minor units` 및 음수 방지 `CHECK` 제약 적용 | `Sponsored` 분리(`076`), 멱등 원장·대조(`075`, `078`) 및 제3자 정산 감사(`080`) |

## 3. 독립 승인 게이트 패키지 및 서명 현황 (Independent Sign-off Package)

`CODEX_MASTER_PROMPT.md` 제0절 제10항 및 `docs/PHASES_0_TO_9.md` 작업 `010`에 따라, Phase 0 게이트 통과 및 후속 출시를 위해 독립 검토자에게 아래 증적 패키지를 제출한다:

1. **제품·아키텍처·권리 명세**: `docs/PRD.md`, `docs/PRODUCT_BOUNDARIES.md`, `docs/ARCHITECTURE.md`, `docs/SECURITY_AND_COMPLIANCE.md`, `docs/VERSIONS.md`, `OWNERS.md`, `docs/DECISIONS/`
2. **데이터베이스 및 계약 검증 증적**: `database/migrations/`, `proto/`, `docs/openapi.yaml`, `docs/events/index-event.v1.schema.json`, `docs/evidence/database.txt`, `docs/evidence/proto.txt`, `docs/evidence/contracts.txt`
3. **애플리케이션·보안·E2E 증적**: `docs/evidence/go-tests.txt`, `docs/evidence/go-coverage.out`, `docs/evidence/govulncheck.txt`, `docs/evidence/gitleaks.json`, `docs/evidence/npm-licenses.json`, `docs/evidence/browser-tests.txt`
4. **결함 및 상태 추적표**: `docs/IMPLEMENTATION_STATUS.md` (001~100 전체), `docs/GAP_REGISTER.md` (G001~G015), `docs/PHASE0_REPORT.md`

### 독립 승인 서명란 (위조 금지 — 현재 BLOCKED)

| 검토 영역 | 지정 검토자 | 검토 결과 | 서명 일시 / 증적 참조 | 차단 사유 |
|---|---|---|---|---|
| **독립 보안·아키텍처 감사** | *미지정 (Unassigned)* | **PENDING (차단)** | — | 외부 독립 보안 감사자 미지정 및 서명 미수령 (`G010`) |
| **법무·상표·데이터 권리 검토** | *미지정 (Unassigned)* | **PENDING (차단)** | — | 상표권·수집 대상·라이선스 법무 검토 미수령 (`G010`) |
| **컨테이너·원격 CI 인프라 검증** | *미지정 (Unassigned)* | **PENDING (차단)** | — | 로컬 호스트 Docker 부재(`G005`), CGO 부재(`G012`), 원격 GitHub CI 미연결(`G011`) |

> **최종 판정**: 위 외부 환경 제약(`G005`, `G011`, `G012`)과 독립 검토자 미지정(`G010`)으로 인해 **Phase 0 게이트는 NO-GO를 유지**한다. 단, 사용자 작업 지시 제3절에 따라 외부 승인 의존 기능(외부 크롤링, 실결제, 공개 배포)은 비활성 상태로 두고 독립 진행 가능한 **Phase 1(`011`~`020`) 구현 및 검증**을 선행 게이트 예외 기록 하에 진행한다.
