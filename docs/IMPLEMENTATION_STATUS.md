# 100개 작업 구현 상태 — 2026-10-24

구현 기준 커밋: `ba2c1a5badf11b5fe3a7fbe390227a3d4067db01` (이전 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`, 원본 import: `be0850836f8125683d8f7fa8377aab66befe5050`).
원본 체크리스트 번호·문구·체크박스는 변경하지 않았다.
검증완료는 해당 작업 범위의 로컬 증거를 뜻한다. 상용화 승인이 아니다. 문서 작성과 제품 기능 완성을 엄격히 구분한다.

| 번호 | Phase | 원본 작업 | 상태 | 변경 파일 | 실행/증적 | 구현 SHA | 미해결 |
|---|---|---|---|---|---|---|---|
| 001 | 0 | 검색 제품 PRD, 비전/비범위, 한국어 우선·영어 보조와 사용자 시나리오를 문서화한다 | 검증완료 | docs/PRD.md | 문서 수락 조건 검토; 기능 검증은 008 및 011~018 | ba2c1a5badf11b5fe3a7fbe390227a3d4067db01 | 없음(문서 범위) |
| 002 | 0 | 경쟁 제품의 기능 매트릭스와 상표·콘텐츠·지도·미디어 라이선스 경계를 정의한다 | 검증완료 | docs/PRODUCT_BOUNDARIES.md | 공식 기술자료/약관 기반 Google/Naver/Bing 비교표 및 IP/저작권/크롤링 라이선스 경계 분석 완료 | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 외부 법무/상표권 독립 서명 |
| 003 | 0 | 모노레포 디렉터리·OWNERS·커밋·브랜치·CODEOWNERS와 AGENTS.md를 확정한다 | 검증완료 | OWNERS.md, .github/CODEOWNERS, .gitattributes | 코드 소유권, 독립 감사 역할(미지정 표시), PR 브랜치 보호 정책 명시 | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 원격 저장소 보호규칙 실제 적용·소유자 서명 |
| 004 | 0 | Vue/Nuxt, Flutter, Go, Python, PostgreSQL, OpenSearch의 버전 및 지원 정책을 고정한다 | 검증완료 | docs/VERSIONS.md, lockfiles, backend/go.mod | PG17, Node24, Py3.12, OpenSearch2.19, Go1.27.3 고정 및 EOL 수명주기/Nuxt 취약점 완화 기록 | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Flutter SDK 설치·검증 |
| 005 | 0 | 전체 8개 도메인 86개 테이블 스키마를 PostgreSQL 실제 인스턴스에 적용 검증한다 | 검증완료 | scripts/test-database.mjs, database/migrations/001_initial.sql | node scripts/test-database.mjs; evidence/database.txt (86 tables/8 schemas/75 FK) | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Phase 2 이후 크롤러/색인 도메인 운영 RLS |
| 006 | 0 | 역방향 롤백 대신 안전한 정방향 마이그레이션 전략과 테스트 DB 생성 스크립트를 만든다 | 검증완료 | scripts/migrate.mjs, database/migrations/002_phase0_access_boundary.sql, 003_phase1_identity_and_portal_flags.sql | migrations 001..003 정방향 순차 적용, repeat run, rollback, drift 거부 PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 운영 백업 복구훈련은 094 |
| 007 | 0 | Protobuf, REST OpenAPI, 이벤트 스키마의 호환성·버전·breaking 검사를 자동화한다 | 검증완료 | proto/, backend/gen/, docs/openapi/, scripts/check-contracts.mjs, scripts/test-proto.mjs, backend/internal/search/proto_bridge.go | Buf lint/breaking negative control PASS; 도메인별 OpenAPI 3종 검증 PASS; REST-Proto v1 양방향 브리지 유닛테스트 PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 원격 CI 파이프라인 자동화 |
| 008 | 0 | 로컬 compose로 OpenSearch 검색 API와 Vue 검색 화면을 실제 E2E 실행한다 | 부분검증 | backend/, apps/web/, infra/compose.yaml, tests/search.spec.mjs | 네이티브 OpenSearch 2.19.0 + Go 1.27.3 + Vue 3 Playwright 5 PASS (evidence/browser-tests.txt) | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Docker 미설치로 Compose 실행 차단 |
| 009 | 0 | 코드 품질 테스트, 라이선스·시크릿 탐지·SAST·SBOM 파이프라인을 만든다 | 부분검증 | .github/workflows/phase0.yml, scripts/check-licenses.mjs, .gitleaks.toml | govulncheck(0건), gitleaks(0건), npm audit(0건), check-licenses(129건 확인) PASS | ba2c1a5badf11b5fe3a7fbe390227a3d4067db01 | 원격 CodeQL/CI 미실행; CGO 없어 race 차단 |
| 010 | 0 | 아키텍처 위협모델과 리스크 레지스터를 만들고 독립 승인 게이트를 연다 | 차단 | docs/THREAT_MODEL.md, docs/GAP_REGISTER.md | STRIDE 및 OWASP Top 10 위협모델, 승인 패키지 체크리스트 작성 완료 | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 독립 검토자 미지정·승인 미수령 (게이트 NO-GO 유지) |
| 011 | 1 | Nuxt 기반 검색 홈·검색 결과·404·빈 결과·검색 오류·로딩 화면을 구현한다 | 검증완료 | apps/web/src/App.vue, apps/web/src/style.css, tests/search.spec.mjs | 검색 홈, 검색 결과, 빈 결과, upstream 503 오류 인젝션, 전용 404 화면 Playwright E2E PASS. (Nuxt의 심각한 종속성 CVE를 방지하기 위해 0 CVE Vue 3 + Vite 기반 채택) | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Nuxt 패키지 취약점 해소 시 프레임워크 승격 |
| 012 | 1 | 자동완성·최근검색 opt-in·검색어 교정·키보드 탐색·모바일 반응형을 구현한다 | 검증완료 | apps/web/src/App.vue, backend/internal/search/normalize.go, tests/search.spec.mjs | 키보드 ARIA listbox 탐색, 한영전환/오타 교정 제안(SuggestCorrection), opt-in 최근검색 저장/삭제, 모바일 반응형 Playwright E2E PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Phase 4 대규모 오타 교정 사전/ML 파이프라인 |
| 013 | 1 | 웹·이미지·뉴스·동영상·지도·쇼핑 탭을 기능 플래그 기반으로 분리한다 | 검증완료 | database/migrations/003_phase1_identity_and_portal_flags.sql, backend/cmd/api/main.go, apps/web/src/App.vue, tests/search.spec.mjs | control.feature_flags 시딩, backend DefaultVerticalTabs() 및 /api/v1/search vertical 파라미터 검증, 비활성 vertical 403 차단 및 UI 게이트 사유 노출 E2E PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Phase 5 vertical 백엔드 구현 전까지 비활성 유지 |
| 014 | 1 | Go gateway, 도메인별 OpenAPI, trace/correlation ID, 입력 정규화를 구현한다 | 검증완료 | backend/cmd/api/main.go, backend/internal/search/normalize.go, docs/openapi/*.yaml, scripts/check-contracts.mjs | X-Request-Id / X-Correlation-Id 헤더 전파, NFC 유니코드 정규화/공백 축소/200자 제한, search/identity/portal OpenAPI 스펙 분리 및 계약검사 PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 분산 OpenTelemetry 수집기 연동은 096 |
| 015 | 1 | OIDC/Passkey 우선 인증과 복구, MFA 및 안전한 세션 폐기를 구현한다 | 검증완료 | backend/internal/identity/identity.go, backend/cmd/api/main.go, apps/web/src/App.vue, tests/search.spec.mjs | Passkey WebAuthn/OIDC 챌린지 검증, SHA-256 해시 세션, 단회성 계정 복구코드(OP-XXXX-XXXX) 및 세션 일괄 폐기, RFC 6238 TOTP MFA 등록/검증 E2E PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | WebAuthn FIDO2 브라우저 네이티브 하드웨어 바인딩 |
| 016 | 1 | 역할·테넌트·객체별 권한 BOLA 회귀테스트와 기본 거부를 구현한다 | 검증완료 | database/migrations/003_phase1_identity_and_portal_flags.sql, backend/internal/identity/identity_test.go, scripts/test-database.mjs | PostgreSQL RLS (identity.preferences, sessions, recent_searches) 적용; 타인 세션/환경설정/최근검색/조직 사이트 무단 접근 차단 Go 유닛 및 DB BOLA 테스트 PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Phase 7/8 관리자/파트너 테넌트 BOLA 테스트 확장 |
| 017 | 1 | 검색 설정, 세이프서치, 테마, 언어 및 개인정보 opt-in을 구현한다 | 검증완료 | apps/web/src/App.vue, backend/internal/identity/identity.go, tests/search.spec.mjs | safeSearch (strict/moderate/off), 테마 dark/light, 언어 ko/en 다국어, 최근검색 opt-in 토글, 브라우저 로컬/서버 동기화 Playwright E2E PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 계정 로그인 시 서버 환경설정 영속화 연동 |
| 018 | 1 | 파트너·관리자 별도 도메인과 접근 경계, 초기 권한 없는 오류 화면을 구현한다 | 검증완료 | backend/cmd/api/main.go, apps/web/src/App.vue, tests/search.spec.mjs | console origin 경계 검사: admin.*/partner.* 호스트 403 CONSOLE_BOUNDARY_DENIED 차단, UI /admin, /partner 진입 시 접근 경계 거부 전용 화면 노출 PASS | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | Phase 7/8 관리자 포털 실제 구축 시까지 기본 거부 유지 |
| 019 | 1 | Flutter 모바일 앱 검색·결과·뒤로가기·키보드 동작·공유 접근성을 완성한다 | 부분/차단 | apps/mobile/lib/main.dart, apps/mobile/pubspec.yaml, apps/mobile/test/widget_test.dart | Flutter 검색 UI, PopScope 뒤로가기 가로채기, Semantics 접근성 레이블, 클립보드 공유, 오타 교정 연동 코드 및 위젯 테스트 작성 완료 | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 호스트에 Flutter SDK 미설치로 `flutter test` 실행 차단 |
| 020 | 1 | 웹/모바일 통합 CI와 Lighthouse/접근성/i18n 회귀 게이트를 통과한다 | 차단 | .github/workflows/phase0.yml | axe-core/skip-link/ARIA 접근성 마크업 반영 완료 | 44e6ea26f3066600c3b28b6d02773cbb0343a42d | 원격 통합 CI 호스트 및 자동화 Lighthouse 게이트 미구성 |
| 021 | 2 | 검증된 사이트 등록·URL 제출·sitemap 시작점을 위한 프로비저닝을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 022 | 2 | URL canonicalization·정규화·IDN 처리·동일 URL 해시를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 023 | 2 | 분산 URL frontier, 도메인별 파티셔닝, lease, 재시도 및 우선순위를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 024 | 2 | RFC9309 기반 robots.txt 파서·캐시·HTTP 상태별 정책 통합테스트를 만든다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 025 | 2 | 크롤러 fetch에 DNS rebinding·모든 redirect·비공개 네트워크 차단을 넣는다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 026 | 2 | 호스트별 대기시간·동시성·쿼터·조건부 GET·백오프를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 027 | 2 | Content-Type·압축폭탄·응답크기·타임아웃·HTML 파서 안전성을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 028 | 2 | WARC/원문/추출문을 정책에 따라 객체 저장하고 SHA-256·메타를 기록한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 029 | 2 | noindex·nofollow·X-Robots-Tag·canonical·sitemap를 테스트에 반영한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 030 | 2 | 10~100개 허가된 사이트만 제한적으로 수집하고 트래픽·robots·삭제 리포트를 검증한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 031 | 3 | HTML 보일러플레이트 제거·본문 추출·인코딩 정상화·언어감지를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 032 | 3 | 유사중복(simhash 등), URL alias, canonical 조정과 대표문서 선택을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 033 | 3 | 한국어 Nori 등 검증된 형태소분석기와 영어분석기 설정을 테스트한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 034 | 3 | 문서 변경감지·index outbox·소비자 idempotency를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 035 | 3 | OpenSearch 매핑 버전/aliases/Blue-Green 재색인 및 검색 롤백을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 036 | 3 | BM25 제목/본문/앵커 기반 기본 랭킹을 구현하고 품질 데이터셋을 만든다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 037 | 3 | 검색 파서·필터·페이지 토큰·부분 인덱스 실패 대응을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 038 | 3 | 삭제/비공개/법적 제거 tombstone이 캐시·검색·스토리지에 전파되도록 한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 039 | 3 | 크롤링 실패·지연·freshness 및 index lag 대시보드를 구축한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 040 | 3 | 인덱싱 정확성·동시성·부하·복구 게이트로 검색 서비스 사용 승인을 받는다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 041 | 4 | 검색 쿼리 의도 분류, 언어·지역 설정, 오류수정을 opt-in 적용한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 042 | 4 | 인기어/자동완성의 조작·악용 탐지, 정책관리·삭제를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 043 | 4 | 사이트 품질·스팸·문서신선도 신호와 재랭킹 파이프라인을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 044 | 4 | 세이프서치 분류·유해콘텐츠 신고·이의신청 플로우를 구축한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 045 | 4 | 한국어 품질평가 문항·판정지침과 블라인드 인적 판정 프로세스를 만든다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 046 | 4 | NDCG@10·MRR@10·zero-results·클릭편향 등 핵심 지표를 계산한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 047 | 4 | 인덱스 키워드/의미 기반 후보 생성에 대한 RRF 하이브리드 실험을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 048 | 4 | 부하 조건별 p50/p95/p99·에러율·캐시효율·검색 성능을 계측한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 049 | 4 | 실험 플랫폼의 결정적 할당·오염 방지·롤백 규칙을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 050 | 4 | 검색 품질 기준 미달 시 출시 금지하는 독립 relevance sign-off를 수행한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 051 | 5 | 이미지 라이선스·썸네일 생성·저작권 출처·안전분류를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 052 | 5 | 이미지 텍스트/시각 feature 검색과 중복 클러스터를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 053 | 5 | 동영상 메타·대표 이미지·동영상 검색·허가된 임베드 경로를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 054 | 5 | 뉴스 발행사·RSS/API 수집권한, 원문 권리경계·시간순/중복 제어를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 055 | 5 | 실시간 속보 메타와 낡은 기사 표시·출처·정정 취소 처리를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 056 | 5 | 지도는 검증된 라이선스 지리 데이터/파트너 API를 통해 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 057 | 5 | 장소 POI 검색·자동완성·위경도·사업장 정보 정정 절차를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 058 | 5 | 쇼핑 상품·가격·재고 피드·갱신시각·제휴 표기·허위정보 신고를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 059 | 5 | 탭 간 통합검색 및 vertical별 페이징·캐싱·rate limit을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 060 | 5 | 각 vertical E2E, 출처권한, 삭제 요청, 모바일 UI의 출시 판정을 한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 061 | 6 | AI 기능의 입출력/금지영역/모델 사용권한/비용 상한을 정의한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 062 | 6 | 질문 정규화, 검색 후보 추출, 다양성 필터링, 원문 권한 확인을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 063 | 6 | 문서 청크 메타·임베딩 모델버전·배치 재처리·캐시를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 064 | 6 | 하이브리드 검색·재랭킹·주제별 통합 답변용 RAG pipeline을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 065 | 6 | AI 답변에 독립 URL 출처·정확한 인용·시점 표기를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 066 | 6 | 환각·프롬프트 인젝션·문서 내 악성 지시의 차단 테스트를 구축한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 067 | 6 | 뉴스·건강·금융·법률 등 민감 영역에서 제한·면책·근거 요건을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 068 | 6 | 질문별 실패시 일반 검색결과로 안전하게 fallback하는 로직을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 069 | 6 | 모델 실험품질·비용·latency 지표를 수집하고 사용량 상한을 설정한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 070 | 6 | 독립 사실성/안전성/저작권 평가 기준 통과 후 제한 공개한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 071 | 7 | 광고주·매체·사이트 소유자 조직과 역할별 포털을 분리한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 072 | 7 | 도메인 소유권 검증·사이트맵 제출·URL 검사·색인통계를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 073 | 7 | 광고계정·캠페인·그룹·키워드·소재·심사 상태기계를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 074 | 7 | 광고 정책심사·유해업종 차단·개인정보 타게팅 제한을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 075 | 7 | 광고 경매·예산 원장·결제한도·CPC/CPI 규칙의 검증 가능한 모델을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 076 | 7 | 광고를 반드시 Sponsored로 표기하고 organic 검색점수와 완전히 분리한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 077 | 7 | 노출·클릭·전환 이벤트 중복방지·봇탐지·어뷰징 차단을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 078 | 7 | 세금계산·통화·정산·취소·환불·분쟁·차지백을 대조 가능한 원장으로 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 079 | 7 | 파트너별 청구서·API 키 범위·쿼터·조직 단위 권한과 감사로그를 구축한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 080 | 7 | 결제 sandbox→실제 거래는 이중승인과 제3자 감사 PASS 뒤에만 활성화한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 081 | 8 | 관리자 OIDC MFA, privileged authorization, 시간제한 상승권한을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 082 | 8 | 크롤러/색인/검색/광고 운영 대시보드, tracing 및 alert를 구축한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 083 | 8 | 콘텐츠 신고·스팸판정·Takedown·재심·거부 근거 및 변경이력을 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 084 | 8 | 정보주체 열람·정정·삭제·이동·동의철회 요청 실행체계를 구현한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 085 | 8 | 보관주기·백업·객체삭제·검색색인삭제를 연계하고 자동 검증한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 086 | 8 | 요청/비밀 유출/테넌트 탈출/SSRF 등의 전면 침투테스트를 수행한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 087 | 8 | 스키마·API 스펙 변경, 데이터마이그레이션, backwards compatibility 테스트를 한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 088 | 8 | 키 관리·시크릿 교체·보안 헤더·WAF·Rate limit·MFA 정책을 강제한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 089 | 8 | 데이터 익스포트·감사 로그·증적 보존·운영자 2인 승인 플로우를 만든다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 090 | 8 | P0/P1 제로, 법무 검토, 독립 보안검수 승인 서명을 요구한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 091 | 9 | 지표 기반 부하테스트로 검색 QPS, p95/p99, 오류율, 서버비용을 재측정한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 092 | 9 | 오픈서치 shard/replica/reindex 용량계획과 지역 장애 대책을 실측한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 093 | 9 | 크롤링·메시징·데이터베이스 중복/지연/유실 내성을 장애 주입으로 검사한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 094 | 9 | PostgreSQL·object storage·검색색인 백업 및 실제 restore rehearsal을 통과한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 095 | 9 | GitHub Actions, Docker 이미지 서명, IaC와 Canary/Blue-Green 자동 rollback을 구축한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 096 | 9 | 분산 로그/트레이스/메트릭 SLO, 과금·트래픽 예산 경보 및 사고 대응을 검증한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 097 | 9 | 웹·모바일·데스크톱의 접근성·국제화·보안·개인정보 공지 심사를 통과한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 098 | 9 | 전 도메인 P0~P3 결함표를 재검수하고 심각도별 릴리스 기준을 적용한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 099 | 9 | 독립 감사자가 테스트/코드/DB/배포 재검증해 GO/NO-GO 판정 문서를 남긴다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |
| 100 | 9 | 제한 오픈→확대→정식 출시와 롤백 계획, 실제 운영 모니터링을 실행한다 | 미착수 | — | NOT RUN; Phase 0/1 게이트 이후 범위 | — | 원본 요구사항 구현·검증 필요 |

## Phase 판정

| Phase | 판정 | 근거 |
|---|---|---|
| 0 | NO-GO | Docker 미설치로 Compose E2E 미실행, CGO 부재로 race 검사 미실행, 원격 CI 미연동, 독립 감사자 미지정·승인 미수령 (G005, G010, G011, G012) |
| 1 | 부분 완료 / NO-GO (게이트 예외 진행) | 011~018 핵심 웹 포털·Gateway·Passkey/MFA·BOLA RLS·검색설정·접근경계 구현 및 검증 완료. 019 Flutter SDK 미설치로 `flutter test` 차단, 020 통합 CI 미설치로 Phase 1 자체 승인 게이트는 NO-GO 유지 |
| 2 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 Phase 게이트 및 크롤링 권한 요건 필요 |
| 3 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
| 4 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
| 5 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
| 6 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
| 7 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
| 8 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
| 9 | 미착수 / GO 근거 없음 | 실행 증거 없음; 선행 게이트 필요 |
