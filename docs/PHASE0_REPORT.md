# OpenPortal Search / Phase 0 / Tasks 001–010

판정: **NO-GO**. 실제 로컬 구현·검증을 진행했지만 Phase 0 전체 완료나 상용화 PASS가 아니다.
구현 기준 커밋: `ba2c1a5badf11b5fe3a7fbe390227a3d4067db01`. 원본 import: `be0850836f8125683d8f7fa8377aab66befe5050`.
2026-10-20~21, Windows x64 / Asia-Seoul. 001~100 전체 상태와 미해결은 IMPLEMENTATION_STATUS.md/GAP_REGISTER.md 참조.

## 실제 변경

- PostgreSQL 원본 001 보존, 002 PUBLIC 접근 차단, migration runner·일회용 DB 검증 스크립트 추가.
- Go API: 부분 shard 실패/timeout/잘못된 응답 차단, 응답 크기 제한, /readyz, loopback 기본 바인딩, 오류 로그 최소화.
- Protobuf lint 오류·Go module 경로 수정, 메시지 생성물, breaking negative test, REST/event 계약 검사 추가.
- Vue 타입검사·빌드 버전 잠금, Compose proxy 설정, 중복 검색 요청 방지, 실제 브라우저 회귀 추가.
- Go1.27.3으로 보안 업데이트, 의존성 lock/SBOM/라이선스·시크릿·취약점 검사, CI 정의 추가.
- PRD/권리 경계/OWNERS/버전 정책/ADR/위협모델 작성. admin/partner/mobile/crawler 제품 구현은 하지 않았다.

## 실행 명령과 결과

아래는 저장소 루트 기준이며 Go 명령은 backend에서 실행했다. Python은 번들 Python3.12.14 사용.

| 명령 | 결과 | 증적/한계 |
|---|---|---|
| `node scripts/test-database.mjs` | PASS | evidence/database.txt; 실제 PG17.10, 86 domain tables/8 schemas/75 FK |
| 동일 DB suite | PASS | FK/CHECK/UNIQUE 거부, rollback, 정방향 repeat, checksum drift, 실패 DDL+ledger rollback, untrusted role deny |
| `go test -count=1 '-coverprofile=../docs/evidence/go-coverage.out' ./...` | PASS | go-tests.txt; API83.8%, search69.0%; generated/config0%도 공개 |
| `go vet ./...` | PASS | go-vet.txt (출력 없음) |
| `go test -race ./...` | NOT RUN 성공 아님 | go-race.txt; CGO 필요 |
| `node scripts/test-proto.mjs` | PASS | proto.txt; lint/build/breaking 및 type-change 거부 |
| `node scripts/check-contracts.mjs` | PASS | contracts.txt; event valid/invalid 및 API breaking negative control |
| `npm run typecheck --prefix apps/web` | PASS | web-typecheck.txt |
| `npm run build --prefix apps/web` | PASS | web-build.txt |
| `python scripts/seed.py` | PASS | seed.txt; 합성 문서3건, 실제 수집 아님 |
| `npx playwright test` (CHROME_PATH 지정) | PASS | browser-tests.txt; 실제 OpenSearch/Go/Vue, 한국어/영어/빈 결과/입력/제안/권한거부/indexable=false; UI503은 명시적 fault injection |
| `govulncheck ./...` | PASS | govulncheck.txt; Go1.27.3, 검출 없음; 이전 실패 보존 |
| `npm audit --json`, web 동일 | PASS | npm-audit.json/web-audit.json, 검출0; 전체 보안 승인 아님 |
| `node scripts/check-licenses.mjs` | PASS | npm-licenses.json; 선언된 license129개 확인, 법무 승인 아님 |
| `gitleaks dir . --redact ...` | PASS | gitleaks.txt/json, 소스 검사; work/node_modules 제외 |
| `npm sbom --sbom-format cyclonedx` (root/web) | 생성 | tooling-sbom.json/web-sbom.json; OS image/Go backend 전체 SBOM 아님 |
| Docker Compose / 원격 CodeQL | NOT RUN | Docker 없음 / 원격 저장소 실행 권한·대상 없음 |

## 재현 데모와 경계

README의 로컬 실행 절차와 tests/search.spec.mjs를 사용한다. OpenSearch는 127.0.0.1:9200,
API8080, Vue5173으로 loopback 한정이다. fixture `검색엔진`→한국어 문서,
`OpenPortal`→영문 문서, 없는 검색어→0건, page101→400, upstream 장애→503.
검색 화면 스크린샷: evidence/search-browser.png. synthetic fixture E2E는 crawler·독립 수집이나 검색 품질 완성을 입증하지 않는다.
PostgreSQL은 control plane이며 Go 검색 요청이 DB를 쓰지 않는다. 실제 관계형 migration 검증과 검색 데이터 경로를 구분한다.
Protobuf 생성물은 메시지 계약이고 gRPC 서비스가 아니다. 마이그레이션 이력은 public에 별도1개 테이블이다.

## GO/NO-GO 근거 및 다음 의존성

로컬 핵심 slice는 실행됐으나 G005/G009/G010/G011/G012/G014가 남아 Phase 0 GO를 권고하지 않는다.
Docker 환경→Compose 전체 실행, CGO 환경→race, 원격 CI→CodeQL/보호규칙, 독립 검토자→증적 승인 순으로 해소한다.
운영 비용·QPS/p95·보존/백업 복구·한국어 relevance 수치는 측정하지 않았다. 월 비용을 추정 성과처럼 표시하지 않는다.
외부 배포/실제 crawling/결제/개인정보 수집은 활성화하지 않았다. 후속 011~100은 원본 순서와 게이트대로 진행해야 한다.
