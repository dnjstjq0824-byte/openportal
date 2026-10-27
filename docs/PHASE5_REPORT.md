# Phase 5 검증 보고서 — 2026-10-27

판정: **부분 구현 / NO-GO**. 단위 테스트 통과는 라이선스 승인이나 수직검색 서비스 E2E 완료를 뜻하지 않는다. 이 보고서는 이전의 기술 검증완료·공인 시드 데이터 주장을 정정한다.

## 소스 구현

[vertical 모듈](../backend/internal/vertical/)에는 이미지·영상·뉴스·장소·쇼핑 정책 및 메모리 기반 처리 코드가 있다. [파이프라인 테스트](../backend/internal/vertical/pipeline_test.go)는 합성 입력으로 함수를 연결하는 테스트이며 PostgreSQL → OpenSearch → API → 웹/모바일의 실제 E2E가 아니다. Tasks 051–059는 부분 구현, 060은 부분/차단이다. 실제 출처 계약, 영속화, 검색 연결, 삭제 전파 및 모바일 검증이 남아 있다.

## SQL 기준선과 변경

[001 기준선](../database/migrations/001_initial.sql)은 8개 스키마·86개 테이블이며 제품 완성 증거가 아니다. 수정 전 일회용 PostgreSQL에서 001–008 순차 실행을 확인했다. 007의 외래키 대상 portal.images, portal.places, portal.product_offers, identity.accounts는 001에서 먼저 생성된다.

[007 마이그레이션](../database/migrations/007_phase5_vertical_search_and_licensing.sql)의 예시 뉴스 매체 2건에 대한 licensed=true 자동 설정과 근거 없는 ODbL 장소 2건 시딩을 제거했다. 운영 테이블 3개와 비활성 기능 플래그 5개는 유지한다. 신규 DB에는 승인된 뉴스 출처나 장소를 자동 등록하지 않는다.

007은 미추적 작업 파일에서 수정했다. 이번 검증은 새 로컬 DB만 대상으로 했다. 이전 007을 적용한 기존 DB는 체크섬이 달라 자동 업그레이드되지 않으며, 별도 후속 마이그레이션 검토가 필요하다. 기존 DB의 라이선스나 데이터를 수정하지 않았다.

## 이번 실행 증거

- 환경: Windows, Node 24.19.0, PostgreSQL 17.10, 127.0.0.1:55432의 실행별 일회용 클러스터. 실행 후 서버 중지.
- `node scripts/test-database.mjs`: 수정 전 기존 검사 PASS. 신규 회귀 검사는 기존 SQL에서 FAIL(licensed 매체 2건, 기대값 0건).
- 같은 명령으로 수정 후 PASS: 기준선 86개 테이블/8개 스키마/75개 검증된 FK, 001–008 적용 및 재실행, 미검증 출처 미등록, Phase 5 플래그 비활성, 비인가 계정 접근 거부, 계정 간 RLS 격리, 실패 DDL/원장 롤백, 체크섬 변조 거부.
- 원시 로그: [수정 전 회귀 실패](../work/phase5-db-regression-before.log), [수정 후 통과](../work/phase5-db-regression-after.log). work는 Git 제외 로컬 증거이다.
- backend 디렉터리의 `go test ./...`: PASS(API 2.112s, 나머지 테스트 패키지는 캐시 포함). Go 소스는 이번 수정 범위에 포함하지 않았다.
- 이번 작업에서 브라우저 E2E, 모바일, 독립 법무/보안 검수, 운영 배포는 NOT RUN.

## 100개 작업 계획과 게이트

[100개 원본 작업 계획](PHASES_0_TO_9.md)의 번호·체크박스는 변경하지 않았다. [상태표](IMPLEMENTATION_STATUS.md)의 Phase 5 완료 과장을 부분 구현으로 정정했다. 실제 수직검색 E2E와 독립 권리 검증 전까지 NO-GO이며 관리자/파트너, 결제, 임의 도메인 수집을 활성화하지 않았다.

커밋 및 배포: 이번 작업에서 수행하지 않음. 기존 Phase 6 및 UI 변경은 보존했다.
