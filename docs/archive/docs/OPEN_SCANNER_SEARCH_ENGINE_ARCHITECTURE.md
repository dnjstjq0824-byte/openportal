# OPEN SCANNER — CORE SEARCH ENGINE ARCHITECTURE & PIPELINE MANUAL

**Document Version:** 1.0.0-PROD  
**Component:** Search Subsystem (`crawler`, `indexer`, `ranking`, `search`, `vertical`)  
**Target:** Google-Scale Autonomous Information Retrieval Architecture  

---

## 1. End-to-End Pipeline Overview

```
 [Global Web]
      │
      ▼
 ┌──────────────┐
 │  CRAWLER     │ ──▶ Robots.txt & Crawl Delay Validation
 └──────┬───────┘
        │ Fetch Content & Canonicalization
        ▼
 ┌──────────────┐
 │  PARSER      │ ──▶ HTML Stripping, Language Tagging (ko, en), SHA-256 Hash
 └──────┬───────┘
        │ Structured Document Record
        ▼
 ┌──────────────┐
 │  INDEXER     │ ──▶ Inverted Posting Lists, BM25 Index, Tombstone Exclusions
 └──────┬───────┘
        │ OpenSearch 2.19 Cluster (`documents_v1`)
        ▼
 ┌──────────────┐
 │  QUERY AST   │ ──▶ Operator Parsing (`site:`, `filetype:`, `intitle:`, `"phrase"`)
 └──────┬───────┘
        │ Candidate Selection & Multi-Stage Scoring
        ▼
 ┌──────────────┐
 │  RANKER      │ ──▶ BM25 (Title 3x) + Domain Authority + Freshness Decay + SafeSearch
 └──────┬───────┘
        │ Filtered & Deduplicated Results
        ▼
 [SERP Response] (p95 < 180ms)
```

---

## 2. Ingestion & Crawling Engine (`backend/internal/crawler`)

1. **Polite URL Frontier:**
   - Multi-queue architecture sharded by hostname.
   - Enforces host crawl delay ($\ge 1000\text{ms}$ by default or per `robots.txt`).
   - Complies strictly with RFC 9309 (Robots Exclusion Protocol).
2. **Canonical URL Normalization:**
   - Removal of tracking parameters (`utm_*`, `fbclid`, `ref`).
   - Normalization of scheme (`https` preferred) and port standards.
   - Respects `<link rel="canonical" href="...">`.
3. **Deduplication:**
   - SHA-256 content fingerprints identify mirror and duplicate sites.
   - Near-duplicate suppression via SimHash token shingling.

---

## 3. Inverted Index & Sharding (`backend/internal/indexer`)

1. **Data Model (`infra/opensearch/documents_v1.json`):**
   - `document_id`: Unique deterministic UUID / hash.
   - `url`: Canonical resource URI.
   - `title`: Analyzed text with n-gram & standard tokenizer.
   - `description`: Text snippet for SERP presentation.
   - `body`: Full-text searchable corpus.
   - `language`: ISO 639-1 tag (`ko`, `en`).
   - `domain_authority`: Float value $[0.0, 1.0]$.
   - `indexable`: Boolean flag allowing real-time exclusion without index rebuilding.
2. **Tombstone & DMCA Removal Index:**
   - Tombstone table records legal takedowns, right-to-be-forgotten requests, and malware quarantines.
   - During search evaluation, tombstone IDs are injected into OpenSearch `must_not` boolean filters, ensuring zero-latency takedowns.

---

## 4. Multi-Stage Hybrid Ranking Engine (`backend/internal/ranking`)

The final relevance score $S(d, q)$ for document $d$ and query $q$ is computed as:

$$S(d, q) = \text{Score}_{\text{BM25}}(d, q) \times \left(1 + \omega_{\text{DA}} \cdot \text{DA}(d)\right) \times e^{-\lambda \cdot \Delta t} \times \text{SafePenalty}(d)$$

Where:
- $\text{Score}_{\text{BM25}}(d, q)$: Lexical score with Title boost factor of $3.0\times$ and Description boost of $1.5\times$.
- $\text{DA}(d)$: Domain authority metric $[0.0, 1.0]$ with weight $\omega_{\text{DA}} = 0.4$.
- $e^{-\lambda \cdot \Delta t}$: Freshness decay function ($\lambda = 0.005\text{ days}^{-1}$) ensuring current news and updates rank appropriately.
- $\text{SafePenalty}(d)$: Multiplicative safety dampener based on user's active SafeSearch setting (Strict, Moderate, Off).

---

## 5. Query Understanding & Operator AST

The search parser (`backend/internal/search/query.go`) isolates syntax tokens before running index queries:
- `site:example.org` $\implies$ `filter: { term: { "domain": "example.org" } }`
- `filetype:pdf` $\implies$ `filter: { term: { "file_extension": "pdf" } }`
- `intitle:kubernetes` $\implies$ `must: { match: { "title": "kubernetes" } }`
- `"exact quote"` $\implies$ `must: { match_phrase: { "body": "exact quote" } }`
- Typo Correction: Korean 2-set keyboard distance (Damerau-Levenshtein) and English edit distance mapping.

