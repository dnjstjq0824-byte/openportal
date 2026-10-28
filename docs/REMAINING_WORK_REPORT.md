# OpenPortal Search — 잔여 개발 작업 보고서

생성일: 2026-10-28
기준: `docs/PHASES_0_TO_9.md` 100개 작업, `docs/IMPLEMENTATION_STATUS.md`, `docs/GAP_REGISTER.md`

## 1. 요약

- 전체 100개 작업 중 **검증완료(로컬 단위 범위)** 다수, **부분검증/차단** 항목 다수.
- Phase 0~9 게이트는 **NO-GO** (Docker/CGO race/원격 CI/Flutter/Nuxt 취약점/독립 감사/법무/결제 미승인).
- 최근 구현: OIDC 인증 경로 연결, moderation 익명 접근 차단, RBAC admin 인가, 프론트엔드 인증 UI, RRF 하이브리드 검색 통합, dev-mode 인증 E2E.

## 2. Phase별 미완료/차단 작업

### Phase 0 — 제품 설계·계약·실행 기반
- 005: PostgreSQL 86테이블 적용 검증 (검증완료, 운영 RLS는 Phase 2 이후)
- 008: 로컬 compose E2E (부분검증 — Docker 미설치, 네이티브 OS/Go/Vue로 대체)
- 009: 코드 품질/SAST/SBOM 파이프라인 (부분검증 — 원격 CodeQL/CI 미실행, CGO race 차단)
- 010: 위협모델·리스크 레지스터 (차단 — 독립 검토자 미지정·승인 미수령)

### Phase 1 — 검색 포털·회원·접근성
- 013: 웹/이미지/뉴스/동영상/지도/쇼핑 탭 기능 플래그 분리 (부분)
- 015: OIDC/Passkey 우선 인증·복구·MFA·세션 폐기 (부분 — WebAuthn 실서명 검증 미구현, dev-mode fixture만)
- 016: 역할·테넌트·객체별 권한 BOLA 회귀테스트 (부분 — 단위 테스트 수준, HTTP 레벨 회귀 보강 필요)
- 018: 파트너·관리자 별도 도메인·접근 경계 (부분 — deny-by-default 유지, admin/partner 미구현)
- 019: Flutter 모바일 앱 검색·결과·접근성 (차단 — Flutter SDK 미설치)
- 020: 웹/모바일 통합 CI·Lighthouse/접근성/i18n 게이트 (차단 — CI 미설치)

### Phase 2 — 웹 발견·크롤링
- 021~030: 검증된 사이트 등록·sitemap·frontier·robots·SSRF·politeness·WARC·noindex·삭제정책 (기술 검증완료, 법적 승인 전 임의 도메인 크롤링 비활성)
- 030: 10~100개 허가된 사이트 제한 수집·트래픽·robots·삭제 리포트 (차단 — 법적 승인/허가 도메인 없음)

### Phase 3 — 인덱싱·검색 핵심
- 031~040: HTML 추출·SimHash·Nori·Outbox·Blue-Green 재색인·tombstone·대시보드 (기술 검증완료, 독립 검토 서명 부재)
- 034: 문서 변경감지·index outbox·소비자 idempotency (부분 — 영속 DB 연동 미완)
- 038: 삭제/비공개/법적 제거 tombstone 전파 (구현, 캐시·스토리지 연계 검증 필요)

### Phase 4 — 랭킹 품질·안전 검색
- 041: 쿼리 의도 분류·언어·지역·교정 (부분 — opt-in 적용, 실제 인적 평가 데이터 없음)
- 042: 인기어/자동완성 조작·악용 탐지 (부분 — in-memory, 영속화 없음)
- 043: 사이트 품질·스팸·신선도 신호·재랭킹 (부분 — 상수 신호 사용, G019)
- 044: SafeSearch·유해콘텐츠 신고·이의 (부분 — 관리자 인가 미구현)
- 045~050: 한국어 품질평가·NDCG/MRR·RRF 하이브리드·부하 (G022 인적 판정 데이터 없음, 047 RRF 통합 진행 중)

### Phase 5 — 수직 검색
- 051~060: 이미지/동영상/뉴스/지도/쇼핑 vertical (부분 — 합성 입력 기반, 상용 계약 미체결)

### Phase 6 — AI 검색·지식 정보
- 061~070: AI 가드레일·RAG·인용·안전 fallback·비용 (070 법무/SLA 계약 미체결)

### Phase 7 — 광고·파트너·웹마스터
- 071~079: 웹마스터·캠페인·경매·부정클릭·원장 (080 제3자 금융·보안 감사 미수령)

### Phase 8 — 운영·관리자·개인정보·보안
- 081~089: JIT PAM·감사로그·Takedown·DSAR·k-익명성·OWASP·이중결재 (090 독립 보안검수 미수령)
- 083: 콘텐츠 신고·스팸판정·Takedown·재심 (부분 — 관리자 인가/RBAC 보강 필요)
- 084: DSAR 열람·정정·삭제·이동·동의철회 (부분 — 영속 저장소 연동 필요)

### Phase 9 — 성능·확장·재해복구·출시
- 091~100: 부하·용량·카오스·DR·SLO·WCAG·결함 제로·GO/NO-GO (099 외부 감사 미완)

## 3. GAP_REGISTER 열린/차단 항목

- G005 (P1): Docker Compose E2E 차단 → 네이티브 실행 증거로 대체
- G006 (P0): 인증/MFA/RLS/SSRF/삭제/정산 통제 — 부분 해소, 운영 노출 금지 유지
- G009 (P1): Flutter SDK 부재 → 코드 작성 완료, 테스트 차단
- G010 (P0): 독립 감사·소스 권리·상표·실결제 승인 없음 (사업 소유자 승인 필요)
- G011 (P2): 원격 CodeQL/CI 미실행
- G012 (P2): CGO race detector 미실행
- G016 (P1): Nuxt CVE로 Vite 유지
- G017 (P1): Flutter SDK 부재
- G019 (P2): 재랭킹 상수 신호 (색인 파이프라인에서 실제 신호 산출 필요)
- G020 (P1): 의도분류·실험·품질 게이트 API/DB 미연결, in-memory 상태
- G021 (P0): moderation 익명 접근 차단 완료, 관리자 인가 미구현으로 운영 노출 금지 유지
- G022 (P1): 인적 판정 데이터·시맨틱 후보 생성기 없음
- G023 (P3): 포트 401 간헐 재현 미규명

## 4. 외부 환경/승인 의존 차단

- Docker 미설치 → Compose full stack E2E 불가
- CGO/gcc 미설치 → `go test -race` 불가
- Flutter SDK 미설치 → 모바일 빌드/테스트 불가
- PostgreSQL/OpenSearch 런타임 미가동 → DB/색인 E2E 불가 (코드/마이그레이션은 작성됨)
- Nuxt 취약점 → Vue+Vite 유지
- 독립 감사자/법무/상표/결제/지도·뉴스 계약/배포 계정 미승인

## 5. 코드 레벨 추가 개발 필요

- WebAuthn passkey 실서명 검증 (라이브러리 추가 필요)
- 영속 저장소 (PostgreSQL repository) 연결 — identity, moderation, experiment, abuse
- 하이브리드 RRF 검색 경로 완전 통합 (다중 후보 생성 소스)
- 인덱스 문서에 신선도/도메인권위/스팸 신호 필드 산출·저장 (G019)
- 관리자 콘솔 UI + RBAC 완성 (현재 백엔드 엔드포인트만)
- 파트너/웹마스터 포털 UI + 테넌트 분리
- Flutter 모바일 앱 실제 빌드/테스트
- Nuxt 프레임워크 취약점 해소 후 승격
- CI/CD 원격 파이프라인 실행·CodeQL
- OpenSearch 인덱싱 파이프라인 E2E (네이티브)
- AI RAG 파이프라인 실제 모델 연동
- 광고 경매·정산 원장 영속화
- DSAR/Takedown 전체 downstream purge 검증

## 6. 권장 우선순위

1. RRF 하이브리드 검색 완전 통합 (다중 후보 생성)
2. 인덱스 신호 필드 산출 (G019)
3. 영속 저장소 연결 (identity/moderation)
4. 관리자 콘솔 UI
5. WebAuthn 라이브러리 추가 + 실서명 검증
6. Flutter SDK 설치 후 모바일 테스트
7. 독립 감사/법무/결제 승인 (외부)

## 7. 결론

"Google과 100% 동등"은 불가능하며, AGENTS.md/CODEX_MASTER_PROMPT에서도 금지합니다. 현재 구현은 로컬 단위 검증 수준이며, 상용화는 외부 승인·감사·인프라가 필요합니다. 위 잔여 작업을 순차적으로 진행해야 합니다.
