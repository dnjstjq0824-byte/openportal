# P0–P3 결함·출시 차단 등록부 — 2026-10-24

구현 SHA `ba2c1a5badf11b5fe3a7fbe390227a3d4067db01` (이전 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`).
담당은 별도 명시 없으면 기술 리드. P0 즉시 출시차단 보안/정산/권리, P1 핵심 기능, P2 중요 품질, P3 개선.
아래 미구현 통제는 현재 공격 성공이 확인되었다는 뜻이 아니라 출시 전 필수 게이트다.

| ID | 심각도 | 상태 | 재현/기대→실제 | 원인·수정 또는 완화 | 회귀/종료 조건 |
|---|---|---|---|---|---|
| G001 | P1 | 수정·검증 | `go test ./internal/search -run TestIncompleteSearchFailsClosed`; 부분 실패는 오류여야 하나 기존엔 빈 성공 | timed_out/failed shards/hits 누락/JSON trailing 검사, 2MiB 상한 | evidence/go-tests.txt PASS |
| G002 | P2 | 수정·검증 | `buf lint proto`; 초기 RPC 이름 6개 실패 | 미배포 ads/portal 이름 수정, go_package 교정·생성 | evidence/proto.txt; breaking negative control PASS |
| G003 | P1 | 수정·검증 | bulk HTTP200에 errors=true면 실패해야 하나 초기 seed는 성공 표시 | scripts/seed.py 오류/개수 검사 | 실제 정상 seed PASS; seed-unit.txt: item-error/누락 item 거부 포함 3건 PASS |
| G004 | P1 | 수정·검증 | 초기 DB 이력/체크섬 없음 | advisory lock·DDL+이력 원자 트랜잭션, PUBLIC revoke | database.txt: 재실행/변조/실패 rollback/권한거부 PASS |
| G005 | P1 | 차단 | `docker compose -f infra/compose.yaml --profile app up --build`; 실행돼야 하나 Docker 없음 | native PG17.10/OpenSearch2.19.0으로 별도 실행 증거 확보 | Docker 호스트에서 Compose full stack 재검증 |
| G006 | P0 | 출시 차단 (Phase 1/2 부분 해소) | 소스 검사: auth/MFA/테넌트 RLS/SSRF crawler/삭제·정산 통제 없음 | Phase 1에서 Passkey/TOTP MFA 인증, 단회성 계정 복구코드, RLS BOLA 격리, 콘솔 경계 차단 구현. Phase 2에서 크롤러 RFC 9309 robots.txt, 엄격한 사설망/메타데이터 SSRF 차단, DNS Rebinding 방지 IP Pinning Dialer, 2MiB 크기제한 및 압축폭탄 방어, 허가 도메인 한정 Bounded Corpus 게이트 구현 완료 | 024~029(크롤러 SSRF/robots/politeness PASS), 038(삭제전파), 078~090(정산/감사) 독립 테스트 |
| G007 | P1 | 수정·검증 | fixture만 있고 자체 fetch→parse→index가 없음 | Phase 2에서 자체 Bounded Crawler 파이프라인 (canonicalize -> robots RFC9309 -> SSRF/IP Pinning -> politeness -> safe fetch 2MB -> WARC ISO 28500 -> HTML/X-Robots-Tag/canonical/sitemap parse -> frontier leasing -> engine 허가 도메인 한정 수집) 구현 완료. 18/18 crawler 유닛/통합테스트 PASS | evidence/go-tests.txt PASS; Phase 3 색인 파이프라인 연계 |
| G008 | P2 | 미구현 | mapping content_analyzer=standard | Nori·한국어 품질/세이프서치·정량 relevance 없음 | 033, 041~050 실제 평가 |
| G009 | P1 | 부분·차단 | Vue/Vite만 실행; Flutter SDK/플랫폼 호스트 없음 | 버전 후보와 Nuxt 이행 계획을 명시; Flutter UI 및 위젯 테스트 코드 작성 완료 | 011~020 cross-platform E2E; Flutter SDK 환경 구성 |
| G010 | P0 | 승인 차단 | 독립 감사·소스 권리·상표·실결제 승인 증적 없음 | 합성 데이터·로컬 전용, 공개/결제 금지 | 사업 소유자 및 독립 법무/보안/정산 검토 서명 |
| G011 | P2 | 차단 | 로컬 go vet/npm/gitleaks 성공과 원격 CodeQL은 별개 | CI 작성, 아직 remote 없음 | GitHub CI 실제 실행, action/image digest 고정, backend/image SBOM 확장 |
| G012 | P2 | 차단 | `go test -race ./...` → requires cgo | CGO/compiler 환경 미설정; 일반 tests 통과와 구분 | Linux CI race PASS |
| G013 | P1 | 수정·검증 | govulncheck: Go1.27.0 9건, 1.27.2 추가 인증서 1건 | Go1.27.3 최소 버전으로 변경 | govulncheck.txt: No vulnerabilities found; 과거 보고서 보존 |
| G014 | P2 | 수정·검증 | REST와 proto 필드·pagination 다름 | `backend/internal/search/proto_bridge.go` 구현: REST <-> Protobuf v1 양방향 변환 및 `Sponsored=false` 유기적 검색 분리 검증 완료 | TestProtoBridgeRoundTripAndOrganicIsolation PASS |
| G015 | P3 | 수정·검증 | 경쟁 기능 비교 틀만 작성 | `docs/PRODUCT_BOUNDARIES.md`에 Google/Naver/Bing 공식 기술자료 기반 비교표 및 IP/저작권/크롤링 권리 경계 분석 완료 | 공식 기술 자료 인용 완료; 독립 법무 서명은 외부 게이트 |
| G016 | P1 | 차단 / 완화 | Nuxt 3/4 도입 시 `@nuxt/devtools` -> `simple-git`, `node-forge` CVE로 인해 `npm audit` 규칙 위반 | 엄격한 제로 취약점 정책 유지를 위해 Vue 3 + Vite 기반 유지하며 Phase 1 포털 및 화면 구현 | 상위 Nuxt 패키지 취약점 패치 릴리스 시 업그레이드 |
| G017 | P1 | 차단 | Flutter SDK 부재로 `flutter test` 실행 불가 | `apps/mobile/lib/main.dart` 및 `test/widget_test.dart` 코드 작성 완료 | Flutter SDK 설치 후 위젯 및 플랫폼 통합 테스트 실행 |

## 실행 환경 장애

- 2026-10-20 Go1.27.2 업데이트 명령은 자동 승인 검토 사용량 제한으로 미실행됐다. 2026-10-21 Go1.27.3으로 수정 완료.
- 2026-10-24 OpenSearch 2.19.0 실행 시 JVM 힙 옵션 `-Xms512m -Xmx512m` 누락 시 메모리 할당 지연 발생. Node 백그라운드 헬스체크 및 데몬 스크립트로 자동화하여 해결.
- 2026-10-24 Nuxt 3/4 설치 시도 시 `node-forge` (CVE-2023-52078) 및 `simple-git` 고위험 취약점이 검출되어 즉시 롤백하고 무결점 Vite 기반 유지.
- 원격 승인·자격 증명이 필요한 배포/법무/결제/독립감사는 요청·수행한 것으로 꾸미지 않는다.
