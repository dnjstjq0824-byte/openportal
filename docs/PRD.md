# Phase 0 PRD — 001

한국어 우선·영어 보조 검색 포털을 구축한다. 사용자는 입력→자체 색인 검색→제목/URL/설명 확인→출처 방문을 수행한다.
Phase 0 고객은 개발자와 검수자이며, 합성 fixture라는 안내를 항상 제공한다.

## 이번 범위와 수락 기준

- Vue 3 입력/결과/로딩/빈 결과/오류, 키보드 Enter 검색.
- Go GET search/suggest, 입력 길이 200자·페이지 1~100·언어 ko/en 제한.
- 실제 OpenSearch BM25 제목 4, 설명 2, 본문 1 가중치. 이것은 품질이 검증된 자체 랭킹 모델이 아니다.
- PostgreSQL 86개 기준 테이블의 적용과 제약, atomic forward migration 재실행·변조 거부.
- 브라우저에서 한국어/영어 fixture, 빈 결과, 오류를 검증. 독립 크롤링 완료로 표현하지 않는다.

## 사용자 시나리오

1. 한국어 사용자가 `검색엔진` 입력 → 한국어 테스트 문서 표시 → 원문 URL 확인.
2. 개발자가 `OpenPortal` 입력 → 영문 테스트 문서 표시.
3. 결과 없는 쿼리 → 명확한 빈 결과; 엔진 장애 → 503과 오류 안내; 과도한 페이지 → 400.
4. 권한 없는 사용자의 admin/partner 접근 → 404; 기능을 가짜로 노출하지 않음.

## 제외 및 이후 단계

Phase 1 Nuxt SSR·실제 Flutter 앱·계정. Phase 2~3 합법적 crawler→index와 삭제.
Phase 4 한국어 형태소/품질·안전검색. Phase 5 라이선스 vertical.
Phase 6 AI, Phase 7 광고/정산, Phase 8 관리자/개인정보, Phase 9 운영/독립감사.
fixture의 standard tokenizer를 한국어 형태소 검색으로 주장하지 않는다.

## 개인정보와 지표

Phase 0 API는 검색어/계정/IP를 DB에 저장하지 않는다. 오류 로그는 upstream URL과 자격정보를 기록하지 않는다.
검색 p95 500ms·99.9% 가용성은 향후 가설이며 달성값이 아니다. 출시 기준은 원본 마스터 문서가 우선한다.
