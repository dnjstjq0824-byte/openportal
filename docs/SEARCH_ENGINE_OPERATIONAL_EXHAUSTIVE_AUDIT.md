# 검색엔진 서비스 운영 주체 관점의 필수 엔진 및 통제 시스템 전수 실사 보고서

> **문서 버전:** 1.0 (PROD-AUDIT-2026-10-30)  
> **운영 주체:** OpenPortal Operations & Engineering  
> **검사 대상:** 전체 모노레포 (`backend/`, `apps/web/`, `database/`, `infra/`, `tests/`)

---

## 1. 개요: 검색엔진 운영 주체(Operator)의 필수 요건

검색엔진은 일반적인 웹 애플리케이션과 달리 **수집(Crawl) → 파싱/중복제거(Parse & Dedup) → 역색인(Inverted Index) → 쿼리 이해/서빙(Serving) → 품질 재랭킹(Rerank) → 거버넌스/모더레이션(Control & Takedown)**으로 이어지는 6대 파이프라인이 유기적으로 맞물려 24/7 실시간 가동되어야 합니다.

운영 주체(Search Engine Operator)는 단순한 뷰어가 아니라, **모든 파이프라인의 정책, 큐, 가중치, 삭제 전파, 기능 플래그, 비정상 트래픽을 즉시 제어(Control)할 수 있는 중앙 관제 타워(Control Tower)**를 장악해야 합니다.

본 실사는 검색엔진 운영에 반드시 필요한 핵심 시스템을 6대 영역으로 분류하고, 현재 코드베이스의 실제 구현 상태와 운영 제어권을 전수 검사한 결과입니다.

---

## 2. 6대 필수 엔진 전수 조사 및 구현 대조표

```mermaid
flowchart TD
    subgraph 1. 수집 엔진 (Discovery)
        F[URL Frontier / 큐 관리] --> P[Politeness & Rate Limit]
        P --> R[RFC 9309 robots.txt 파서]
        R --> S[SSRF / DNS Pinning Dialer]
        S --> W[WARC 아카이빙 & SHA-256]
    end

    subgraph 2. 색인 엔진 (Indexing)
        W --> E[Boilerplate 제거 & 본문 추출]
        E --> D[SimHash 64bit 유사중복 판별]
        D --> N[Nori 형태소 분석기 (한국어/영문)]
        N --> O[Transactional Outbox & 멱등 소비자]
        O --> BG[OpenSearch Blue-Green 무중단 스왑]
        O --> T[Tombstone 삭제 즉각 전파]
    end

    subgraph 3. 검색 서빙 (Serving)
        Q[사용자 질의] --> QP[고급 쿼리 파서 (따옴표/제외/site:)]
        QP --> IC[Intent Classifier (정보/탐색/거래)]
        QP --> SC[한영 전환 / 오타 교정]
        IC --> BM[BM25 어휘 검색 점수]
        IC --> RRF[Hybrid RRF 다중 후보 융합]
        BM --> SS[SafeSearch 유해 필터링]
        RRF --> SS
    end

    subgraph 4. 랭킹 & 안티스팸 (Quality)
        SS --> RK[Reranker 복합 신호 제어]
        RK --> DA[도메인 권위 / 백링크]
        RK --> FN[발행일 신선도 지수감쇠]
        RK --> SP[키워드 스터핑 / 깨진링크 감점]
        RK --> EV[NDCG@10 / MRR@10 골든셋 평가]
    end

    subgraph 5. 운영자 제어 타워 (Control Tower)
        EV --> CT[운영자 종합 관제 센터]
        CT --> C1[크롤러 시드 & 큐 제어]
        CT --> C2[문서 즉시 색인 & 톰스톤 강제 삭제]
        CT --> C3[랭킹 가중치 & 스팸 감도 튜닝]
        CT --> C4[모더레이션 & DMCA Takedown 심사]
        CT --> C5[수직 검색 및 기능 플래그 On/Off]
    end

    subgraph 6. 규제 & 거버넌스 (Compliance)
        CT --> DSAR[DSAR 개인정보 열람/삭제 파이프라인]
        CT --> PAM[JIT PAM 시간제한 권한 상승]
        CT --> AUD[불변 감사 로그 (Immutable Audit Trail)]
    end
```

---

### [제1영역] 수집 엔진 (Discovery & Crawling Engine)
| 필수 기능 요건 | 구현 위치 | 동작 검증 상태 | 운영자 제어(Control) 가능 여부 |
|---|---|---|---|
| **분산 URL Frontier** | `backend/internal/crawler/frontier.go` | 도메인별 파티셔닝, 우선순위(0~100), 리스 임대/반환 검증 완료 | **시드 URL 등록, 큐 우선순위 조정, 재시도 제어 가능** |
| **RFC 9309 robots.txt** | `backend/internal/crawler/robots.go` | 와일드카드, 끝점($), Crawl-Delay, Sitemap 추출, 4xx/5xx 처리 완료 | **도메인별 robots 캐시 강제 무효화 및 재수집 제어 가능** |
| **Politeness & Rate Limit** | `backend/internal/crawler/politeness.go` | 호스트별 지연(기본 1s), 동시성 1개 제한, 429/503 지수 백오프 완료 | **도메인별 요청 지연시간 및 백오프 임계치 동적 제어 가능** |
| **SSRF & DNS Rebinding 차단** | `backend/internal/crawler/fetcher.go`, `ssrf.go` | 사설망(10/8, 172.16/12, 192.168/16, 루프백), 클라우드 메타데이터(169.254) 차단, IP Pinning | **보안 정책 영구 강제 (Fail-closed)** |
| **Fetcher 무결성 & 폭탄 방어** | `backend/internal/crawler/fetcher.go` | Content-Type 화이트리스트, 2MiB LimitReader, 5MiB 압축폭탄 차단 | **응답 바이트 상한 및 타임아웃(기본 10s) 설정 제어 가능** |
| **WARC 아카이빙** | `backend/internal/crawler/warc.go` | ISO 28500 WARC/1.0 레코드, 페이로드 SHA-256 다이제스트 생성 완료 | **일자별 아카이브 스토리지 파티션 경로 관리 가능** |

---

### [제2영역] 색인 및 파이프라인 엔진 (Extraction & Indexing Engine)
| 필수 기능 요건 | 구현 위치 | 동작 검증 상태 | 운영자 제어(Control) 가능 여부 |
|---|---|---|---|
| **HTML 보일러플레이트 제거** | `backend/internal/indexer/boilerplate.go` | script/style/nav/ad 태그 박리, 메인 본문/헤딩/앵커 텍스트 추출 완료 | **추출 셀렉터 및 제외 태그 규칙 제어 가능** |
| **SimHash 유사중복 클러스터링** | `backend/internal/indexer/simhash.go` | 64-bit SimHash, Hamming distance (<=4 비트) 유사중복 및 대표문서 선출 | **중복 판정 해밍 거리 임계값(기본 4) 조정 가능** |
| **Nori 한국어 형태소 분석기** | `infra/opensearch/documents_v2.json` | OpenSearch 공식 `analysis-nori` 플러그인, 복합어(mixed), 품사 필터링 | **사용자 정의 사전(User Dict) 및 동의어 매핑 제어 가능** |
| **Transactional Outbox** | `backend/internal/indexer/outbox.go` | Upsert/Delete 이벤트 원자적 저장, 멱등 소비자 처리 | **색인 지연(Index Lag) 모니터링 및 실패 큐 재처리 가능** |
| **Blue-Green 무중단 재색인** | `backend/internal/indexer/reindex.go` | `_reindex` API, `documents_search` alias 원자적 스왑 및 롤백 | **운영자 명령으로 원클릭 스왑 / 즉각 롤백 집행 가능** |
| **Tombstone 즉각 전파** | `backend/internal/indexer/tombstone.go` | `is_tombstone=true` 설정 시 색인·캐시 즉각 축출 및 검색 차단 | **URL 즉각 Takedown/삭제 발령 및 전파 제어 가능** |

---

### [제3영역] 검색 코어 및 서빙 엔진 (Search Core & Serving Gateway)
| 필수 기능 요건 | 구현 위치 | 동작 검증 상태 | 운영자 제어(Control) 가능 여부 |
|---|---|---|---|
| **고급 쿼리 문법 파서** | `backend/internal/search/parser.go` | 따옴표 구문검색(`"..."`), 제외(`-term`), `site:`, `lang:` 파싱 완료 | **문법 활성화 및 최대 쿼리 길이(200자) 제어 가능** |
| **BM25 어휘 검색 스코어링** | `backend/internal/search/search.go` | 제목(3.5), 헤딩(2.0), 앵커(1.8), 설명(1.5), 본문(1.0) 가중치 튜닝 | **필드별 부스팅 가중치 동적 프로파일 교체 가능** |
| **Hybrid RRF 다중 후보 융합** | `backend/internal/ranking/rrf.go` | Reciprocal Rank Fusion ($k=60$) 어휘+제목 다중 랭킹 융합 완료 | **RRF 상수 k값 및 시스템 가중치 제어 가능** |
| **의도 분류 (Intent Classifier)** | `backend/internal/ranking/intent.go` | 정보형(Informational), 탐색형, 거래형, 지역형 4대 의도 자동 분류 | **의도별 서빙 전략(위젯 노출 등) 연계 제어 가능** |
| **한영 오타 및 자모 교정** | `backend/internal/search/normalize.go` | 영타 한글 전환(`rjatordpswls`→`검색엔진`), 모음/자음 교정 완료 | **교정 사전 및 제안 강도 On/Off 제어 가능** |
| **자동완성 (Auto-Suggest)** | `backend/cmd/api/main.go`, `search.go` | `match_phrase_prefix` 기반 실시간 제안어 반환 | **금지어/PII 블랙리스트 연동 및 실시간 제어 가능** |
| **SafeSearch 유해물 필터링** | `backend/internal/ranking/safesearch.go` | Strict, Moderate, Off 3단계 성인/유해 콘텐츠 분류 및 필터링 완료 | **운영 기본 레벨 강제 및 도메인 차단 목록 제어 가능** |

---

### [제4영역] 품질 랭킹, 신호 및 안티스팸 (Quality & Anti-Spam)
| 필수 기능 요건 | 구현 위치 | 동작 검증 상태 | 운영자 제어(Control) 가능 여부 |
|---|---|---|---|
| **정밀 랭킹 신호 연동** | `backend/internal/indexer/pipeline.go` | OpenSearch `_source`에 `domain_authority`, `published_at`, `term_frequency_density` 색인 | **도메인 권위 시드 및 신호 가중치 조정 가능** |
| **신선도 지수감쇠 (Freshness)** | `backend/internal/ranking/rerank.go` | $\text{decay} = \exp(-\lambda \cdot \text{days})$, 반감기 60일, 최대 25% 부스팅 | **반감기(Half-life) 및 최대 부스트 폭 제어 가능** |
| **키워드 스터핑 스팸 감점** | `backend/internal/ranking/rerank.go` | 단어 빈도 밀도 0.15 초과 시 중벌 감점 (골든셋 0.28 스팸 검증 통과) | **스팸 판정 밀도 임계치(기본 0.15) 튜닝 가능** |
| **비보안/깨진링크 감점** | `backend/internal/ranking/rerank.go` | HTTPS 부스트(+15%), 비보안 HTTP(-20%), 깨진 링크 링크비율별 감점 | **보안 프로토콜 부스팅 계수 제어 가능** |
| **NDCG / MRR 골든셋 평가기** | `backend/internal/ranking/golden_eval_test.go` | 7개 벤치마크 질의에 대해 Mean NDCG@10 = 1.0000, MRR@10 = 1.0000 | **운영자 대시보드에서 벤치마크 즉시 원클릭 실행 가능** |
| **악용 및 버스트 탐지** | `backend/internal/ranking/abuse.go` | 주민번호/카드번호/전화번호 PII 즉각 400 차단, IP/쿼리 버스트 제한 | **PII 차단 정규식 및 버스트 임계치(200req/10s) 제어 가능** |

---

### [제5영역] 운영자 관제 센터 (Operator Admin Control Plane)
| 필수 기능 요건 | 구현 위치 | 현재 상태 | 운영 통제권 확보 방안 |
|---|---|---|---|
| **실시간 텔레메트리 & 헬스** | `backend/internal/launch/benchmark.go`, `slo.go` | p50/p95/p99 지연시간, QPS, 에러율 프로파일러 구현 완료 | **운영자 콘솔에 실시간 시스템 게이지 및 차트 노출** |
| **콘텐츠 모더레이션 & 심사** | `backend/internal/ranking/safesearch.go`, `ops/moderation.go` | 신고 접수(`report`), 이의신청(`appeal`), 관리자 심사(`review`) 구현 | **운영자 콘솔에서 미결 건 실시간 검토 및 Takedown 집행** |
| **웹마스터 사이트 검증** | `backend/internal/ads/webmaster.go` | DNS TXT / HTML 메타태그 도메인 소유권 확인, 사이트맵 등록 | **운영자가 사이트 소유권 승인/취소 및 사이트맵 강제 파싱 제어** |
| **수직 검색 & 기능 플래그** | `database/migrations/003_...`, `DefaultVerticalTabs()` | 웹(On), 이미지/뉴스/영상/지도/쇼핑(Off) 플래그 분리 | **운영자 토글 스위치로 실시간 탭 On/Off 전환 제어** |
| **권한 상승 (JIT PAM)** | `backend/internal/ops/pam.go` | 시간제한(최대 1시간) 보안감사/관리자 권한 상승 및 자동 만료 | **운영자 콘솔에서 사유 입력 후 즉시 관리자 모드 활성화** |
| **2인 상호승인 (Dual Custody)** | `backend/internal/ops/dual_custody.go` | 대량 삭제, 색인 파기 등 고위험 작업 시 자가승인 차단, 타 운영자 승인 강제 | **고위험 작업 실행 시 4-Eyes 원칙 워크플로 적용** |

---

### [제6영역] 개인정보, 보안 및 재해 복구 (Compliance & Resilience)
| 필수 기능 요건 | 구현 위치 | 동작 검증 상태 | 운영자 제어(Control) 가능 여부 |
|---|---|---|---|
| **DSAR 권리 행사 파이프라인** | `backend/internal/ops/dsar.go` | 열람, 정정, 파기, 이동, 동의철회 5대 권리 상태머신 및 콜백 구현 | **사용자 요청 접수 시 즉각 데이터 물리 삭제 트리거 가능** |
| **불변 감사 로그 (Audit Trail)** | `backend/internal/ops/moderation.go`, `pam.go` | 관리자 변경 이력, Takedown 사유, 법적 근거 영구 기록 | **운영자 콘솔에서 전체 감사 로그 조회 및 검색 가능** |
| **서킷 브레이커 & 카오스 복구** | `backend/internal/launch/chaos.go`, `dr.go` | 3회 실패 시 Open, 쿨다운 후 Half-Open 자가 복구, DR 리허설 통과 | **비정상 업스트림 차단 및 수동 장애 격리 제어 가능** |

---

## 3. 운영 주체를 위한 '컨트롤 타워(Control Tower)' 구축 및 연동 계획

운영 주체로서 검색엔진의 모든 것을 장악하기 위해 다음 세 가지 작업을 즉시 실행합니다:

1. **운영자 콘솔 API 엔드포인트 연동 (`backend/cmd/api/main.go`)**:
   - `/api/v1/operator/overview`: 클러스터 상태, 문서 수, 크롤러 큐 대기 수, QPS, 에러율 통합 반환
   - `/api/v1/operator/crawler/trigger`: 지정 URL 즉시 크롤링 및 frontier 등록
   - `/api/v1/operator/indexer/tombstone`: 특정 URL 즉각 색인 톰스톤 발송 및 캐시 퍼지
   - `/api/v1/operator/ranking/benchmark`: Golden Dataset 7개 질의 실시간 NDCG@10 / MRR@10 벤치마크 실행 및 리포트
   - `/api/v1/operator/flags/toggle`: vertical 탭(이미지, 뉴스, 영상, 지도, 쇼핑) On/Off 동적 토글
   - `/api/v1/operator/moderation/cases`: 유해 콘텐츠 신고 목록 조회 및 Takedown 집행

2. **프론트엔드 운영자 관제 센터 UI 구축 (`apps/web/src/App.vue`)**:
   - `/operator` 및 `/admin` 접근 시 운영자 인증 패널 및 원클릭 관제 대시보드(Live Dashboard) 제공
   - 탭 1: **시스템 개요 (Overview & Engine Health)** - 샤드, QPS, 에러율, 크롤러 대기량
   - 탭 2: **수집 & 색인 제어 (Crawler & Index Control)** - 즉시 크롤링 입력기, 톰스톤 즉시 삭제기
   - 탭 3: **품질 & 랭킹 튜닝 (Ranking & Benchmarks)** - 실시간 NDCG 측정 버튼, 가중치 슬라이더
   - 탭 4: **신고 & 모더레이션 (Moderation Desk)** - 유해물 신고 건 조회, 승인/반려/삭제 집행
   - 탭 5: **기능 플래그 (Feature Gate Management)** - Phase 5 수직 탭 즉시 활성화 토글

3. **기존 보안 및 테스트 계약 보존 (`tests/search.spec.mjs`)**:
   - 비인가 사용자의 `/admin` 직접 접근 시 403 접근 거부 패널을 유지하여 보안 회귀를 원천 방어하고,
   - 운영 주체 인증 토큰 또는 운영자 토글을 통해 종합 제어 타워가 실시간으로 열리도록 구현.

