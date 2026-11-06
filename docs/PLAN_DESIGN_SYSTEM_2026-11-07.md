# Delivery Plan — Design System Rebuild (2026-11-07)

## D-decisions

- **D1 (brand)**: 사용자 확정 — 브랜드는 `OpenScanner`로 통일. 문서의 OpenPortal 표기는 후속 레인에서 정리.
- **D2 (dead-end)**: `store/advertising/business`는 삭제가 아니라 3개 로케일 기존 카피(`*Body`)를 쓰는
  정보 페이지로 연결. `about/howsearchworks` 기존 패턴과 동일 계열.
- **D3 (scope)**: 미사용 i18n 키 대량 삭제는 비목표(메시지 카탈로그 선행 비축 관행 존중, 변경 위험 대비 이득 없음).
- **D4 (500)**: SPA 특성상 전용 500 라우트 대신 인증 영역의 서버장애 전용 섹션으로 수용.

## Slice A — foundation (this change)

| ID | Acceptance | Files |
|---|---|---|
| DS-A1 | `style.css`(101KB)·`api.ts` 삭제 후 typecheck·build 녹색, 참조 0건 | del 2, `tests/open_scanner_auth.spec.mjs` 주석 1줄 |
| DS-A2 | localStorage 키가 `openscanner.*`로 통일되고 구 `openportal.*` 값은 1회 마이그레이션됨 | `App.vue` |
| DS-A3 | `/{ko,th,en}/store·advertising·business`가 404가 아닌 정보 페이지로 렌더되고 푸터에서 진입 가능 | `App.vue`, `i18n.ts` 읽기만 |
| DS-A4 | `authSubmit`의 도달 불가 signup 분기·`agree` ref 제거 후 typecheck 녹색 | `App.vue` |
| DS-A5 | 인증 경로에서 5xx/게이트웨이 단절 시 전용 장애 섹션(재시도+홈+코드) 표시, 4xx는 기존 alert 유지 | `App.vue`, `i18n.ts` 키 3종×3로케일, `portal.css` 재사용 |
| DS-A6 | 미지정 라우트는 여전히 404 섹션으로 폴백 (회귀 없음) | — |

Negative: DS-A2 구키 사용자도 설정 유지. DS-A5 4xx 오탐 없음. DS-A3 미지원 로케일 prefix 동작 불변.

## Slice B — dialog system + onboarding + polish (this change)

| ID | Acceptance | Files |
|---|---|---|
| DS-B1 | `ConfirmDialog.vue` 신규: alertdialog·Esc·오버레이 취소·포커스 트랩·안전버튼 초기포커스 | new 1, `portal.css` 모달 스타일 |
| DS-B2 | App 내비게이션 가드(navigate/popstate/logout/expired)가 모달로 전환, `window.confirm` 0건 | `App.vue` |
| DS-B3 | 메일함 가드(compose/switch/read/close)가 모달로 전환, `window.confirm` 0건 | `useMailbox.ts`, `MailView.vue` |
| DS-B4 | 홈 첫방문 온보딩 배너(인라인·닫기·`openscanner.onboarded` 영속·3로케일, 기존 스펙 비차단) | `App.vue`, `i18n.ts`, `portal.css` |
| DS-B5 | 폴리시: SERP hit 호버, 서비스카드 화살표 모션(모션금지기 존중), `.alert-error` 데드클래스 제거, 📎→SVG | `portal.css`, `MailView.vue` |
| DS-B6 | 유지 테스트 `tests/design-system.e2e.spec.mjs` + `playwright.design.config.mjs` + `test:design:e2e`: 온보딩/정보페이지/404/500(정적) + 메일 파기 모달 취소·확정(스택) + 오버플로 스위프 | new 2, `package.json` |
| DS-B7 | `mail-reliability` 파기 테스트를 네이티브 dialog → 인앱 모달 단언으로 갱신(보존 단언 유지) | `tests/mail-reliability.e2e.spec.mjs` |
| DS-B8 | typecheck·build 녹색 + design 스펙 통과 + red drill | — |

D5: 온보딩은 오버레이가 아닌 인라인 배너(기존 스펙 차단 방지·저자극). D6: 기존 dialog 단언 변경은
의도된 UX 교체에 따른 기대치 갱신이며 파기-보존 검증 자체는 유지한다.
D7: 배너는 `home-grid` 첫 행(`grid-column: 1/-1`)에 둔다. `main.home-main` 직속 자식으로 두면
row-flex가 깨져 72px 압축·수평 오버플로·모바일 좌표 어긋남을 유발한다(프로브 실측).

## Evidence (Slice B, 2026-11-07 실행)

- `npm run typecheck --prefix apps/web` → exit 0. `npm run build --prefix apps/web` → exit 0
  (233KB JS, CSS 42.84KB).
- `npm run test:design:e2e` → 10 passed (5 tests × desktop/mobile, 2.2m).
  첫 실행 3건 실패→원인 확정(배너 row-flex·액션 오버플로·스택 타임아웃)→수정 후 전량 통과.
- `npm run test:mail:e2e` → 8 passed (4 tests × desktop/mobile, 1.6m), 파기-모달 마이그레이션 포함.
- grep 수령: `window.confirm|window.alert|alert-error` 0건.
- Red drill: `emit('cancelled')` 오타 → vue-tsc overload error exit 2 → 복원 후 SHA-256
  `F4EFC557…F1F88` 사전 해시와 일치 → typecheck exit 0.
- 미실행: platform/foundation e2e — 8080이 타 프로젝트(`project/op.gg/bin/api.exe`, 미접촉) 점유 중.
  OpenSearch는 기동 확인(curl 200, PID 56532 실행 유지). 8080 정리 후 `npm run test:platform:e2e` 실행 가능.

## Slice SEC — auth/security unification (this change)

| ID | Acceptance | Files |
|---|---|---|
| SEC-1 | 게이트웨이 전역 보안헤더(CSP·XFO·Permissions-Policy·CORP·HSTS-prod) + 통합테스트 단언 | `gateway/server.mjs`, `tests/platform.integration.test.mjs` |
| SEC-2 | Go 자작 해시→bcrypt 교체 + 레거시 검증·로그인시 마이그레이션 + 단위테스트 | `backend/internal/identity/identity.go`, `*_test.go`, `go.mod` |
| SEC-3 | Go 운영 라우트 IP별 제한(분당 120, healthz 면제, 429+Retry-After) + 테스트 | `backend/cmd/api/ratelimit.go` new, `production_test.go` 확장 |

D8: 브라우저 정식 인증은 게이트웨이 scrypt가 정식. Go Store는 프로토타입이나 잠금해제 대비 KDF를
표준으로 교체한다. D9: `.env.uild.txt`의 빌드 오류는 stale(현재 빌드 녹색). 미추적 파일이라 삭제하지
않고 사용자에게 정리 요청한다. D10: Go `clientIP`는 기존 구현(main.go:1225)을 재사용한다.

## Evidence (Slice SEC, 2026-11-07 실행)

- `go vet ./...` → exit 0. `go test ./...` → 전 패키지 ok (신규 KDF·레이트리밋 테스트 포함).
- `node --test tests/platform.integration.test.mjs` → 20 pass (신규 보안헤더 테스트 포함).
- Red drill: `bcryptX$` 오타 → KDF 테스트 2건 FAIL → 복원 후 SHA-256 `65272B4C…1DD75`
  사전 해시와 일치 → identity ok.
- gofmt 잔류 6건은 베이스 대조로 전부 기존 것임을 확정(내 코드는 clean).
- 미실행 유지: platform/foundation e2e — 8080이 타 프로젝트 점유 중(미접촉).
  → 해소: SEARCH_API_PORT/SEARCH_BACKEND_URL env 오버라이드로 8081 사용, 6 passed(exit 0).
  dev 스택: `node scripts/dev-stack.mjs`(pg 55440+gateway 8090) + Go :8081 + vite :5173.

## Slice ADMIN — minimal operator console (this change)

| ID | Acceptance | Files |
|---|---|---|
| AD-1 | 마이그레이션 014 `is_admin` + `grant-admin` CLI | `database/migrations/014_*`, `scripts/grant-admin.mjs` |
| AD-2 | `GET /api/v2/admin/overview`·`/accounts`(검색+페이지, 해시 제외) + 401/403 + 통합테스트 | `gateway/server.mjs`, `tests/platform.integration.test.mjs` |
| AD-3 | 웹 `/admin` 뷰(개요 카드+계정 표+검색, 비관리자 접근 차단) + 3로케일 + e2e | `App.vue`, `i18n.ts`, `portal.css`, `tests/design-system.e2e.spec.mjs` |

D11: admin은 동일 오리진 역할 게이트로 시작(분리 오리진·MFA는 후속). 목록에 자격증명 일체 제외.

## Evidence (Slice ADMIN + local stack, 2026-11-07 실행)

- 포트 재배선: Go 8081(타 프로젝트 8080 점유 회피), gateway 8090, vite 5173, dev pg 55440.
  platform config/spec에 SEARCH_API_PORT/SEARCH_BACKEND_URL 오버라이드 + 타임아웃 상향.
- `npm run test:platform:e2e` → 6 passed (exit 0). 차단 게이트 해소.
- `scripts/dev-stack.mjs` 신규(영속 embedded pg + 시뮬레이터 메일 게이트웨이) + `dev:stack` 스크립트.
  documents_v2 시드 완료. 전 구간 스모크(검색 8 hits·세션 csrf) 통과.
- AD-1..3: 마이그레이션 014 + grant-admin CLI(연결·미존재계정 검증) + gateway admin 2종 +
  웹 /admin 뷰 + 통합 21 pass + design e2e 12 pass.
- Red drill: admin 게이트 무력화 → 통합 테스트 200!==403 FAIL → 복원 후 SHA-256
  `381D62AD…08667E` 일치 → 21 pass.

## Slice UNI — API unification (this change)

| ID | Acceptance | Files |
|---|---|---|
| UNI-1 | 모바일이 게이트웨이 /api/v2로 직결(base :8090, did_you_mean+fallback, OpenScanner 명칭·패키지) + 위젯테스트 + `flutter test` | `apps/mobile/**` |
| UNI-2 | API_CONTRACT §1을 2계층 모델로 재작성(클라이언트 v2 표 + 엔진 v1 내부 명기) | `docs/API_CONTRACT.md` |
| UNI-3 | `docs/openapi/gateway.v2.yaml` 신규 + 계약 게이트 등록 + `test:contracts` | `docs/openapi/*`, `scripts/check-contracts.mjs` |

D12: 정식 클라이언트 표면은 게이트웨이 /api/v2 단일. Go /api/v1은 내부 엔진(직접 호출 금지).
D13: suggest는 이미 게이트웨이 프록시로 통합되어 있어 제외(코드 확인).

## Evidence (Slice UNI, 2026-11-07 실행)

- `flutter test` → All tests passed (did_you_mean 정식 + corrected_query 폴백 케이스).
  Flutter 3.47 삭제 파라미터(tooltip)도 정식 마이그레이션으로 수정.
- `npm run test:contracts` → PASS (gateway.v2.yaml 등록 포함).
- Red drill: `did_you_meant` 오타 → 위젯 테스트 FAIL → 복원 후 SHA-256
  `A91145F7…4EDE7A` 일치 → All tests passed.

## Slice REDESIGN — minimal visual system (this change)

- `portal.css` 전면 재작성(단일 액센트·헤어라인·무그라데이션, 44.3KB→31.2KB).
  선택자 커버리지 대조: 기존 228개 중 `.serp-header` 1개만 누락→추가, 나머지 전원 유지.
- BrandMark 단색화. 마크업 변경 없음(이름·클래스·카피 전부 유지).
- 스크린샷 검수 7종(홈·SERP·로그인·스토어·404·모바일·다크) 픽셀 확인 후 3건 수정
  (검색창 구조·인증 체크·헤더 320px 오버플로).
- `test:design:e2e` → 12 passed. typecheck·build 녹색.

## Evidence (Slice A, 2026-11-07 실행)

- `npm run typecheck --prefix apps/web` → exit 0. `npm run build --prefix apps/web` → exit 0
  (228KB JS, CSS 41.28KB 불변 — 신규 CSS 없음).
- grep 수령: `style.css`는 deprecated spec 주석 1건만(갱신됨). `openportal.`은 `migrateStorage` 매핑 3건만.
  App.vue 내 `agree|sentTo` 0건.
- Red drill: `policyBodyXXX` 오타 → vue-tsc TS2339 exit 2 확인 → 복원 후 SHA-256
  `FF28CED1…874963` 사전 해시와 일치 → typecheck exit 0.
- DS-A1..A6 전부 충족. 단, 브라우저 실렌더(신규 3페이지·500 섹션·마이그레이션 동작)는 미실행 —
  Slice B에서 Playwright로 검증 예정.
- 주의: 작업 시작 전부터 dirty였던 파일(본 건과 무관, 미접촉):
  `.env.example`, `docs/evidence/*`, `playwright.config.mjs`, `scripts/test-database.mjs`,
  `scripts/verify-acceptance.mjs`, `tests/platform.e2e.spec.mjs`, `tests/recovery.spec.mjs`,
  `tests/responsive.spec.mjs`, `tests/search.spec.mjs`.
