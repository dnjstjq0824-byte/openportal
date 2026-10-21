# P0–P3 결함·출시 차단 등록부 — 2026-10-21

구현 SHA `ba2c1a5badf11b5fe3a7fbe390227a3d4067db01`. 담당은 별도 명시 없으면 기술 리드. P0 즉시 출시차단 보안/정산/권리,
P1 핵심 기능, P2 중요 품질, P3 개선. 아래 미구현 통제는 현재 공격 성공이 확인되었다는 뜻이 아니라 출시 전 필수 게이트다.

| ID | 심각도 | 상태 | 재현/기대→실제 | 원인·수정 또는 완화 | 회귀/종료 조건 |
|---|---|---|---|---|---|
| G001 | P1 | 수정·검증 | `go test ./internal/search -run TestIncompleteSearchFailsClosed`; 부분 실패는 오류여야 하나 기존엔 빈 성공 | timed_out/failed shards/hits 누락/JSON trailing 검사, 2MiB 상한 | evidence/go-tests.txt PASS |
| G002 | P2 | 수정·검증 | `buf lint proto`; 초기 RPC 이름 6개 실패 | 미배포 ads/portal 이름 수정, go_package 교정·생성 | evidence/proto.txt; breaking negative control PASS |
| G003 | P1 | 수정·검증 | bulk HTTP200에 errors=true면 실패해야 하나 초기 seed는 성공 표시 | scripts/seed.py 오류/개수 검사 | 실제 정상 seed PASS; seed-unit.txt: item-error/누락 item 거부 포함3건 PASS |
| G004 | P1 | 수정·검증 | 초기 DB 이력/체크섬 없음 | advisory lock·DDL+이력 원자 트랜잭션, PUBLIC revoke | database.txt: 재실행/변조/실패 rollback/권한거부 PASS |
| G005 | P1 | 차단 | `docker compose -f infra/compose.yaml --profile app up --build`; 실행돼야 하나 Docker 없음 | native PG/OpenSearch로 별도 실행 증거 확보 | Docker 호스트에서 Compose full stack 재검증 |
| G006 | P0 | 출시 차단 | 소스 검사: auth/MFA/테넌트 RLS/SSRF crawler/삭제·정산 통제 없음 | admin/partner 404, crawling·광고·결제 비활성 | 015~016, 024~029, 038, 078~090 독립 테스트 |
| G007 | P1 | 미구현 | fixture만 있고 자체 fetch→parse→index가 없음 | 크롤링을 완료로 주장하지 않음 | 021~040 실제 허가 corpus E2E |
| G008 | P2 | 미구현 | mapping content_analyzer=standard | Nori·한국어 품질/세이프서치·정량 relevance 없음 | 033, 041~050 실제 평가 |
| G009 | P1 | 부분·차단 | Vue/Vite만 실행; Flutter SDK/플랫폼 호스트 없음 | 버전 후보와 Nuxt 이행 계획을 명시 | 011~020 cross-platform E2E |
| G010 | P0 | 승인 차단 | 독립 감사·소스 권리·상표·실결제 승인 증적 없음 | 합성 데이터·로컬 전용, 공개/결제 금지 | 사업 소유자 및 독립 법무/보안/정산 검토 서명 |
| G011 | P2 | 차단 | 로컬 go vet/npm/gitleaks 성공과 원격 CodeQL은 별개 | CI 작성, 아직 remote 없음 | GitHub CI 실제 실행, action/image digest 고정, backend/image SBOM 확장 |
| G012 | P2 | 차단 | `go test -race ./...` → requires cgo | CGO/compiler 환경 미설정; 일반 tests 통과와 구분 | Linux CI race PASS |
| G013 | P1 | 수정·검증 | govulncheck: Go1.27.0 9건, 1.27.2 추가 인증서 1건 | Go1.27.3 최소 버전으로 변경 | govulncheck.txt: No vulnerabilities found; 과거 보고서 보존 |
| G014 | P2 | 부분 | REST와 proto 필드·pagination 다름 | 별도 미래 계약으로 명시, 직접 REST response schema 검사 | gRPC 활성화 전에 동등성/자동 변환 테스트 |
| G015 | P3 | 진행 | 경쟁 기능 비교 틀만 작성 | 현재 제품 기능·약관 조사 미수행으로 완료 표시 금지 | 공식 자료 기반 비교표 검증 |

## 실행 환경 장애

2026-10-20 Go1.27.2 업데이트 명령은 자동 승인 검토 사용량 제한으로 미실행됐다. 안전성 거부가 아니었다.
2026-10-21 사용자 재개 후 승인을 받아 수행했고 최종 1.27.3으로 수정했다. 해당 일시 차단은 해소됐다.
PowerShell coverage 인자가 분리되어 한 실행이 FAIL이었다. 인자를 인용해 재실행했으며 실패 원문도 보존한다.
원격 승인·자격 증명이 필요한 배포/법무/결제/독립감사는 요청·수행한 것으로 꾸미지 않는다.
