# Phase 4 보고서 — 랭킹 품질·안전 검색 (Tasks 041–050)

작성일: 2026-10-26 · 환경: Windows, Go 1.27.3, Node 24.19.0, PostgreSQL 17.10(embedded, 일회용), OpenSearch 2.19.0(native, analysis-nori)
커밋: 본 보고서를 포함한 단일 커밋 (실제 SHA는 `git log -1` 및 원격 검증 결과 참조; 보고서 내부에는 자기 SHA를 기록할 수 없음)

## 판정: **NO-GO** (Phase 0~4 모두 유지)

사유: Docker Compose 전체 스택·CGO race·원격 CI/CodeQL 미검증, 독립 감사/relevance sign-off 미수령, 실제 인적 판정 데이터 없음. 로컬에서 가능한 검증만 수행했다.
Phase 0 NO-GO 상태에서의 Phase 1~4 진행은 "선행 게이트 예외"로 기록하며 GO 선언이 아니다.

## 변경 파일
- 신규: `backend/internal/ranking/{intent,abuse,rerank,safesearch,eval,ndcg,rrf,benchmark,experiment,signoff}.go` 및 각 `_test.go`, `pipeline_test.go`
- 신규: `database/migrations/006_phase4_ranking_quality_and_safety.sql`, `docs/QUALITY_EVALUATION_GUIDE_KO.md`, 본 보고서
- 수정: `backend/cmd/api/main.go`(남용탐지/SafeSearch/재랭킹 연결, moderation report/appeal 엔드포인트), `backend/cmd/api/http_test.go`, `scripts/test-database.mjs`(마이그레이션 6건 단언, 실패 픽스처를 `007_failure.sql`로 변경), `docs/IMPLEMENTATION_STATUS.md`, `docs/GAP_REGISTER.md`, `docs/evidence/*`

## 실행 결과 (2026-10-26, 이 환경)
| 검증 | 명령 | 결과 |
|---|---|---|
| Go 단위/통합 | `cd backend; go test -count=1 ./...` | PASS (api/config/crawler/identity/indexer/ranking/search) |
| go vet | `cd backend; go vet ./...` | PASS (출력 없음) |
| DB 마이그레이션 | `node scripts/test-database.mjs` | PASS: 86 테이블/8 스키마/FK 75, 001~006 적용·재실행·실패 rollback·체크섬 변조 거부·권한 거부·RLS |
| 계약/Proto | `node scripts/test-proto.mjs; node scripts/check-contracts.mjs` | PASS |
| Seed 단위 | `python -m unittest discover -s scripts -p test_seed.py` | PASS (3) |
| Web | `npm run typecheck`, `npm run build` (apps/web) | PASS |
| E2E | `npm run test:e2e` (실제 OpenSearch→Go→Vue/Chrome) | PASS 5/5 (수정 후) |
| govulncheck | `govulncheck ./...` | No vulnerabilities found |
| gitleaks | `gitleaks dir .` / `gitleaks git .` | no leaks (작업트리 2.38MB, 7 커밋) |
| go test -race | — | BLOCKED (CGO/C compiler 없음) |
| Docker Compose | — | BLOCKED (Docker 없음) |
| 원격 CI/CodeQL | — | NOT RUN |

### 발견·수정된 결함
- **G018 (P1)**: 재랭킹 연결 시 `language` 필드 누락으로 E2E 1건 실패 → 수정 및 회귀 단언 추가. 첫 실행(서버 재시작 전)의 5/5 실패(401)는 원인 미규명(G023).

## 작업별 상태 (과대 주장 금지)
041~049는 **단위 테스트 수준의 라이브러리 구현**이며 "부분"이다. 050은 독립 sign-off 부재로 "차단". 세부 한계는 `IMPLEMENTATION_STATUS.md`와 `GAP_REGISTER.md`(G019~G022) 참조. 핵심:
- 재랭킹은 현재 상수 신호를 사용(G019). 의도분류/실험은 API 미연결(G020).
- SafeSearch는 키워드 휴리스틱이며 분류 성능 평가 전.
- 시맨틱 후보 생성기가 없어 RRF는 알고리즘만 검증.
- 신고/이의 엔드포인트는 인증·rate limit 없음 → 운영 노출 금지(G021).

## 미해결 P0~P3
P0: G010(독립 감사·법무·상표·결제 승인), G021(모더 엔드포인트 접근통제) · P1: G005, G006(잔여), G009, G017, G020, G022 · P2: G011, G012, G019 · P3: G023

## 다음 세션
Phase 4 잔여(실신호 연동·영속화·판정 데이터) 후 Phase 5(051~060)로 진행하되, 라이선스/출처 권리 게이트 전에는 vertical을 비활성 유지한다.

