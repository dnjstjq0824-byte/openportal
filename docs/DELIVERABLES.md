# 산출물 목록: 변경 파일 + 라우터 URL + UX 의도

## 1. 수정 파일 (14)

- apps/mobile/lib/main.dart — ProviderScope + MaterialApp.router 진입점 교체
- apps/mobile/pubspec.yaml·pubspec.lock — go_router·Riverpod·http·intl 추가
- apps/web/index.html — OG·트위터 메타 추가
- apps/web/package.json·package-lock.json — Router·Pinia·vue-i18n 추가, 감사 수정
- apps/web/src/App.vue — 50KB 모놀리스 → 셸(헤더·푸터·RouterView)로 교체
- apps/web/src/i18n.ts — 관리자 키 4종·에러 1종 × 3언어 추가
- apps/web/src/main.ts — Pinia·Router·vue-i18n 장착
- apps/web/src/portal.css — 차트·900px 검색헤더 2단 추가
- backend/cmd/api/main.go — /v1/search·/v1/search/suggest 별칭 + 허용목록
- gateway/server.mjs — admin 가입추이·역할변경 API 추가
- package.json — test:platform에 admin-crud 편입
- tests/helpers/platform-stack.mjs — PG_TEST_PORT 환경변수 지원

## 2. 신규 파일 (요약 60+)

- 웹: router·stores 3종·i18n 어댑터·useBreakpoint·storage·ui 9종·auth 3종·views 14종
- 앱: theme·l10n·data·providers 2종·router·screens 2종·widgets·test 3종·web 스캐폴딩
- 문서: COMMERCIALIZATION_REPORT·INSPECTION_MATRIX·LEGAL_REVIEW_CHECKLIST·BACKUP_RESTORE_DRILL
- 증적: docs/evidence/web 126 PNG + matrix·metrics·fix JSON, docs/evidence/mobile 13 PNG
- 기타: playwright.local.mjs, tests/admin-crud.test.mjs

## 3. 웹 라우터 URL + UX 의도 1줄

| URL (:{locale}=ko·th·en) | UX 의도 |
|---|---|
| /:locale | 첫인상: 슬로건·검색·서비스 안내를 한 화면에 |
| /:locale/search?q·page·safe_search | 답 그 자체: 결과·교정·페이지로 탐색 완결 |
| /:locale/login | 복귀: 30초 로그인 후 원래 페이지로 |
| /:locale/signup | 온보딩: 단계 마법사로 계정+메일함 개설 |
| /:locale/forgot-password | 구제: 복구 메일 발송을 한 번에 |
| /:locale/reset-password | 재설정: 토큰 링크로 안전하게 비번 교체 |
| /:locale/verify-email | 확인: 메일 인증을 한 번의 클릭으로 |
| /:locale/mail | 작업: 폴더·목록·리더 3단 메일함 |
| /:locale/account | 내 정보: 표시명·복구·탈퇴를 한곳에 |
| /:locale/admin | 통제: 지표·추이·회원·역할을 한눈에 |
| /:locale/settings | 취향: 언어·테마·세이프서치·기록 설정 |
| /:locale/about·store·advertising·business·howsearchworks·privacy·terms | 신뢰: 서비스·정책 정보를 얇고 명확하게 |
| / · /register · /:locale/register | 유도: 로케일 감지로 올바른 첫 페이지로 |

## 4. 앱 라우트 + UX 의도 1줄

| 라우트 | UX 의도 |
|---|---|
| / | 검색이 곧 홈: 입력·결과·히스토리를 한 화면에 |
| /settings | 손안의 설정: 언어·세이프서치·기록을 큰 터치로 |
| 그 외 | 길잃음 방지: 찾을 수 없음 화면에서 홈으로 복귀 |

## 5. API 라우터 (신규·변경분)

| 메서드·경로 | 의도 |
|---|---|
| GET /v1/search · /v1/search/suggest | 공개 자동완성 계약 별칭 |
| GET /api/v2/admin/signups?days | 관리자 가입 추이 차트 데이터 |
| PATCH /api/v2/admin/accounts/{email}/role | 관리자 역할 지정·해제 (자기강등 금지) |
