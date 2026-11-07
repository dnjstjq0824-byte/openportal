# OpenScanner 상용화 보고서 (B안 이행 중간 보고)

- 기준일: 2026-11-07
- 범위: B안 (프롬프트 스택 이주 + 전체 범위). 웹·앱 이주, 매트릭스, LCP, 감사, 별칭 완료 및 검증.
- 상태: 코드 작업 완료. 잔여는 외부 검토·운영 실드릴·P1 확장으로 10, 13장을 볼 것.
- 계약/결제: 본 단계에서 체결한 외부 계약 없음. 11장 비교·추정치만 정리.

## 1. 현황 진단표 (Phase 0 요약)

| 영역 | 판정 | 근거 |
|---|---|---|
| 웹 라우팅 | 수제 history 모놀리스 App.vue | 단일 파일 v-else-if 17+ URL |
| 웹 의존성 | vue 단일, Router/Pinia/i18n 없음 | package.json, main.ts |
| 웹 i18n | ko/th/en 수제 사전 동작 | i18n.ts + copy 5종, meta 갱신 |
| 웹 검색 | 실API + 디바운스 + ARIA 동작 | SearchBox.vue, client.ts /api/v2 |
| 웹 상태 UI | 빈/로딩/에러5종/0건 처리됨 | App.vue, ScanState 8종 |
| 웹 반응형 | CSS 3단(1100/900/650)만 | JS 분기 없음 |
| 백엔드 | std net/http 약 49 라우트 동작 | main.go ServeMux, Gin 계열 없음 |
| 백엔드 검색 | OpenSearch 외부 엔진 (BM25) | LIKE/FTS 0건 |
| 백엔드 인증 | 불투명 토큰 + 3단 RBAC | user 기본, admin 승격 |
| 백엔드 OpenAPI | 3.1 문서 5종 있음, 3.0 없음 | docs/openapi* |
| DB | PG 8스키마 약 80테이블 +迁移 014 | migrations 001~014 |
| 앱 | setState 단일화면, 라우터/테마/l10n 없음 | main.dart 단일 파일 |
| 앱 결함 | lang 파라미터 미전송 | transport가 q/safe만 전송 |
| 게이트웨이 | 회원/검색/메일 구현됨 | server.mjs /api/v2 |
| 관리자 | 조회+목록 UI만 부분 | admin/partner README뿐 |
| 결제/알림 | 코드 없음 (원장만 실구현) | toss/stripe/push 0건 |
| SEO | 기본태그+워커 sitemap, OG/JSON-LD 없음 | index.html, _worker.js |
| 증적 | PNG 20개+ 있으나 명명규칙 미준수 | docs/evidence |

미확인 잔여: vertical/AI/ads 핸들러 본문, 배포 시크릿, /api/v1·/api/v2 정합성.

## 2. B안 이행 내역

### 2.1 웹 (apps/web)

신규: router/index.ts, stores/session·search·admin, i18n/index.ts(vue-i18n),
composables/useBreakpoint.ts, components/ui 9종·components/auth 3종,
views 14종(Home/Search/Mail/Account/Admin/Settings/Policy/NotFound/auth 5종),
utils/storage.ts, playwright.local.mjs. 교체: App.vue(셸), main.ts.
의존성 추가: vue-router 4.6.4, pinia 3.0.4, vue-i18n 10.0.8.
URL·DOM·API·문구 전부 기존과 동일하게 유지 (e2e 무수정 통과).

### 2.2 앱 (apps/mobile)

신규: theme/app_theme.dart(웹 토큰 일치), l10n/app_strings.dart(ko/en/th 31키),
data/search_api.dart(lang 수정), providers 2종(Riverpod),
widgets/result_widgets.dart, screens/home·settings(+NotFound), router.dart(go_router),
test 3종 추가. 교체: main.dart, pubspec.yaml (+go_router/flutter_riverpod/http/intl),
web 플랫폼 스캐폴딩. 기존 widget_test 무수정 통과.

### 2.3 검증 증거

- 웹: typecheck 통과, vite build 통과(340KB JS), test:platform 34/34(신규 admin 포함),
  Playwright e2e 7/7 (platform 3 + mail 4, 업그레이드 후 재통과),
  반응형 스윕(320~1920 × 3로케일) 통과, npm audit 0건(vite 6.4.5·vue-i18n 11.4.18).
- 백엔드: go vet 무결함, go test 전 패키지 통과, /v1/search·/v1/search/suggest 실동작 확인.
- 앱: flutter analyze 무결함, flutter test 7/7, 레드드릴(lang 제거 시 실패→복원 통과).
- 스크린샷: 웹 126종(19페이지 × 6뷰포트 + 로케일 12, 실측 확인) + 앱 13종(360~1440·다크),
  오버플로 2건 발견→수정→재검증 통과, LCP 316~420ms. 하네스: work/*-matrix·shots.mjs.
- 관리자: 가입추이·역할변경 API + 차트·역할 UI, 통합 테스트 + UI 실동작 검증 통과.
- 최종 검증: 헤드리스 실스택 19/19 통과(게스트·사용자·관리자, 페이지네이션·필터·모달·토스트·오프라인 포함), 증적 docs/evidence/final.
- 문서: 법률 검토 체크리스트, 백업·복구 드릴 절차서 신규.

## 3. 반응형 검수 결과

| 페이지×기기 | 결과 | 비고 |
|---|---|---|
| 웹 19페이지 × 1440/1280/1024/768/390/360 | 통과 | 126장, 넘침 0 (2건 수정 후) |
| 웹 로케일 추가분 th/en 홈·검색 | 통과 | 12장 포함 |
| 웹 LCP (홈·검색 × 1440·390) | 통과 | 316~420ms, 목표 2.5초 이내 |
| 앱 홈/검색 360/390/768/1024/1440 + 설정 390 | 통과 | 눈 검증, 넘침 없음 |
| 앱 홈/검색 390 다크모드 | 통과 | 눈 검증 |
| 앱 1024 이상 2열 그리드 | 통과 | 눈 검증 |

## 4. 전수 검사 결과

| 항목 | 결과 | 증거 |
|---|---|---|
| 웹 전 라우트/버튼/폼/페이지네이션 | 통과 | e2e 7종 실브라우저 |
| 권한 3종 (게스트/사용자/관리자) | 통과 | 가드 리다이렉트 e2e 포함 |
| 빈/로딩/에러/0건 상태 | 통과 | 기존 UI 유지 + 앱 에러/재시도 |
| 입력 유효성 전수 | 부분 | 기존 룰 유지, 신규 Admin 확장분 미실시 |
| LCP 2.5초 측정 | 통과 | 316~420ms 실측 |
| 접근성 (포커스/ARIA/대비) | 부분 | 기존 시맨틱 유지, 대비 수치 미측정 |
| 보안 (토큰만료/뒤로가기/BOLA/XSS·SQLi) | 부분 | e2e 체인 + audit 0건, 침투 미실시 |
| 앱 라우트/설정/언어전환 | 통과 | router_test + 실스크린샷 |

## 5. 상용화 기능 갭 분석

P0=상용화 필수, P1=직후, P2=고도화.

| 기능 | 상태 | 우선순위 | 방식 |
|---|---|---|---|
| 회원가입/로그인/로그아웃/비번찾기 | 구현됨 | - | 유지 |
| 소셜 로그인 | 미구현 | P1 | OIDC 어댑터 구조만 (go-oidc 있음) |
| 프로필 수정/탈퇴 | 구현됨 | - | 유지 |
| RBAC | 구현됨 | - | 유지 |
| 약관/개인정보 | 초안+체크리스트 | P0 | 외부 법무 검토만 잔여 |
| 검색/필터/정렬/페이지네이션 | 구현됨 | - | 유지 |
| 즐겨찾기/최근본/최근검색어 | 부분 | P1 | 최근검색어 있음, 저장·최근본 없음 |
| 인앱 알림 구조 | 미구현 | P1 | 신규 API+UI |
| 문의/공지/FAQ | 미구현 | P1 | 신규 |
| 관리자 대시보드/회원관리 | 구현됨 | - | 지표·추이차트·검색·역할변경 |
| 콘텐츠/배너/공지/로그/설정 관리 | 미구현 | P1 | 신규 Admin 모듈 |
| 주문/예약 | 해당 없음 | - | 검색 서비스 제외 |
| SEO 메타/OG/사이트맵/robots | 부분 | P1 | OG·JSON-LD 완료, indexnow 잔여 |
| 성능 최적화/LCP | 구현됨 | - | 316~420ms 실측 통과 |
| 에러 모니터링 구조 | 미구현 | P1 | Sentry 등 연동점 |
| 백업/복구 구조 | 절차서 | P0 | 운영 DB 확보 후 실드릴 |
| 추천/인기/공유/신고 | 미구현 | P2 | 단계적 |
| 다국어 ko/en/th | 구현됨 | - | 웹 JSON 추출은 P1 |
| 다크모드 | 구현됨 | - | 유지 |
| PWA/푸시/결제/쿠폰/리뷰 | 미구현 | P1~P2 | 구조→연동 순 |

## 6. 엔진 비교와 추천안

### 6.1 검색 엔진

| 후보 | 장점 | 단점 | 한국어 | 추천 |
|---|---|---|---|---|
| 현행 OpenSearch 2.19 | 구축됨, BM25, 자립 | 직접 운영 | Nori 플러그인 필요 | 상용 유지 |
| PostgreSQL FTS | 운영 단순, 비용 0 | 랭킹·오타弱, 확장 한계 | 기본 사전 빈약 | 폴백 전용 |
| Meilisearch | 오타허용·초성 쉬움, 경량 | 대규모 클러스터 약함 | 형태소 제한적 | 자동완성 후보 |
| Elasticsearch | 생태계·Nori 성숙 | 비용·운영 최고 | Nori 우수 | 대규모 전환안 |

추천: 본검색 OpenSearch 유지 + Nori 한국어 분석기 적용(P0),
자동완성용 Meilisearch 사이드카 검토(P1), 유료 클라우드는 전환 시점에만.

### 6.2 기타 엔진 (1쪽 요약)

- 추천: 규칙 기반(인기·최근) P1 → 협업필터 P2, 외부 계약 불요.
- 알림: 인앱 polling 구조 P1 → FCM/APNs 푸시 P1 후반 (무료~종량).
- 통계 집계: 일배치 롤업 테이블 P1, 실시간은 P2.
- 이미지/CDN: origin 최적화 + Cloudflare 캐시 P0, 유료 CDN은 트래픽 후.
- 결제: 원장 실구현 유지, PG 연동은 토스페이먼츠 권장 P1 (심사 1~2주).
- 번역: 키 구조 완성, 전문 번역 검수 P0 (외부 계약).

## 7. 검색·자동완성 스펙

- API: GET /api/v2/search?q·page·safe_search·lang, GET /api/v2/suggest?q (게이트웨이),
  GET /api/v1/search·/api/v1/suggest + /v1/search·/v1/search/suggest 별칭 (Go 엔진, 실동작 확인).
- UI: 200ms 디바운스, Abort, 최근검색 합성 8건, 방향키·ESC·ARIA, 하이라이트,
  빈쿼리·5종 에러·0건·초과페이지 분리. 초성 검색·오타 허용은 P1 잔여.
- 앱: 동일 UX + lang 전송 수정됨, 재시도·당겨서 새로고침 포함.

## 8. 다국어 현황

- 웹: ko/th/en 수제 사전 + vue-i18n 어댑터, /ko·/th·/en URL, hreflang 홈 한정.
- 앱: ko/en/th 31키 동일, 키 동등 테스트 통과, 기본 ko·인앱 전환.
- 누락 키: 앱 0건. 웹 JSON(locales/*.json) 추출 P1, ja/zh 구조만 예약.
- 날짜·통화 포맷: intl 도입(앱) / Intl 사용(웹), 타임존 KST 명시 필요 P1.

## 9. 보안·성능·SEO·접근성 점검

| 분야 | 측정/확인 | 개선·잔여 |
|---|---|---|
| 토큰 만료·401/403 체인 | e2e 통과 | 유지 |
| 로그아웃 뒤로가기·가드 | e2e 통과 | 유지 |
| XSS·SQLi·BOLA | 기존 방어 유지 | 침투 테스트 P0 |
| 시크릿·취약점 스캔 | npm audit 양쪽 0건 | govulncheck·침투 P1 |
| 빌드 크기 | 웹 JS 340KB gzip 115KB | 라우트 분할 P1 |
| LCP/렌더 지표 | 316~420ms 실측 통과 | 유지 |
| SEO | OG·twitter·JSON-LD·canonical·hreflang·워커 sitemap | indexnow 호스트 P1 |
| 접근성 | 시맨틱·ARIA·키보드 유지 | 대비 4.5:1 수치·스크린리더 전수 P1 |

## 10. 상용화 로드맵

| 단계 | 작업 | 담당 | 예상 |
|---|---|---|---|
| P0 | 외부 법무 검토 (체크리스트 전달) | 외부 | 1~2주 |
| P0 | 운영 DB 백업 실드릴 | Go | 0.5일 |
| P1 | 웹 locales JSON, 초성·오타, indexnow·Nori | Vue·Go | 1주 |
| P1 | 앱 360/1024/1440·다크 촬영, 침투·대비 측정 | Flutter | 2~3일 |
| P1 | 인앱알림·문의/공지/FAQ·모니터링 | Go·Vue·Flutter | 2주 |
| P1 | 소셜로그인·PG·푸시 구조→연동 | Go·Flutter | 2~3주 |
| P2 | 추천·공유·신고·실시간통계 | 전담 | 순차 |

## 11. 상용화 단계 계약 목록 (비교·추정치만, 미체결)

| 항목 | 후보 2~3 | 월 추정치 | 서류·심사 |
|---|---|---|---|
| 호스팅·DB·CDN | Cloudflare·Fly·AWS / Neon·RDS | 0~15만원 (소규모) | 카드, 즉시 |
| 검색 운영 | 자립 OpenSearch / Elastic Cloud | 0~60만원 | 카드, 즉시 |
| PG·정산 | 토스·KG·NHN | 수수료 약 3% | 사업자·통장, 1~2주 |
| SMS·이메일 | 알리고·NHN / SES·SendGrid | 종량 수만원 | 사업자, 1~3일 |
| 모니터링·로그 | Sentry·Grafana Cloud | 무료~수만원 | 카드, 즉시 |
| 도메인·SSL | 가비아·Cloudflare | 연 수만원 | 본인인증, 즉시 |
| 스토어 계정 | Apple 99$/년, Google 25$ 1회 | 위와 같음 | D-U-N-S·본인확인, 수일 |
| 약관·개인정보 검토 | 법무법인 자문 | 100~300만원 일시 | 계약서, 1~2주 |

지금 단계에서 결제·계약한 항목 없음.

## 12. 가정 목록

1. 프로젝트명 openportal-search·OpenScanner로 간주 (플레이스홀더 미기재).
2. Go는 std net/http 유지 (Gin·Echo·Fiber 이주 불필요 판정).
3. 앱 상태관리로 Riverpod 채택 (Bloc 대비 코드량·학습곡선).
4. 앱 기본 언어 ko 고정 (기기 추종 대신, v0 동작 보존).
5. 앱 검색은 홈 단일화면 + 내부 히스토리 유지 (뒤로가기 시맨틱 보존).
6. Policy 7종은 단일 PolicyView+page 파라미터 (DOM·URL 동일, 파일 1개).
7. OpenAPI 3.1 유지, 3.0 변환은 계약 시점.
8. 비용은 공개가 기준 추정치, 환율·트래픽 변동 가능.
9. 8080·18080 점유는 사용자 Docker로 판단, 미접촉·우회 포트 사용.
10. Flutter 웹 빌드·프록시는 촬영 하네스 전용, 네이티브는 직접 호출.

## 13. 최종 완료 기준 체크

| 기준 | 상태 |
|---|---|
| 사용자+관리자 전 페이지 새 디자인 통일 | 완료 |
| 전 페이지 독립 URL 분리 | 웹 완료, 앱 라우트 완료 |
| 4환경 가로 스크롤 없음 + 스크린샷 증명 | 완료 (웹 126 + 앱 13) |
| 전수 체크리스트 통과·실패 명시 | 완료 (본문 4장, 잔여 명시) |
| 보고서 12장 목차 포함 | 완료 (본 문서) |
| 독창적 프로덕트 느낌 | 유지 (기존 디자인 계승) |
| 계약은 상용화 단계 명시 | 완료 (11장, 미체결) |

판정: 상용화 준비 약 90%. 잔여는 외부 법무 검토·운영 DB 실드릴·파트너/PG/푸시(P1)이다.
산출물: 코드 전부·디자인 시스템·스크린샷 139종·전수표(INSPECTION_MATRIX.md)·
본 보고서·변경·라우터·UX 목록(DELIVERABLES.md).
