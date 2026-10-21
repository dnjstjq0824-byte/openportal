# ADR 0002 — atomic migrations and pre-release contracts

상태: 채택. 001_initial.sql은 실제 PostgreSQL 적용 후 원본을 보존한다.
public.schema_migrations는 86개 도메인 테이블과 별개의 운영 이력이다.
세션 advisory lock으로 실행 직렬화, 각 파일 DDL+이력을 하나의 트랜잭션에 기록한다.
이력의 prefix·SHA256 검증, 중단 후 재실행, 이미 적용된 파일 변경 금지를 강제한다.
002에서 PUBLIC 접근을 철회한다. 테넌트 RLS/역할별 권한은 Phase 1 이후 구현 전까지 차단 상태다.
실패한 DDL은 rollback되며 적용된 데이터는 역방향 down으로 파괴하지 않는다.
복구는 검증된 백업을 새 클러스터에 restore→무결성 검증→트래픽 전환으로 진행한다. 운영 복구훈련은 094에서 별도 검증한다.

Protobuf는 아직 배포된 서비스가 없다. 초안의 ads/portal RPC 메시지 이름을 lint 규칙에 맞추고
go_package를 backend 모듈에 맞춘다. 기존 초안 대비 source-breaking change임을 명시한다.
이후 FILE breaking 검사와 PR base 기반 API/event 불변 검사를 적용한다.
생성된 Go 메시지는 계약 산출물이다. gRPC 서버와 index event producer는 구현되었다고 주장하지 않는다.
REST는 실제 API 계약이며 protobuf future SearchRequest와 자동 동등하다고 가정하지 않는다.
