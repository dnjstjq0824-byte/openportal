# Phase 진행 검증 — 2026-10-28

판정: 전체 Phase NO-GO 유지. 이번 결과는 로컬 검증이며 독립 승인이나 배포 증거가 아니다. 변경은 미커밋 작업 트리 기준이다.

## 소스 구현

- [Go API](../backend/cmd/api/main.go)의 미사용 초기화 메서드 5개/전용 필드 제거 및 SafeSearch switch 정리.
- 공유 서비스의 지연 초기화 16곳을 하나의 초기화 mutex로 보호한다. 객체 생성/조회만 잠그며 서비스의 작업 자체는 기존 내부 동기화를 사용한다.
- [동시 초기화 회귀 테스트](../backend/cmd/api/initialization_test.go): identity/abuse/ranking/safety/moderation에 각각 64개 동시 호출을 보내 동일 인스턴스 반환을 검사한다. 수정 전 20회 반복 실행은 서로 다른 인스턴스 반환으로 실패했고 수정 후 통과했다.
- 실행 진입점은 productionRoutes 공개 읽기 allowlist를 사용한다. 인증 프로토타입은 WebAuthn 서명을 검증하지 않으므로 admin/partner/auth 및 변경 API는 계속 비공개다. 기존 상태표의 인증 관련 단위 테스트를 제품 인증 완료로 해석하면 안 된다.

## 실행 증거

Windows 로컬에서 실행했다. Go 명령의 작업 디렉터리는 backend다.

| 명령 | 결과 | 한계 |
|---|---|---|
| `go test ./...` | PASS | 일부 패키지는 캐시 사용; E2E 승인 아님 |
| `go test ./cmd/api -run TestConcurrentServiceInitialization -count=20` | 수정 전 FAIL, 수정 후 PASS | race detector와 별개 |
| `go test -race ./cmd/api` | BLOCKED | `-race requires cgo`; gcc/clang PATH에서 발견되지 않음 |
| `node scripts/check-contracts.mjs` | PASS | OpenAPI/event 및 breaking negative control |
| `node scripts/test-database.mjs` | PASS | 아래 DB 범위만 검증 |
| `npm run typecheck --prefix apps/web` | PASS | Vue 타입 검사 |
| `npm run build --prefix apps/web` | PASS | Vite 빌드 |
| `node --test tests/pages-worker.test.mjs` | PASS, 6 tests | 로컬 테스트; 실제 배포/브라우저 검증 아님 |

## SQL 기본 설계와 100개 계획의 구분

SQL은 변경하지 않았다. 일회용 PostgreSQL 17.10에서 기본 86테이블/8스키마/75 FK를 검증했다. 추가 migration 001–011 적용과 재실행, FK/CHECK/UNIQUE 위반 거부, 실패 DDL+이력 rollback, checksum drift 거부, 비인가 접근 차단 및 계정 RLS 테스트도 통과했다. 테이블과 feature flag의 존재는 해당 서비스 구현 완료를 뜻하지 않는다.

100개 작업의 번호·체크박스는 유지한다. Compose 전체 실행, CGO race, 원격 CI, 실제 인증/영속 데이터 경로, Flutter 및 각 Phase E2E, 출처 권리와 독립 검토 증거 등은 남아 있다. 기존 상태표의 ‘검증완료’는 표에 적힌 단위/로컬 범위로만 해석한다. 다음 작업은 Phase 0 실행 환경 차단 해소와 Phase 1 실제 인증 경로 구축이며, 외부 승인 없이 결제·배포·임의 도메인 수집을 활성화하지 않는다.
