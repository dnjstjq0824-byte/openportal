# ADR 0003 — Phase 0 Re-evaluation and Phase 1 Architecture under Gate Exception

- **상태**: 채택 (Accepted)
- **일시**: 2026-10-24
- **맥락(Context)**:
  1. 인계 패키지(`OpenPortal_Search_Gemini_Handoff.zip`, 기준 HEAD `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`)의 SHA-256 무결성 검증(112개 파일 일치)을 완료하고 Git 이력을 복원했다.
  2. 현재 호스트(Windows 11 x64)에서 PostgreSQL 17.10, OpenSearch 2.19.0, Go 1.27.3(`GOTOOLCHAIN=auto`), Node 24.19.0, Python 3.12.14, Buf 1.74.0, govulncheck, gitleaks, Chrome/Playwright E2E를 모두 실제 재실행하여 통과를 확인했다.
  3. 그러나 로컬 호스트에 Docker Engine(`G005`), CGO C 컴파일러(`G012`), Flutter SDK(`G009`), 원격 GitHub 저장소(`G011`) 및 외부 법무·보안 독립 감사자(`G010`)가 없으므로 Phase 0 게이트는 **NO-GO**를 유지해야 한다.
  4. 사용자 인계 지시 제3절("Phase0를 한 번 재평가한 후 외부 승인만 기다리는 경우, 승인 의존 기능은 비활성으로 유지하고 독립적으로 진행 가능한 Phase1 011~020 개발을 이어가라. Phase0가 GO로 바뀌었다고 주장하지 말고 선행 게이트 예외와 의존성을 기록하라")에 따라 Phase 1(`011`~`020`) 구현에 착수한다.

## 결정(Decisions)

1. **선행 게이트 예외(Gate Exception) 및 금지 경계 유지**:
   - Phase 0는 `NO-GO`로 명시하며, 외부 승인 의존 기능(임의 인터넷 크롤링, 유료 광고 과금/실결제, 수직검색 외부 비인가 수집, 공개 배포)은 계속 비활성화한다.
2. **REST ↔ Protobuf v1 계약 브리지(`007`, `G014`)**:
   - 기존 OpenAPI 계약(`GET /api/v1/search`, `GET /api/v1/suggest`)의 필드·동작 하위 호환성을 100% 보존하면서, Go 백엔드 내부에 `searchv1.SearchRequest`/`SearchResponse` 및 `SuggestRequest`/`SuggestResponse` 양방향 변환·검증 패키지를 구현하고 단위 테스트로 검증한다.
3. **Go 게이트웨이, Trace/Correlation ID, 입력 정규화, 도메인별 OpenAPI(`014`)**:
   - 모든 요청에 `X-Request-Id` / `X-Correlation-Id`를 부여·전파하고 응답 헤더 및 구조화 JSON(옵션/신규 엔드포인트)에 반영한다.
   - 검색어 입력에 유니코드 NFC 정규화 및 공백 정규화를 적용하고, 검색어 교정(오탈자/한영타 변환 제안, `012`)과 기능 플래그 기반 수직검색 탭 상태(`013`)를 반환한다.
   - 도메인별 OpenAPI 명세(`docs/openapi/search.v1.yaml`, `docs/openapi/identity.v1.yaml`, `docs/openapi/portal.v1.yaml`)를 분리 작성하고 `docs/openapi.yaml`과 함께 계약 검사 스크립트로 자동 검증한다.
4. **인증·세션 폐기·MFA·BOLA 기본 거부(`015`, `016`, `017`, `018`)**:
   - OIDC/Passkey 우선 인증 구조(도전값 생성/검증 인터페이스 및 개발용 로컬 OIDC/Passkey 시뮬레이션이 아닌 표준 WebAuthn/OIDC 토큰 해시 세션 원장)와 계정 복구 코드, TOTP(RFC 6238 HMAC-SHA1) MFA 등록·검증, 세션 즉시 폐기(`revoked_at`)를 구현한다.
   - 조직·역할·객체별 소유권 검사 미들웨어를 구현하여 타 계정/타 조직 리소스 접근(BOLA)을 `403`/`404`로 차단하고 회귀 테스트로 입증한다.
   - 사용자 검색 설정(세이프서치 `strict`/`moderate`/`off`, 테마 `light`/`dark`/`system`, 언어 `ko`/`en`, 최근검색 및 개인화 명시적 `opt-in`)을 구현하며, opt-in이 꺼진 경우 최근검색어를 서버/클라이언트에 저장하지 않는다.
   - 파트너·관리자 경로 및 별도 호스트 접근 시 초기 권한 없는 사용자에게 명확한 접근 경계 오류(`403 Forbidden` 및 전용 접근 거부 안내 화면)를 반환한다.
5. **Nuxt 3 사용자 검색 포털 이행(`011`, `012`, `013`, `020`)**:
   - `apps/web`을 Nuxt 3(`3.17.5`) + Vue 3 + TypeScript 기반으로 구성하여 검색 홈(`/`), 검색 결과, 404 페이지, 빈 결과, 검색 오류(503 등), 로딩 상태, 자동완성·키보드 방향키 탐색, 최근검색 opt-in, 검색어 교정 제안, 기능 플래그 기반 탭(웹 활성, 이미지/뉴스/동영상/지도/쇼핑 플래그 제어), 설정 모달/화면, 파트너·관리자 권한 거부 안내 화면, 한국어/영어 i18n 및 웹 접근성(ARIA 랜드마크, 스킵 링크, 포커스 관리, 색상 대비)을 구현한다.
6. **Flutter 모바일 앱 고도화(`019`)**:
   - `apps/mobile`에 검색·결과·빈 결과·오류·뒤로가기(`PopScope`)·키보드 포커스/제출·결과 공유/복사·시맨틱 접근성(`Semantics`)·세이프서치/언어 설정 연동 및 위젯/네트워크 테스트 코드(`apps/mobile/test/widget_test.dart`)를 작성한다. 단, 호스트에 Flutter SDK가 없으므로 네이티브 빌드/테스트 실행은 `BLOCKED (NOT RUN)`로 정직하게 기록한다.

