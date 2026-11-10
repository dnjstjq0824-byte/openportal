# OPEN SCANNER — GLOBAL SEARCH ENGINE ARCHITECTURE SPECIFICATION
**Document ID:** OPENSCANNER_SEARCH_ENGINE_ARCHITECTURE_20261031  
**Project:** Open Scanner  
**Architecture Classification:** High-Throughput Modular Distributed Search Engine  

---

## 1. High-Level Architectural Diagram

```
                           [ Global User Traffic ]
                                      │
                                      ▼
                        [ Cloudflare Pages Global CDN ]
                        (Edge Routing / WAF / SSL / Assets)
                                      │
                   ┌──────────────────┴──────────────────┐
                   ▼                                     ▼
        [ Edge Worker Runtime ]                [ Reverse Proxy / Tunnel ]
        - BM25 Edge Core                      - Healthcheck & Routing
        - Prefix Autocomplete                 - Request Correlation ID
        - Local Auth / WebAuthn               - TLS Termination
        - Resilient Corpus Serving                       │
                   │                                     ▼
                   │                           [ Go Search Gateway ]
                   │                           - Rate Limiter (Token Bucket)
                   │                           - Query Normalizer & Parser
                   │                           - Korean Nori Tokenizer
                   │                           - RRF / BM25 Ranker
                   │                           - SSRF IP Pinning Protection
                   │                                     │
         ┌─────────┴─────────┐                 ┌─────────┴─────────┐
         ▼                   ▼                 ▼                   ▼
[ OpenSearch 2.19 ]  [ Redis Cache ]   [ OpenSearch 2.19 ]  [ PostgreSQL 17 ]
(documents_v1 / v2)  (Sessions/TTL)    (BM25 Inverted Index)(System Ledger/DB)
```

---

## 2. Core Subsystems

### 2.1. Ingestion & Crawling Pipeline
1. **URL Frontier:** Manages prioritized domain queues, enforced by politeness delays (minimum 1,000ms between requests to the same hostname).
2. **SSRF Guard Dialer:** Resolves DNS using IP Pinning, inspecting addresses against IPv4/IPv6 private subnets (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`, `169.254.169.254`, `::1`) before initiating TCP handshakes.
3. **Robots.txt Engine:** Implements RFC 9309 compliance with user-agent specificity and crawl-delay throttling.
4. **WARC Archival:** Writes raw HTTP requests and responses to ISO 28500 standardized WARC records with SHA-256 content digests for forensic auditability.

### 2.2. Indexing & Analysis Engine
1. **HTML Sanitization & Extraction:** Removes `<script>`, `<style>`, and navigation boilerplate to yield pure body text, titles, canonical tags, and OpenGraph metadata.
2. **Language Detector & Morphological Analysis:** Identifies language (`ko`, `en`), tokenizing Korean queries and documents with Lucene Nori de-compounding and part-of-speech filtering.
3. **SimHash Deduplication:** Generates 64-bit fingerprint hashes, suppressing near-duplicate content with Hamming distance $\le 3$.
4. **Transactional Outbox:** Ensures atomic document upserts and deletion tombstones across PostgreSQL and OpenSearch.

### 2.3. Query & Ranking Engine
1. **Okapi BM25:** Computes relevance scores using parameters $k_1 = 1.2$, $b = 0.75$, scoring term frequency against document length.
2. **Abuse & Spam Demotion:** Penalizes keyword stuffing, hidden text, and low domain authority.
3. **Reciprocal Rank Fusion (RRF):** Merges title, body, and heading matching scores with dampening constant $k = 60$.
4. **SafeSearch Classifier:** Segregates adult or harmful content based on cryptographic dHash values and strict policy flags.

### 2.4. Cloudflare Edge Resiliency Layer
1. Advanced-mode `_worker.js` intercepts all `/api/v1/*` requests.
2. If the backend tunnel is reachable, traffic seamlessly flows upstream with distributed tracing headers (`X-Request-ID`, `X-Correlation-ID`).
3. If disconnected or operating in edge-native mode, the edge worker serves verified corpora, 6-digit PIN email verification, and federated vertical search without failing with HTTP 503 or 404.
