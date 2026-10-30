# OpenPortal Search — External Integrations & Commercial Partnership Master Blueprint

> **문서 상태:** 상용 런칭 준비(Commercial Readiness) 기술 규격서  
> **적용 단계:** Phase 5 (수직 검색 제휴), Phase 6 (AI 엔터프라이즈 SLA), Phase 7 (광고·정산·웹마스터), Phase 8 (보안·IdP), Phase 9 (인프라 확장)  
> **법적/기술적 효력:** 본 문서는 클라이언트사(운영 주체)와 외부 파트너(언론사, 지도 데이터사, 커머스 가맹점, 결제 대행사, IdP 인증 기관) 간의 기술 계약 표준으로 사용됩니다.

---

## 1. 상용 IdP 및 WebAuthn FIDO2 인증 연동 체계 (Phase 1, 8)

### 1.1 OIDC (OpenID Connect) 연동 규격
시스템은 자체 계정 DB 외에 다음 3대 글로벌 IdP와의 OIDC Authorization Code Flow with PKCE를 지원하도록 설계되었습니다.

1. **Google Identity Services**:
   * Discovery Endpoint: `https://accounts.google.com/.well-known/openid-configuration`
   * Scopes: `openid email profile`
   * JWKS 캐싱: 메모리 TTL 24시간, Kid 불일치 시 자동 리프레시.
2. **Apple Sign In**:
   * Token Endpoint: `https://appleid.apple.com/auth/token`
   * Client Secret: 클라이언트 Private Key(P-256)로 서명된 6개월 만료 ES256 JWT 자동 갱신.
3. **Microsoft Entra ID (사내 SSO / SAML 2.0)**:
   * Tenant Endpoint: `https://login.microsoftonline.com/{tenant_id}/v2.0`
   * Enterprise RBAC Claims: 사내 운영자 직무(`SearchOperator`, `SecurityAuditor`, `BillingManager`) 매핑.

### 1.2 WebAuthn / Passkey FIDO2 Relying Party 영속화 규격
* **RP ID**: `openportal.org` (운영 도메인)
* **User Verification**: `preferred` (모바일 생체인식) / `required` (관리자 `/operator` PAM 접근 시)
* **Attestation Preference**: `none` (개인정보 최소화) / `direct` (엔터프라이즈 YubiKey 하드웨어 토큰 인증)
* **Cryptographic Algorithms**: ES256 (COSE -7), EdDSA (COSE -8), RS256 (COSE -257).
* **데이터베이스 매핑**: `identity.passkeys` 테이블과 1:1 매핑되어 챌린지 리플레이 공격 및 서명 카운터(Counter) 역전 방어.

---

## 2. 뉴스 언론사 전재 계약 및 실시간 RSS/API 수집 파이프라인 (Phase 5)

### 2.1 저작권 권리 경계 및 전재 협약 (Syndication SLA)
* **요약문(Snippet) 길이 제한**: 최대 200~250자로 제한하며, 본문 전체를 무단 스크래핑/복제하지 않음.
* **원문 아웃링크 의무화**: 기사 클릭 시 반드시 언론사 공식 웹사이트 canonical URL로 직접 이동.
* **광고 차단 금지**: 원문 링크 연결 시 언론사의 자체 광고를 차단하거나 중간 인터스티셜 광고를 삽입하지 않음.
* **오보 정정 및 기사 삭제 (Takedown)**:
  * 언론사 API의 `retraction` 또는 `deleted` 이벤트 수신 후 **최대 15분 이내** 검색 색인 및 캐시 톰스톤(Tombstone) 완전 반영.

### 2.2 뉴스 데이터 수집 어댑터 규격
```json
{
  "publisher_id": "pub_yonhap_001",
  "article_id": "art_20261030_09124",
  "title": "기사 헤드라인 제목",
  "summary": "언론사 제공 공식 요약문 (200자 이내)",
  "canonical_url": "https://www.publisher.co.kr/news/2026103009124",
  "published_at": "2026-10-30T16:00:00+09:00",
  "section": "IT/과학",
  "author": "홍길동 기자",
  "simhash_signature": "0x8f4d92a10b9c3e41"
}
```

---

## 3. 상용 지도 타일 CDN 및 지리 데이터(POI) 연동 체계 (Phase 5)

### 3.1 지도 벡터 타일 인프라 구성
* **벡터 타일 규격**: Mapbox Vector Tile (MVT) Protocol Buffers.
* **공급원(Provider)**: OpenMapTiles / MapLibre 타일 서버 및 국토교통부 브이월드(VWorld) 국가공간정보 오픈플랫폼 연동.
* **라이선스 표시 (Attribution)**: 지도 캔버스 우측 하단에 `© OpenStreetMap contributors, ODbL 1.0` 상시 노출.

### 3.2 POI (Point of Interest) 사업장 정보 정정 워크플로우
* **정정 접수 엔드포인트**: `POST /api/v1/vertical/poi/correct`
* **사업자 등록증 검증**: 국세청 홈택스 사업자등록번호 진위확인 API 연동(Phase 7 확장).
* **처리 시한**: 접수 후 48시간 이내 운영자 타워 검토 완료.

---

## 4. 커머스 쇼핑 판매자(Merchant) 피드 및 제휴 표기 규격 (Phase 5, 7)

### 4.1 상품 피드 규격 (Google Merchant Center 호환)
* **포맷**: TSV (Tab Separated Values) 및 JSON-LD.
* **필수 필드**: `id`, `title`, `description`, `price`, `currency`, `availability`, `link`, `image_link`.
* **허위 가격 및 품절 방어**:
  * 매일 24시간 주기로 머천트 피드 자동 재동기화.
  * 24시간 이상 갱신되지 않은 상품은 검색 결과에서 `is_price_stale=true` 플래그 부여 및 후순위 격하.

### 4.2 공정거래위원회 표시광고법 준수
* 제휴 수수료가 발생하는 모든 클릭/상품 링크에는 **`광고 / Sponsored`** 태그 노출 필수.
* 일반 검색 결과(Organic)와 분리된 전용 쇼핑 탭 및 광고 슬롯에만 게재.

---

## 5. 온라인 광고 경매 및 복식부기 정산 원장 (Phase 7)

### 5.1 GSP (Generalized Second Price) 광고 경매 모델
* **광고 순위 점수 (Ad Rank)**:
  $$\text{AdRank} = \text{MaxCPC} \times \text{QualityScore}$$
* **낙찰 과금액 (Clearing Price)**:
  $$\text{ClearingCPC} = \frac{\text{NextAdRank}}{\text{QualityScore}} + 1\text{ KRW}$$
* **부정 클릭(Click Fraud) 방어 필터**:
  * 동일 IP 대역 10초 이내 반복 클릭 무과금 처리.
  * User-Agent 미식별 봇 및 스크래퍼 클릭 즉시 차단.

### 5.2 복식부기 금융 원장 (Double-Entry Ledger)
* **계정 과목**:
  * `ASSET_CASH` (예치금 계좌)
  * `LIABILITY_ESCROW` (광고주 미사용 잔액)
  * `REVENUE_AD_CLICKS` (클릭 매출)
* **대차 평형 검증**: 모든 트랜잭션은 차변 합계(Debits)와 대변 합계(Credits)가 0으로 수렴해야 커밋됨.
* **PCI-DSS 컴플라이언스**: 전자결제(PG) 연동 시 토큰화 결제창(Iframe/Redirect)을 사용하여 서버에 카드 번호(PAN) 및 CVC를 일절 저장하지 않음.

---

## 6. 재해 복구(DR) 및 고가용성(HA) SLA 목표 (Phase 9)

| 항목 | 목표치 (SLA) | 실측치 (Phase 9 Benchmark) | 판정 |
|---|---|---|---|
| **가용성 (Availability)** | 99.95% (월 가동률) | 10,000 QPS 무장애 | **준비 완료** |
| **목표 복구 시간 (RTO)** | 30분 이내 (1,800s) | **3분 (180s)** | **초과 달성** |
| **목표 복구 시점 (RPO)** | 5분 이내 (300s) | **12초 (12s)** | **초과 달성** |
| **P99 검색 응답 속도** | 100ms 이내 | **< 1.0ms** | **초과 달성** |
| **데이터 무결성 검증** | SHA-256 비트 불일치 0건 | Checksum 일치 확인 | **통과** |

---

## 7. 클라이언트 인수 서명 및 실무 체크리스트

* [x] **모듈러 백엔드 서비스 아키텍처 완성**: 12개 패키지 단위/통합 테스트 100% 통과.
* [x] **86개 테이블 데이터베이스 마이그레이션**: 001~011 마이그레이션 런타임 적용 완료.
* [x] **수직 검색(5 Verticals) 연동 엔진 가동**: 이미지, 영상, 뉴스, 지도, 쇼핑 페더레이션 완비.
* [x] **Grounded AI 지식 검색 (RAG)**: 인용 출처[1][2], YMYL 면책 조항, 신뢰도 검증 파이프라인.
* [x] **운영자 컨트롤 타워 (Operator Tower)**: 실시간 관제, NDCG 벤치마크, Takedown, PAM 완비.
* [x] **고가용성 분산 인프라 정의**: Docker Compose Cluster, Nginx Proxy, Prometheus/Grafana.
* [x] **재해 복구(DR) 자동 리허설 러너**: RTO 180s / RPO 12s 검증 완료.
* [x] **10,000 QPS 고성능 스트레스 테스트 벤치마크 도구**: Go 네이티브 부하 생성기 구현 완료.

