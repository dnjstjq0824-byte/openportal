# API and event contract / 007

GET /api/v1/search: q(1~200 Unicode 문자), page(1~100, 기본 1), lang(ko/en, 선택).
REST page size는 10이다. 결과 JSON은 docs/openapi.yaml을 따른다.
GET /api/v1/suggest: q 필수. GET /healthz는 생존, /readyz는 색인 검색 가능 여부다.
성공 200, 잘못된 입력 400, 잘못된 메서드 405, 검색 장애/부분 샤드 실패 503.
관리/파트너 endpoint는 미구현으로 404. 인증이 구현된 것과 다르다.
쓰기 API·webhook 없음: idempotency key/signature는 활성화 전 해당 Phase에서 정의·검증한다.
요청 ID, cursor pagination, quota/scopes는 014/037/079에 남아 있다.

Protobuf는 미래 서비스 계약 초안으로 gRPC 서버가 존재하지 않는다.
현재 REST와 미래 proto의 필드명/페이지 토큰은 동일하지 않으며 자동 매핑 완료로 표시하지 않는다.
buf lint/build/FILE breaking과 protoc-gen-go 컴파일은 계약 도구 검증이다.
docs/events/index-event.v1.schema.json은 Phase 3 outbox 계약이며 아직 producer/consumer가 없다.

호환성: CI PR base SHA와 buf breaking, OpenAPI/event 보수적 불변 검사를 실행한다.
기존 필드·endpoint 변경은 버전 올림이나 명시적 이행 검토가 필요하다. 신규 endpoint/스키마는 허용한다.
브라우저 E2E는 실제 REST 응답을 OpenAPI JSON Schema로 검증한다.
