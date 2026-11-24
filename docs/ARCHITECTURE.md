# Technical architecture (current + Google-class target)

최종 현행화: 2026-11-24. 상태 기준은 [GAP_REGISTER.md](GAP_REGISTER.md),
목표 표준은 [GOOGLE_CLASS_SEARCH_STANDARD.md](GOOGLE_CLASS_SEARCH_STANDARD.md)다.

## Product boundary

- OpenScanner(OpenPortal Search)는 주류 검색 패턴에서 영감을 받은 **독립 검색 포털**이다.
  Google의 상표·보호 비주얼·독점 백엔드 코드·독점 색인·랭킹 공식을 복제하지 않고,
  Google SERP를 무단 추출·재게시하지 않는다. Google과 1:1 동등함을 주장하지 않는다.
- 진짜 독립 검색은 권리 준수 발견 → 크롤링 → 추출 → 중복제거 → 색인 → 랭킹 → 서빙 경로를 요구한다.
  외부 결과만 반환하는 UI는 메타검색 프론트엔드로만 표기한다.
- 1차 시장: 한국어(`ko-KR`), 영어(`en-US`) 보조. 태국어(`th`) UI 지원, 형태소 품질은 별도 게이트.
- 개인화를 숨기지 않는다: 개인화는 동의·거부·삭제 절차를 요구한다.

## Runtime topology (current)

```text
[Vue SPA :5173 / Flutter] → [Node gateway :8090 /api/v2/*] → [Go engine :8080 /api/v1/*]
        (clients call /api/v2 only)      auth·account·search branch·mail·report
                                                                    → [OpenSearch :9200]
                                                                    → [PostgreSQL 8 schemas + webapp]
```

- 클라이언트 표면은 게이트웨이 `/api/v2/*` 단일이다. Go `/api/v1/*`은 내부 엔진 표면이며
  직접 호출을 금지한다. 운영 노출 allowlist는 읽기 경로로 제한한다.
- 검색 제공자: `SEARCH_PROVIDER` 1개 선택기 (`opensearch` 기본 · `local` · `brave` · `hybrid` ·
  `naver` · `mojeek`). `bing`은 퇴역 API로 부팅 거부, `serper`·`youcom`은 규격 미확정으로
  부팅 거부. 외부 제공자는 키 + 약관 승인 + 일일·초당 예산 적용, 예산 소진 시 자체 결과로
  degraded 폴백한다. `total: null`은 합집합 크기 미상 설계다.
- 제공자 추상화는 `gateway/providers.mjs` 레지스트리로 구현됨 (RV-01 DONE): 공통 정규화·재시도·
  서킷브레이커·쿼터 + 네이버·Mojeek 전송 검증 ([제공자 전략](SEARCH_PROVIDER_STRATEGY.md)).

## Data plane (current)

1. 시드: 검증된 사용자 제출 sitemap, 크롤 허용 사이트, 승인 피드, Common Crawl(데이터 이용 조건 준수).
   크롤러는 `CRAWL_ALLOWED_HOSTS` allowlist 없이는 1건도 수집하지 않는다 (fail-closed).
2. URL 정규화(RFC3986·IDN·추적 파라미터 제거) → 호스트 파티션 → robots RFC9309 →
   SSRF·DNS rebinding 방어(IP pinning) → politeness → fetch(크기·폭탄 가드).
3. WARC 원문은 객체 저장(로컬/S3 호환) + SHA-256, PostgreSQL에 메타. 파싱 → canonical →
   `noindex`/`nofollow`/`X-Robots-Tag` → 언어 감지 → SimHash(64bit, 해밍 ≤4) → tombstone.
4. 색인 이벤트는 transactional outbox(PGOutbox) + 멱등 소비자. OpenSearch alias blue-green
   재색인·롤백, 삭제 전파(`is_tombstone` 즉시 제외).
5. 어휘 역색인 + BM25(OpenSearch, Nori 한국어·영어 분석기 검증됨). 하이브리드 회수
   (키워드+fuzzy/phrase RRF, 기본 ON) + 색인 기반 교정·완성(completion suggester) +
   색인 시점 SafeSearch 라벨 + DSL 필터. 벡터 회수는 k-NN 연결점만 있고 임베딩 모델 미확보(BLOCKED).
6. 쿼리 라우터: 오타/한영 교정 → 어뷰즈(PII·버스트) 검사 → 의도 분류(정보/탐색/거래/지역) →
   회수 → 재랭킹(신선도·권위·스팸 감점) → SafeSearch → organic/`sponsored_ads: []` 분리 응답.
   AI Overview는 `AI_OVERVIEW_ENABLED` 기본 OFF (모델 파이프라인 미연동).
7. AI 답변은 근거 있는 허용 문서만 인용하고 출처·시점을 표기한다. 근거 부족 시 일반 검색으로
   fallback하며, YMYL 면책·비용 상한을 강제한다 (코드 구현, 실측·계약 잔여).

## Control plane (current)

- PostgreSQL: `001_initial.sql` 8스키마/86테이블 기준선 + 002~018 정방향 마이그레이션
  (Phase 플래그·webapp 계정·메일·OTP·신고·감사). 역색인 저장 금지, RLS·최소 권한 적용.
- Redis: 분산 어뷰즈 카운터·할당량/캐시. 원장·단일 내구 큐로 사용하지 않는다.
- Kafka 호환 브로커·ClickHouse는 규모 요구 시 채택 (현재 미도입).
- Python ML 워커는 모델 레지스트리·평가 기준 도입 시 분리 (현재 Go 내 휴리스틱 + 연결점).
- 인프라: Compose(dev·prod·cluster), Cloudflare Pages(프론트), Prometheus/Grafana 뼈대,
  GitHub Actions CI. 운영 원격 실행·이미지 서명은 F-5 BLOCKED.

## Isolation & permissions

- 사용자·관리자·파트너는 별도 오리진·배포가 원칙. 현재 관리자 표면은 전 경로 403
  (`ADMIN_DISABLED`), 파트너 콘솔은 의도적 미구현으로 기본 거부한다.
- 역할·조직·객체 소유권 검사는 API·데이터 계층에서 수행하고, 클라이언트 JWT role claim만
  신뢰하지 않는다. 서비스 토큰은 범위 지정 + 교체.
- 광고는 전용 경매·슬롯·원장을 사용하고, 오가닉 점수를 rewriting하지 않으며
  `광고`/`Sponsored` 표기가 필수다. 현재 슬롯은 명시적 빈 배열이다.
- 개인정보 요청·takedown·검색 삭제 tombstone은 1급 운영 플로우다.

## Ranking and evaluation

- 베이스라인: BM25(제목^3.5·헤딩^2.0·앵커^1.8·설명^1.5·본문^1.0) + 신선도 감쇠 +
  권위·스팸 신호 재랭킹. 사이트별 하드코딩 부스트 금지.
- 지표: NDCG@10·MRR@10·zero-result율·클릭편향 보정 계산기 구현. 현재 게이트 데이터는
  12질의 live 골든(NDCG@10 0.9986·MAP 1.0, toy 규모)뿐이며, 수천 질의 인적 판정셋이
  Google급 게이트 조건이다 ([평가 가이드](QUALITY_EVALUATION_GUIDE_KO.md)).
- 지연·가용성 목표는 가설이 아니라 측정 대상이다. 조건(쿼리셋·QPS·색인 규모·장비)을
  고정한 SLO로 운영화하는 것이 잔여 작업이다 (048·091·096).

## Google-class target (delta)

목표 전문은 [GOOGLE_CLASS_SEARCH_STANDARD.md](GOOGLE_CLASS_SEARCH_STANDARD.md).
아키텍처 관점의 핵심 델타는 (1) 제공자 어댑터 추상화, (2) 인적 판정셋 + 실험 플랫폼 운영화,
(3) 벡터 회수 + LTR 단계 도입, (4) 실규모 부하·용량·DR 실측, (5) 라이선스 확보 버티컬 순차 개방이다.
외부 승인(F-1~F-5) 없이 결제·공개 크롤·공개 출시를 활성화하지 않는다.

## Production gap (honest list)

- Compose prod 스택·`/readyz`·`/metrics`·감사로그는 구현됐으나 운영 SLO·경보·DR 실측은 잔여다.
- OpenSearch dev 보안 비활성 설정은 로컬 전용이다. 운영은 TLS·인증·키 관리·WAF·SSRF 방화벽·예산·독립 감사가 필수다.
- Flutter 앱은 검색·히스토리·세이프서치·설정이 동작하는 코드이나 크로스플랫폼 E2E·스토어 심사는 잔여다. Nuxt 이행은 CVE 해소 후 재검토로 Vite를 유지한다.
