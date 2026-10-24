# API and event contract / 007 & 014

## 1. REST Gateway Endpoints

- `GET /api/v1/search`: `q`(1~200 Unicode 문자, NFC/공백 정규화 적용), `page`(1~100, 기본 1), `lang`(`ko`/`en`, 선택), `safe_search`(`strict`/`moderate`/`off`, 선택), `vertical`(`web`/`image`/`news`/`video`/`map`/`shopping`, 기본 `web`).
  - REST page size는 10이다. 결과 JSON은 `docs/openapi.yaml` 및 도메인별 명세 `docs/openapi/search.v1.yaml`을 따른다.
  - 모든 요청에 `X-Request-Id` 및 `X-Correlation-Id`를 부여/전파하며, 오탈자/한영타 변환 시 `corrected_query`를 함께 반환한다.
  - 비활성 수직검색(`image`, `news`, `video`, `map`, `shopping`) 요청 시 `403 Forbidden`(`VERTICAL_DISABLED`)을 반환한다.
- `GET /api/v1/suggest`: `q` 필수. 자동완성 목록(`suggestions`)과 교정 제안(`corrected_query`)을 반환한다.
- `GET /api/v1/portal/config`: 기능 플래그 기반 수직검색 탭 목록(`tabs`)과 콘솔 격리 상태를 반환한다(`docs/openapi/portal.v1.yaml`).
- `POST /api/v1/auth/challenge`, `POST /api/v1/auth/verify`, `GET /api/v1/auth/session`, `POST /api/v1/auth/revoke`, `POST /api/v1/auth/mfa/enroll`, `POST /api/v1/auth/mfa/verify`, `POST /api/v1/auth/recover`: OIDC/Passkey 우선 인증, TOTP MFA, 복구 코드 및 세션 즉시 폐기(`docs/openapi/identity.v1.yaml`).
- `GET|PUT /api/v1/accounts/{accountId}/preferences`, `GET|DELETE /api/v1/accounts/{accountId}/recent-searches`, `DELETE /api/v1/accounts/{accountId}/sessions/{sessionId}`, `GET /api/v1/organizations/{orgId}/sites/{siteId}`: 계정·역할·테넌트·객체별 소유권 검증(BOLA 기본 거부 `403`).
- `GET /healthz`는 프로세스 생존, `GET /readyz`는 색인 검색 가능 여부다.
- 성공 `200`, 잘못된 입력 `400`, 미인증 `401`, 권한/기능플래그/콘솔경계 거부 `403`, 미존재/미구현 관리자 경로 `404`, 잘못된 메서드 `405`, 검색 장애/부분 샤드 실패 `503`.
- 별도 호스트(`admin.*`, `partner.*`)로 인입되는 요청은 게이트웨이 미들웨어에서 즉시 `403 CONSOLE_BOUNDARY_DENIED`로 차단한다(`018`).

## 2. Protobuf ↔ REST 브리지 및 이벤트 계약

- `backend/internal/search/proto_bridge.go`는 REST 검색 요청/응답과 `openportal.search.v1`(`SearchRequest`, `SearchResponse`, `SuggestResponse`) 간 양방향 변환과 `Sponsored=false` 유기적 결과 격리를 수행하며 `proto_bridge_test.go`로 검증된다.
- `buf lint/build/FILE breaking`과 `protoc-gen-go` 컴파일은 `scripts/test-proto.mjs`로 자동 검증한다.
- `docs/events/index-event.v1.schema.json`은 Phase 3 outbox 계약이며 `scripts/check-contracts.mjs`로 유효/무효 fixture 및 불변성을 검증한다.
- 브라우저 E2E(`tests/search.spec.mjs`)는 실제 REST 응답을 OpenAPI JSON Schema로 검증한다.
