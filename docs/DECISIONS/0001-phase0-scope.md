# ADR 0001 — Phase 0 vertical slice and verification

상태: 채택, 2026-10-20. 사용자 지시와 마스터 명세에 따라 기존 starter를 개선한다.
빈 작업 폴더에 ZIP을 outputs/openportal_search_codex로 추출하고 원본 import commit을 보존했다.
프레임워크 교체보다 실제 검색 경로의 실행 증거를 먼저 확보한다. Vue/Vite는 Phase 0용, Nuxt 이행은 011.
PostgreSQL은 control plane이며 Go 검색 API는 OpenSearch만 사용한다. 관계형 DB를 검색 원장처럼 연결하는 가짜 경로를 만들지 않는다.
Docker 부재로 PostgreSQL/OpenSearch native 실행을 대체 증거로 사용한다. Compose 실행 통과와 구별한다.
실제 인터넷 crawling은 Phase 2 접근정책/SSRF 게이트 전까지 비활성이다.
합성 fixture E2E는 자체 수집·독립 검색엔진 완성의 증거가 아니다.
프로덕션 서비스 버전 승인과 이미지 digest 고정은 별도 공급망 검토를 요구한다.
