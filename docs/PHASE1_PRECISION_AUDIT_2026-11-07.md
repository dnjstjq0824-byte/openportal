# PHASE 1 정밀 분석 리포트 — OpenPortal Search / OpenScanner (2026-11-07)

> 방법: 5개 병렬 감사 레인(Go 백엔드 / 프론트·디자인 / 게이트웨이·DB·인프라 / 문서·계약·테스트 / 앱·법무·운영)이
> 구현·테스트 본문을 직접 열어 증거(`파일:줄: 사실`)를 수집했고, 부모가 핵심 주장 6건을 2차 대조했다.
> 추측·미열람 항목은 §9 미확인 목록에 분리했다.

## 1. 플랫폼 정체

- **정체**: 독립 검색 포털 + 브라우저 계정/메일 게이트웨이를 결합한 개발 중 플랫폼.
  `docs/openapi.yaml:3`은 API 정체를 `OpenPortal Search development API v0.1.0`으로 고정한다.
- **브랜드 혼재 (확정 결함)**: 문서는 OpenPortal, 실제 화면은 OpenScanner
  (`apps/web/src/App.vue:709` 404 표기 `404 / OPENSCANNER`, `:645` 인증 비주얼, `:712` 푸터).
  메일 도메인도 미확정이다 (`docs/GAP_REGISTER.md:20`, `omail.com` 소유 미확인).
- **핵심 도메인**: 검색(크롤→색인→랭킹→서빙) + 사용자 계정/메일 + 광고(GSP 경매) + 버티컬(이미지·뉴스·지도·쇼핑·동영상 뼈대).
- **비즈니스 모델**: 광고 GSP 2위가격 경매·예산원장·CPC/CPI (`docs/IMPLEMENTATION_STATUS.md:95`,
  `backend/internal/ads/auction.go:36`). 단 실결제는 이중승인·SOC 감사 없이 NO-GO 샌드박스 강제
  (`backend/internal/ads/signoff.go:40-42`)이며 Stripe·Toss 호출 코드는 전무하다 (전수 검색 0건).

## 2. 기술 스택 (코드·문서 대조)

| 영역 | 스택 | 근거 |
|---|---|---|
| Frontend | Vue 3.5 + Vite 6 SPA, 수제 History 라우팅, 라우터 없음 | `apps/web/package.json`, `App.vue:189-236` |
| Backend | Go 1.27, 표준 http.ServeMux, protobuf | `backend/go.mod`, `backend/cmd/api/main.go` |
| Gateway | Node 24 ESM + `pg` + `nodemailer` + `sanitize-html` | `package.json:25-29`, `gateway/server.mjs` |
| DB | PostgreSQL 17, 13개 마이그레이션, 114 테이블 | `database/migrations/`, `scripts/migrate.mjs` |
| Index | OpenSearch 2.19 (BM25), 로컬 보안플러그인 비활성 | `infra/compose.yaml`, `infra/opensearch/` |
| Mobile | Flutter Material3 (소스만, SDK 빌드 미검증) | `apps/mobile/lib/main.dart`, `pubspec.yaml` |
| Infra | Compose 로컬/클러스터, nginx, Prometheus+Grafana, Cloudflare Pages | `infra/`, `wrangler.toml` |
| Auth | Go 불투명 세션토큰 + Gateway 세션쿠키(이중화) + OIDC(go-oidc) | `identity.go:773`, `gateway/security.mjs` |
| Mail | JMAP + SMTP(nodemailer) + ClamAV 스캔 | `gateway/mail.mjs`, `backend/internal/email/` |
| 외부검색 | Brave API + RRF 하이브리드 (Bing은 2025-08 종료로 503 고정) | `gateway/search.mjs:74-217` |
| CI | GitHub Actions (phase0.yml 회귀·보안, deploy-pages.yml) | `.github/workflows/` |

실측 버전(2026-11-07): Go 1.27.0, Node 24.19.0, Python 3.14.10.
문서 고정값(Go 1.27.3, Py 3.12)과 경미한 차이가 있다.

## 3. 아키텍처와 데이터 플로우

```
[Vue SPA :5173] ──/api/v2──▶ [Node Gateway :8090] ──/api/v1──▶ [Go API :8080] ──▶ [OpenSearch :9200]
      │                        │  auth·mail(JMAP)·search 분기        │  search·crawler·indexer·ranking
      │                        └───────────▶ [PostgreSQL webapp 스키마] │
[Flutter] ──/api/v1 직결──▶ [Go API] (게이트웨이 우회)                  └▶ [PostgreSQL 8스키마 86테이블]
[Pages _worker.js] ──X-Gateway-Key──▶ [Gateway] (WEB_GATEWAY_ORIGIN 주석 미연결)
```

- **이중 API 계층 (구조 결함)**: 프론트 실제 연동은 `/api/v2`(게이트웨이)인데 계약 문서는 `/api/v1`(Go) 기준이다
  (`apps/web/src/client.ts:6`, `docs/API_CONTRACT.md:5`). 모바일은 게이트웨이를 우회해 `:8080` 직결
  (`apps/mobile/lib/main.dart:7`)이라 인증·정책이 갈라진다.
- **Go 운영 노출**: `productionRoutes()` allowlist가 `/healthz·/readyz·/api/v1/search·/api/v1/suggest` 4개만 서빙하고
  나머지는 404 (`backend/cmd/api/main.go:2220-2224`, `production_test.go:11-54` 고정). 내부 `routes()`에는
  약 45개 프로토타입 핸들러가 등록되어 있다 (`main.go:2190` 주석).
- **게이트웨이**: auth 8종·search·mail·account 실구현 (`gateway/server.mjs:159-367`),
  DB 토큰버킷 전역 300/분 + 경로별 제한, CSRF 상시 비교, production ingress 시크릿 게이트 (`server.mjs:141-153`).
- **데이터**: 크롤→WARC→추출→SimHash→outbox→OpenSearch 적재 실파이프라인
  (`crawler/engine.go:87`, `indexer/pipeline.go:55`). 삭제 tombstone·blue/green reindex 뼈대 존재.
- **ERD**: identity/crawl/catalog/query/portal/ads/publisher/control 8스키마 86테이블 기준선 + webapp 7테이블
  (`001_initial.sql`, `012_web_gateway_accounts.sql:3-55`, `013_webapp_mailbox_signup.sql`).
- **관측**: `/metrics`·Prometheus 스크랩·알림 규칙 존재하나 게이트웨이 잡 없음, Grafana 기본 비번 노출
  (`infra/prometheus.yml`, `infra/compose.cluster.yml:199-223`).

## 4. 구현 완료 vs 미구현·스텁

### 4.1 실제 동작 (본문 확인)

- 검색·서제스트 OpenSearch 실호출 (`search.go:142,197`), 크롤러 실파이프라인, 인덱서·리랭크·GSP 경매 실연산.
- 게이트웨이 auth 8종·세션 슬라이딩·메일 JMAP·ClamAV·첨부 매직바이트 검증 (`server.mjs`, `mail.mjs:12-192`).
- Brave 실호출 + RRF 하이브리드, 한쪽 실패 시 degraded (`search.mjs:74-217`).
- OIDC 실검증(HTTPS·nonce·azp) + TOTP MFA (`oidc.go:30`, `identity.go:346`).
- 프론트: 로딩 스켈레톤·에러 5종·빈결과·페이지네이션·aria·메일 sandbox iframe 완비
  (`App.vue:573-637`, `MailView.vue:222-230`), CSRF 포함 실호출 (`client.ts:6-12`).
- 모바일: 검색·히스토리 opt-in·세이프서치 동작 앱 (`main.dart:53-318`).
- 테스트: Go `_test.go` 80개·비테스트 85개, 40KB 통합 HTTP·동시성·골든 테스트, Playwright 13종.

### 4.2 미구현·스텁·차단 (본문 확인)

- AI RAG는 LLM 미연동 추출요약 (`rag.go:76` 명시), vertical은 하드코딩 시드 (`service.go:64`),
  superplatform은 하드코딩 배열 (`main.go:1986`), gate-status는 고정 체크리스트 (`main.go:205`).
- 어드민·파트너 콘솔은 의도적 미구현 (README 2종).
- 메일박스 프로비저닝은 운영자 CLI만, HTTP API 없음 (`provision-mail.mjs`). 미주입 시 가입 fail-closed
  (`server.mjs:66-69`).
- 광고원장은 인메모리만, 영속·복식부기 없음 (`ledger.go:42-88`). 실결제 코드 0건.
- 약관·개인정보는 초안 + `policyDraft` 배너 상시 노출 (`i18n.ts:4`, `App.vue:707`). 500 전용 페이지 없음.
- `titles`의 store/advertising/business는 라우트 분기 없이 404 폴백 (`App.vue:156-171` vs `:549-709`).
- 데드코드: `style.css`(101KB, 미로드·이중 토큰 `#1a73e8`), `api.ts`(미사용) — `main.ts:1-4`는 `portal.css`만 번들.
- TODO/FIXME/미구현 문자열은 backend·gateway·apps 전수 검색 0건 (스텁은 명시적 fail-closed/주석 형태).

## 5. 상용화까지의 치명적 Gap

1. **전체 판정 NO-GO**: Phase0~8 NO-GO, Phase9 사내 카나리만 조건부 (`GAP_REGISTER.md:13`, `IMPLEMENTATION_STATUS.md:123-132`).
2. **이중 인증·이중 세션**: Go 자작 sha256 5000회(`identity.go:643-678`, 표준 PBKDF2·bcrypt·argon2 아님) vs
   게이트웨이 scrypt N=32768 (`security.mjs:17`). 세션 체계도 분리. → 통합·교체 없이 상용 불가.
3. **이중 API·우회 경로**: v1/v2 버전 불일치 + 모바일 직결 + Pages 미연결(`wrangler.toml:8` 주석).
4. **보안헤더·제한 미통합**: 게이트웨이 CSP·HSTS·X-Frame 없음(`server.mjs:134`), Go 전역 제한 없음,
   게이트웨이 300/분 vs nginx 100r/s 정책 분산.
5. **가입·메일 P0 BLOCKED**: 운영메일 어댑터·도메인·DNS·평판 없음 (`GAP_REGISTER.md:20`).
6. **크롤링 가동-판정 분리**: 코드는 실파이프라인이나 법적 승인 전 임의 크롤링 비활성이 게이트 (`GAP_REGISTER.md:47`).
7. **외부 차단 4종**: 언론사·지도 계약, LLM SLA·법무면책, 금융·보안감사+이중승인, SOC2/외부 모의해킹
   (`IMPLEMENTATION_STATUS.md:97-107`) — 코드로 해결 불가.
8. **인프라 미검증**: Compose E2E·race·원격 CI·독립감사 미실행, OpenSearch 보안플러그인 비활성,
   Grafana 기본 비번, 게이트웨이 Prometheus 잡 없음.
9. **콘솔·결제 부재**: 어드민·파트너 미구현, 실결제 0건.
10. **법무·정책 초안**: 약관 초안 배너, 500 페이지 없음, 온보딩 없음.

## 6. 디자인 전수 감사

### 6.1 인벤토리

- 페이지: 홈(히어로+검색+최근검색+서비스카드 3종), SERP, 메일 3열, 인증 5종(login/signup/forgot/reset/verify),
  계정, 설정, 정책 4종(about/how/privacy/terms), 404. 총 13 라우트 + SignupWizard 7단계.
- 컴포넌트: App, SearchBox, BrandMark, ScanArt, ScanState, MailView, SignupWizard.
- 토큰(`portal.css:9-42`): brand-blue `#2563eb`·hover `#1d4ed8`·cyan `#22c7e8`·violet `#7c3aed`·navy `#0b1020`,
  라이트 팔레트(bg `#f7f9fc`·surface·ink `#0f172a`·muted·line), 상태색 3종+배경, 그림자 3단계, radius 16px,
  시스템 폰트+Noto Sans KR/Thai, 버튼 최소높이 44px·radius 10px, notice/alert/success 박스.
- `theme-color #2563EB` 일치 (`index.html:9`).

### 6.2 결함 목록

- **이중 토큰**: 미로드 `style.css`에 Google식 `#1a73e8/#1a0dab`·14px/4px 폼 정의가 남아 혼란 (삭제 대상).
- **UX 데드엔드**: store/advertising/business 타이틀만 있고 라우트 없음 → 404 폴백. SERP 버티컬 탭 단일(웹) 고정.
- **법무 차단**: 정책 초안 배너 상시 노출.
- **미확인**: `portal.css:151` 이후(다크테마·반응형·모달/카드), 보조 컴포넌트 원문 — 반응형 깨짐 확정 불가(§9).
- **양호**: 스켈레톤·에러 5종·빈/초과 상태, skip-link·focus-visible·aria, 메일 sandbox 격리, 더미 텍스트 0건.

### 6.3 결론: 전면 재설계 불필요, 시스템화 리빌드가 정답

유지 가치 있음: 단일 토큰 체계·상태/접근성 처리·실API 연동이 이미 상용 뼈대를 갖췄고 더미가 없다.
단 (a) 브랜드 혼재 해소, (b) 데드코드·데드엔드 제거, (c) 다크테마·반응형·모달/카드 시스템화,
(d) 인증·정책·온보딩·500 추가로 **새 디자인 시스템 위에 재구축**해야 한다. 답습이 아닌 통일이다.

## 7. PHASE 2 체크리스트 매핑 (현 상태)

| # | 항목 | 상태 |
|---|---|---|
| 1 Core | 검색 실동작, AI·vertical·superplatform 스텁 | 부분 |
| 2 Auth·Security | 이중 인증, 자작 해시, 전역 제한·헤더 미통합 | 미달 |
| 3 Data | 마이그레이션·원장·검증 실구현, 원장 영속 없음 | 부분 |
| 4 API·Backend | 에러·페이지네이션·검색/필터 실구현, 이중 계층 | 부분 |
| 5 Frontend | 상태·접근성·실연동 양호, 반응형 미확인, 더미 0건 | 부분 |
| 6 Payment | 스키마·인메모리 원장만, 실결제 0건 | 미구현 |
| 7 Admin | 의도적 미구현 | 미구현 |
| 8 Infra·Ops | Compose·CI·테스트 존재, E2E·race·감사 미검증 | 미달 |
| 9 Legal·UX | 초안 배너, 500·온보딩 없음 | 미달 |
| 10 Design | 토큰·상태 완비, 이중 토큰·데드엔드·브랜드 혼재 | 부분 |

베이스라인(2026-11-07): `npm run build --prefix apps/web` 통과(5.0s, 226KB JS),
`go build ./...` 통과(exit 0).

## 8. 다음 단계 (코드 가능 범위, 의존순)

- L0 계약: 브랜드 단일화(OpenScanner vs OpenPortal) + API 버전 단일화 + 인증 단일화 결정.
- L1 보안: 자작 해시→표준 KDF 교체(마이그레이션 포함), 보안헤더·RateLimit 통합.
- L2 데드코드·데드엔드 제거: `style.css`·`api.ts`, titles 미연결 3종 처리, 500·온보딩 추가.
- L3 디자인 시스템 재구축: 토큰 확정→다크/반응형→컴포넌트→전 페이지 적용.
- L4 콘솔·결제: 어드민·파트너 최소 콘솔, 결제 샌드박스 연동(외부 승인 병행).
- 외부 차단(코드 불가): 메일 도메인·언론/지도 계약·LLM SLA·금융감사·SOC2 — 승인 트랙 분리 필요.

## 9. 미확인 목록 (후속 검증 필요)

- Go 개별 핸들러 본문·`repository.go`·`postgres_frontier.go`·protobuf·dr/loadtest·보조 구현 미정독.
- `migrations 002~011` 본문, `translations.mjs`, proto·documents 매핑, 나머지 scripts, Pages Functions 미확인.
- `portal.css:151` 이후(다크·반응형·모달/카드), 보조 컴포넌트 원문, `i18n.ts` 후반 일부 미확인.
- `go test`·race·Docker·Flutter·Compose E2E 미실행. 등록 45 vs 차단 41 vs deny 40여 개수 미재계.
- 500 페이지 부재는 전수 검색 기반, 파일 부재 확정은 추가 확인 필요.
