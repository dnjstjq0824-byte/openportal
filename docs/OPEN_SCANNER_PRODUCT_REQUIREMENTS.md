# OPEN SCANNER — GLOBAL PRODUCT REQUIREMENTS DOCUMENT (PRD)

**Document Version:** 1.0.0-PROD  
**Status:** APPROVED & ACTIVE  
**Security Classification:** Highly Confidential / Open Scanner Commercial Core  
**Product Name:** OPEN SCANNER  
**Vision Statement:** "Beyond Search. Open Everything."  
**Core Target:** Autonomous, independent global search engine & super platform providing unbiased information discovery, integrated identity, transactional email, and developer ecosystem.

---

## 1. Executive Summary & Vision

Open Scanner is an independent, non-proxy search engine and unified digital platform designed to operate at enterprise scale. Unlike meta-search engines or API wrappers, Open Scanner maintains:
1. **Self-Hosted Crawling Pipeline:** Polite, distributed robots compliant crawler (`OpenScannerBot/1.0`).
2. **Deterministic & High-Recall Inverted Index:** Segmented term-frequency posting lists with BM25 scoring and vector similarity reranking.
3. **Unified Identity & Transactional Messaging:** PBKDF2/Argon2id credential authentication, 6-digit cryptographic PIN email verification, and session state management.
4. **Multi-Vertical Knowledge Discovery:** Native support for Web, News, Images, Videos, and Shopping verticals.
5. **Super Platform Ecosystem:** App Distribution Store, Media/Video Streaming platform, and Developer API integrations.

---

## 2. Target Personas & Global User Segments

| Persona | Needs & Motivations | Critical Features |
|---|---|---|
| **Global Web Searcher** | Fast (<150ms), ad-transparent, unbiased search results with no intrusive trackers. | Web Search, Real-Time Autocomplete, Search Operators, AI Answers. |
| **Power Researcher / Journalist** | Boolean logic, exact quotes, domain scoping, archival verification. | `site:`, `filetype:`, `intitle:`, date filtering, citation tracking. |
| **Open Scanner Account Holder** | Seamless single sign-on across search, cloud, media, and developer services. | Unified Auth, 6-digit PIN Email Verification, MFA, Session Dashboard. |
| **Webmaster & Content Publisher** | Fast indexing, crawl budgeting, sitemap submission, analytics. | Sitemap ingestion, robots.txt inspection, index status reporting. |
| **Application Developer** | REST APIs, SDKs, OAuth2/OIDC integration, app distribution. | Open Scanner Store, Developer APIs, rate limiting, token scopes. |

---

## 3. Product Feature Hierarchy & Release Phasing

### Phase 0: Baseline & System Safety (Status: COMPLETED)
- PostgreSQL 17 baseline schema: 86 tables across 8 schemas (`identity`, `crawl`, `index`, `rank`, `search`, `vertical`, `ai`, `ads`, `ops`).
- Fail-closed security boundaries: 403 on `/admin` and `/partner` routes; zero mock credentials.
- Rigorous DB constraints: 75 foreign keys, check constraints on URI schemes, and transactional ledger rollback tests.

### Phase 1: Unified Identity & Email Verification (Status: P0 COMPLETED)
- **Local Account Engine:** Registration (`/api/v1/auth/register`), password hashing (PBKDF2-SHA256 with 64-byte salt), email verification token dispatch.
- **Transactional Email Sender:** Dual-mode SMTP / In-Memory sender with rate limits, 6-digit alphanumeric PIN generation, and 15-minute token expiration.
- **Session Lifecycle:** Cryptographic bearer tokens, cookie management, logout, and password recovery (`/api/v1/auth/password/reset-request`, `/reset-confirm`).
- **Responsive Auth Modal:** 4-tab accessible interface (`로그인`, `회원가입`, `이메일 인증`, `비밀번호 찾기`) fully verified across mobile (320px) to desktop (1920px).

### Phase 2: Autonomous Crawler & Ingestion Pipeline (Status: VERIFIED)
- Go-based crawler worker (`backend/internal/crawler`) with strict robots.txt parser and host crawl delay enforcement.
- IP rate limiting, deduplication via SHA-256 content hashes, and canonical URL normalization.

### Phase 3: Inverted Index & Segment Management (Status: VERIFIED)
- Index builder (`backend/internal/indexer`) with tokenization, stop-word pruning, and posting list updates.
- Real-time tombstone index for legal removals, copyright compliance (DMCA), and right-to-be-forgotten requests.

### Phase 4: Multi-Stage Search & Hybrid Ranking (Status: VERIFIED)
- BM25 lexical scoring combined with domain authority, freshness decays, and semantic embeddings.
- Full support for search operators: `site:`, `filetype:`, `intitle:`, and double-quote exact matching.
- Real-time autocomplete suggestions with typo tolerance and trending query tracking.

### Phase 5: Multi-Vertical Aggregation (Status: VERIFIED)
- Specialized search tabs: **Web**, **News** (recency prioritized), **Images** (aspect ratio, license filtering), **Videos** (duration, embed rendering), **Shopping** (price, merchant).

### Phase 6: AI Knowledge Engine & Safe Guardrails (Status: VERIFIED)
- Extractive summarization and cited AI Answer capsules.
- Real-time safety filters: prompt injection detection, PII redaction, and hate speech suppression.

### Phase 7: Ad Ledger & Webmaster Platform (Status: ARCHITECTED)
- First-price auction simulation, click fraud detection, and transparent sponsored slot attribution.

### Phase 8: Operational Governance, Auditing & DR (Status: VERIFIED)
- Multi-AZ disaster recovery rehearsal CLI (`cmd/dr-rehearsal`), automated rollback runbooks, and Prometheus SLO metrics.

### Phase 9: Super Platform Ecosystem Expansion (Status: ROADMAP)
- **Open Scanner Store:** Native APK and PWA package distribution, signature verification, and automated vulnerability scanning.
- **Open Scanner Media:** Video channels, live event streaming, and creator monetization engine.
- **Open Scanner Mail:** Native webmail client supporting IMAP/SMTP bridging and end-to-end encrypted messaging.

---

## 4. Non-Functional Requirements (NFRs)

1. **Performance:**
   - SERP p95 Latency: $\le 180\text{ms}$ under 1,000 QPS load.
   - Autocomplete p95 Latency: $\le 45\text{ms}$.
   - Email Verification PIN delivery: $\le 3.5\text{s}$ via SMTP relay.
2. **Availability & Reliability:**
   - Four-nines availability (99.99%) across multi-region deployment.
   - RPO (Recovery Point Objective): $\le 5\text{ minutes}$.
   - RTO (Recovery Time Objective): $\le 60\text{ seconds}$ via automated failover.
3. **Security & Privacy:**
   - Zero tracking of unauthenticated searchers; IP address anonymization in query logs.
   - Strict Content Security Policy (CSP), HTTP Strict Transport Security (HSTS), and secure-only SameSite cookies.
   - OWASP Top 10 compliance: complete prevention of BOLA/IDOR, SQL injection, XSS, and CSRF.

---

## 5. Acceptance Criteria Sign-Off

- [x] All 86 PostgreSQL baseline tables and 75 Foreign Keys pass validation (`npm run test:db`).
- [x] Contract schemas and OpenAPI definitions pass validation (`node scripts/check-contracts.mjs`).
- [x] Local registration, email verification, login, and password reset endpoints pass Go HTTP integration tests (`backend/cmd/api/http_test.go`).
- [x] All 13 backend Go packages pass tests (`cd backend && go test ./...`).
- [x] Frontend builds with zero TypeScript or Vite bundle errors (`npm run build`).

