# ADR 0004 — Google급 표준 문서화 및 제공자 전략 종합

- 상태: 채택 (Accepted)
- 일시: 2026-11-24
- 맥락:
  1. 2026-11-10 감사 시정(Phase 1~4, 커밋 `af27bad`)과 2026-11-11 제공자 검토(R-01~R-07, V-01~V-07) 이후,
     활성 문서 다수가 실측과 어긋났다 (Docker/Flutter "미설치" 차단 문구, vertical 403 표기, admin 구현 표기,
     존재하지 않는 파일 인용 CSV, glossy 완료 보고서 잔재).
  2. `docs/GAP_REGISTER.md`가 유일 상태 기준으로 선언되었으나, 목표 표준(Google급 품질 바)과
     제공자 결정 종합 문서가 없어 후속 착수가 보고서 3개를 오가며 판단해야 했다.
- 결정:
  1. `docs/GOOGLE_CLASS_SEARCH_STANDARD.md`를 목표 표준 + 현황 대조 단일 문서로 신설한다.
     "Google급"은 동등 주장이 아니라 공학적 품질 기준이며, 목표와 실측을 구분 표기한다.
  2. `docs/SEARCH_PROVIDER_STRATEGY.md`에 R/V 판정을 종합하고 2단계 로드맵(추상화+웹 축 → 글로벌 축+버티컬)을 확정한다.
  3. 과거 Phase 보고서·일일 스냅샷은 원문 보존하고, 정정이 필요하면 문서 상단 정정 헤더로만 처리한다
     (IMPLEMENTATION_STATUS·REMAINING_WORK 선례 준용).
  4. `docs/PHASES_0_TO_9.md`의 번호·체크박스는 변경하지 않는다 (마스터 지시 불변 조건).
  5. 활성 트리의 허위·과장 서술(미존재 파일 인용, 미측정 수치의 실측 표기)은 발견 즉시 정정하고
     GAP_REGISTER에 상태 기준을 남긴다.
- 결과:
  - README·ARCHITECTURE·API_CONTRACT·VERSIONS 현행화, PRODUCT_BOUNDARIES·QUALITY_EVALUATION_GUIDE_KO 확장,
    COMMERCIALIZATION_REPORT·EXHAUSTIVE_AUDIT·EXTERNAL_INTEGRATIONS 정정 헤더, CSV 2종 정정,
    GAP_REGISTER에 제공자 후속 항목(RV-01~) 등록.
