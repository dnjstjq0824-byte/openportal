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
| G006 | P0 | 출시 차단 (Phase 1/2/3 부분 해소) | 소스 검사: auth/MFA/테넌트 RLS/SSRF crawler/삭제·정산 통제 없음 | Phase 1에서 Passkey/TOTP MFA 인증, 단회성 계정 복구코드, RLS BOLA 격리. Phase 2에서 크롤러 RFC 9309 robots.txt, 엄격한 사설망/메타데이터 SSRF 차단, DNS Rebinding 방어 IP Pinning Dialer, 허가 도메인 게이트 구현. Phase 3에서 삭제/비공개/법적 제거 Tombstone(`is_tombstone=true`, `indexable=false`) 즉시 색인/캐시/Outbox 전파 구현 완료 | 024~029(크롤러 SSRF/robots/politeness PASS), 038(삭제전파 PASS), 078~090(정산/감사) 독립 테스트 |
| G007 | P1 | 수정·검증 | fixture만 있고 자체 fetch→parse→index가 없음 | Phase 2 자체 Bounded Crawler 및 Phase 3 자체 추출(Boilerplate 제거) -> SimHash 중복제거 -> Outbox -> OpenSearch Nori 색인 -> 검색 쿼리 파서 파이프라인 완성. 8/8 indexer, 9/9 search, 18/18 crawler 유닛/통합테스트 PASS | evidence/go-tests.txt PASS; Phase 4 랭킹/품질 평가 연계 |
| G008 | P2 | 수정·검증 | mapping content_analyzer=standard | Phase 3에서 OpenSearch 공식 `analysis-nori` 한국어 형태소 분석기 플러그인 탑재, mixed 복합어 분해, 품사(posfilter) 필터링, 영어 형태소(stemmer) 분석기 분리, documents_v2 매핑 및 Blue-Green 무중단 재색인/스왑/롤백 구현 완료 | `documents_v2` Nori 14개 형태소 토큰화 및 `TestReindexAndAliasLifecycle` PASS |
| G009 | P1 | 부분·차단 | Vue/Vite만 실행; Flutter SDK/플랫폼 호스트 없음 | 버전 후보와 Nuxt 이행 계획을 명시; Flutter UI 및 위젯 테스트 코드 작성 완료 | 011~020 cross-platform E2E; Flutter SDK 환경 구성 |
| G010 | P0 | 승인 차단 | 독립 감사·소스 권리·상표·실결제 승인 증적 없음 | 합성 데이터·로컬 전용, 공개/결제 금지 | 사업 소유자 및 독립 법무/보안/정산 검토 서명 |
| G011 | P2 | 차단 | 로컬 go vet/npm/gitleaks 성공과 원격 CodeQL은 별개 | CI 작성, 아직 remote 없음 | GitHub CI 실제 실행, action/image digest 고정, backend/image SBOM 확장 |
| G012 | P2 | 차단 | `go test -race ./...` → requires cgo | CGO/compiler 환경 미설정; 일반 tests 통과와 구분 | Linux CI race PASS |
| G013 | P1 | 수정·검증 | govulncheck: Go1.27.0 9건, 1.27.2 추가 인증서 1건 | Go1.27.3 최소 버전으로 변경 | govulncheck.txt: No vulnerabilities found; 과거 보고서 보존 |
| G014 | P2 | 수정·검증 | REST와 proto 필드·pagination 다름 | `backend/internal/search/proto_bridge.go` 구현: REST <-> Protobuf v1 양방향 변환 및 `Sponsored=false` 유기적 검색 분리 검증 완료 | TestProtoBridgeRoundTripAndOrganicIsolation PASS |
| G015 | P3 | 수정·검증 | 경쟁 기능 비교 틀만 작성 | `docs/PRODUCT_BOUNDARIES.md`에 Google/Naver/Bing 공식 기술자료 기반 비교표 및 IP/저작권/크롤링 권리 경계 분석 완료 | 공식 기술 자료 인용 완료; 독립 법무 서명은 외부 게이트 |
| G016 | P1 | 차단 / 완화 | Nuxt 3/4 도입 시 `@nuxt/devtools` -> `simple-git`, `node-forge` CVE로 인해 `npm audit` 규칙 위반 | 엄격한 제로 취약점 정책 유지를 위해 Vue 3 + Vite 기반 유지하며 Phase 1 포털 및 화면 구현 | 상위 Nuxt 패키지 취약점 패치 릴리스 시 업그레이드 |
| G017 | P1 | 차단 | Flutter SDK 부재로 `flutter test` 실행 불가 | `apps/mobile/lib/main.dart` 및 `test/widget_test.dart` 코드 작성 완료 | Flutter SDK 설치 후 위젯 및 플랫폼 통합 테스트 실행 |
| G018 | P1 | 수정·검증 | 2026-10-26 Phase 4 게이트웨이에 재랭킹을 연결한 뒤 `npm run test:e2e`: `lang=ko` 검색 결과의 `language`가 빈 값 → 1/5 실패(`search.spec.mjs:21`) | `cmd/api/main.go` 재랭킹이 `search.Hit`를 재구성하며 `Language` 누락 → 원본 hit 맵에서 복사. 회귀 단언을 `TestRankingAbuseAndModerationEndpoints`에 추가 | `go test -count=1 ./...` PASS, E2E 5/5 PASS (evidence/browser-tests.txt) |
| G019 | P2 | 열림 | 재랭킹 핸들러가 `PublishedAt=now`, `DomainAuthority=0.5`, `TermFrequencyDensity=0.05` 상수 사용 | 색인 문서에 신선도/도메인 권위/스팸 신호 필드가 없음; 현재는 http/https 가중치만 실질 영향 | 색인 파이프라인에서 실제 신호를 산출·저장하고 골든셋 NDCG로 효과 검증 |
| G020 | P1 | 열림 | 의도분류·실험 플랫폼·품질 게이트가 API/DB에 미연결, 남용탐지·신고/이의 상태는 프로세스 메모리 | 단위 테스트 수준 구현; `control.moderation_cases`/`query.experiments` 영속화 없음 | 영속 저장소 연결 + 재시작/다중 인스턴스 테스트 |
| G021 | P0 | 승인 차단 | 신고/이의 `POST /api/v1/control/moderation/*`는 익명 접근 가능, 검토·해결 API는 미노출 | 어뷰즈 방지(rate limit·인증) 및 관리자 인가 구축 전까지 운영 노출 금지 | 016/081 인증·인가 완료 후 재검증 |
| G022 | P1 | 차단 | 045/050 실제 인적 판정 데이터·독립 relevance 검토자 없음; 047 시맨틱 후보 생성기 없음 | 판정 도구·게이트 로직만 구현, 출시 승인 주장 불가 | 한국어/영어 판정 세트 수집, 독립 검토 서명 |
| G023 | P3 | 관찰 | 2026-10-26 재시작 전 첫 E2E 실행에서 8080 요청이 401(5/5 실패) — 원인 미규명; 재시작 후 재현되지 않음 | 해당 시점 8080에 다른 프로세스가 있었을 가능성(미확인) | 재발 시 포트 점유 프로세스 기록 |

## 실행 환경 장애

- G024 / P1 / 수정·로컬 검증 (2026-10-28): Go API의 동시 첫 요청이 서로 다른 identity/abuse/safety 객체를 받는 결함. `go test ./cmd/api -run TestConcurrentServiceInitialization -count=20`으로 수정 전 실패 재현, 초기화 mutex 적용 후 PASS. 전체 Go 테스트 PASS. CGO race 검증은 G012 차단 유지. 상세: [진행 보고서](PHASE_PROGRESS_2026_10_28.md).
- G021 정정 (2026-10-28): 실행 진입점 productionRoutes는 moderation을 포함한 변경 API를 404로 차단한다. 내부 routes의 익명 프로토타입은 운영 인가 구현이 아니며 공개하면 안 된다.

- 2026-10-20 Go1.27.2 업데이트 명령은 자동 승인 검토 사용량 제한으로 미실행됐다. 2026-10-21 Go1.27.3으로 수정 완료.
- 2026-10-24 OpenSearch 2.19.0 실행 시 JVM 힙 옵션 `-Xms512m -Xmx512m` 누락 시 메모리 할당 지연 발생. Node 백그라운드 헬스체크 및 데몬 스크립트로 자동화하여 해결.
- 2026-10-24 Nuxt 3/4 설치 시도 시 `node-forge` (CVE-2023-52078) 및 `simple-git` 고위험 취약점이 검출되어 즉시 롤백하고 무결점 Vite 기반 유지.
- 원격 승인·자격 증명이 필요한 배포/법무/결제/독립감사는 요청·수행한 것으로 꾸미지 않는다.
