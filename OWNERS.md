# Ownership, Repository Governance, and Review Policy — 003

## 1. 책임 주체 및 독립 검토 분리 원칙

- **저장소 소유자(Repository Owner)**: `@dnjstjqppk-dev` (로컬 Git 설정 기준)
- **기술 리드 및 구현 담당(Engineering Lead)**: Codex / Gemini 개발 에이전트
- **독립 감사 및 출시 승인권자(Independent Approvers — 현재 미지정/BLOCKED)**:
  - **보안·인프라 감사자(Security & Penetration Reviewer)**: 미지정 (Phase 0 `010`, Phase 8 `086`/`090`, Phase 9 `099` 필수)
  - **법무·개인정보·라이선스 검토자(Legal & Privacy Counsel)**: 미지정 (Phase 0 `002`/`010`, Phase 5 `060`, Phase 8 `090` 필수)
  - **재무·정산 감사자(Billing & Ledger Auditor)**: 미지정 (Phase 7 `080` 실결제 전환 전 필수)
  - **검색 품질 평가 책임자(Search Relevance Sign-off Reviewer)**: 미지정 (Phase 4 `050` 필수)

> **절대 규칙**: 코드를 작성한 에이전트나 단일 개발자는 독립 감사자 역할을 겸임할 수 없으며, 서명이나 승인 기록을 임의로 생성하지 않는다.

## 2. 모노레포 디렉터리 구조 및 경로별 검토 역량

| 디렉터리 / 경로 | 역할 및 경계 | 필수 검토 역량 | 관련 CODEOWNERS |
|---|---|---|---|
| `backend/` | Go 모듈러 백엔드 (API Gateway, Search, Identity, Crawl, Index, Ads, Control) | Go 동시성, 보안 헤더, 타임아웃, PII 비노출, 에러 처리 | `@dnjstjqppk-dev` |
| `proto/` | Protobuf v1 도메인 서비스 계약 및 Buf 설정 (`buf.yaml`, `buf.gen.yaml`) | 하위 호환성(`buf breaking`), 필드 번호 불변성, 타입 안전성 | `@dnjstjqppk-dev` |
| `database/migrations/` | PostgreSQL 정방향 마이그레이션(`001_initial.sql`, `002_...` 등) | FK 무결성, 트랜잭션 안전성, 최소 권한(GRANT/RLS), 무손실 진화 | `@dnjstjqppk-dev` |
| `apps/web/` | 사용자 검색 웹 포털 (Vue 3 / Nuxt 3 / TypeScript) | SSR 안전성, XSS 방어, 키보드·스크린리더 접근성, 반응형 UI, 한국어/영어 i18n | `@dnjstjqppk-dev` |
| `apps/admin/` | 관리자 콘솔 (Phase 8 전까지 기본 거부·비활성) | OIDC + 피싱 저항 MFA, 최소 권한 RBAC, 2인 승인, 전체 감사로그 | `@dnjstjqppk-dev` |
| `apps/partner/` | 파트너·웹마스터·광고주 센터 (Phase 7 전까지 기본 거부·비활성) | 테넌트 격리(BOLA 방어), 사이트 소유권 인증, 광고 원장 정합성 | `@dnjstjqppk-dev` |
| `apps/mobile/` | Flutter Android/iOS 클라이언트 (Dart) | 모바일 접근성, 뒤로가기/키보드/딥링크, 릴리스 평문 HTTP 차단 | `@dnjstjqppk-dev` |
| `infra/`, `.github/` | 로컬 Compose, 컨테이너 정의, GitHub Actions CI/보안 파이프라인 | 루프백 바인딩, 최소 권한 컨테이너, 공급망/SBOM/시크릿 스캔 | `@dnjstjqppk-dev` |
| `scripts/`, `tests/` | 마이그레이션 러너, 계약 검증, 시드, Playwright E2E 회귀 테스트 | 결정적 재현성, 격리된 임시 DB 사용, 실제 엔진 검증 | `@dnjstjqppk-dev` |
| `docs/` | PRD, ADR(`docs/DECISIONS/`), 스키마 카탈로그, 상태·결함 등록부, 실행 증적(`docs/evidence/`) | 증적 무결성, 원본 100개 작업 추적, GO/NO-GO 객관성 | `@dnjstjqppk-dev` |

## 3. 브랜치·커밋·PR 보호 규칙 (Branch & Commit Policy)

1. **기준 브랜치**: `main`은 항상 검증 스크립트(`test:db`, `test:proto`, `test:contracts`, `go test ./...`, 웹 빌드/E2E)가 통과한 상태로 유지한다.
2. **작업 브랜치 및 원자적 커밋**:
   - 기능·단계별 작업은 `phaseN/<topic>` 또는 원자적(atomic) 단위로 커밋한다.
   - 커밋 메시지는 Conventional Commits(`feat:`, `fix:`, `docs:`, `test:`, `chore:`) 형식을 따르며, 문서(`IMPLEMENTATION_STATUS.md`, `GAP_REGISTER.md`, Phase 리포트)에 실제 커밋 SHA를 기록한다.
3. **강제 차단 규칙**:
   - 이미 커밋된 마이그레이션 SQL 파일 변조 금지(신규 번호 파일로만 정방향 확장).
   - Protobuf 및 OpenAPI/이벤트 스키마의 하위 호환성 파괴 금지.
   - 시크릿(`.env`, 개인키, API 토큰), `node_modules/`, 로컬 임시 DB(`work/`) 커밋 금지.
4. **원격 저장소 브랜치 보호(Remote Branch Protection — 현재 차단)**:
   - `.github/CODEOWNERS` 및 `.github/workflows/phase0.yml`이 정의되어 있으나, 현재 로컬 Git 저장소에 연결된 원격(GitHub 등) 호스팅 서버가 없으므로 서버 측 Branch Protection Rule(필수 상태 체크, 1인 이상 독립 리뷰 승인 강제, 강제 푸시 차단) 적용은 **원격 저장소 프로비저닝 전까지 차단(G011)** 상태다.
