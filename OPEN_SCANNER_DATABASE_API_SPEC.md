# OPEN SCANNER — DATABASE SCHEMA & API SPECIFICATION

**Document Version:** 1.0.0-PROD  
**Database:** PostgreSQL 17 Enterprise  
**Baseline Coverage:** 86 Tables / 8 Schemas / 75 Foreign Keys  
**API Engine:** Go 1.24 High-Throughput HTTP Gateway  

---

## 1. Database Schema Breakdown (86 Tables Across 8 Schemas)

```
  PostgreSQL 17 Database
  ├── identity (8 tables)    ── accounts, credentials, sessions, roles, permissions...
  ├── crawl (12 tables)       ── hosts, robots_txt, seeds, crawl_queue, crawl_history...
  ├── index (10 tables)       ── documents, inverted_terms, postings, tombstones...
  ├── rank (11 tables)        ── domain_authority, ranking_profiles, click_signals...
  ├── search (9 tables)       ── queries, query_sessions, corrections, analytics...
  ├── vertical (14 tables)    ── news_articles, image_metadata, video_channels, products...
  ├── ai (10 tables)          ── knowledge_nodes, citations, summaries, guardrail_logs...
  └── ads (12 tables)         ── advertisers, campaigns, creatives, click_ledger...
```

### 1.1 `identity` Schema (P0 Foundation)
- `identity.accounts`: Core user table (`id UUID PK`, `email TEXT UNIQUE`, `display_name TEXT`, `status TEXT`, `email_verified BOOLEAN`, `created_at TIMESTAMPTZ`).
- `identity.credentials`: Hashed credentials (`account_id UUID FK`, `algo TEXT`, `password_hash TEXT`, `salt BYTEA`, `updated_at TIMESTAMPTZ`).
- `identity.sessions`: Active sessions (`id UUID PK`, `account_id UUID FK`, `token_hash TEXT UNIQUE`, `expires_at TIMESTAMPTZ`, `ip_address INET`).
- `identity.email_verifications`: Verification ledger (`id UUID PK`, `email TEXT`, `pin_hash TEXT`, `attempts INT`, `expires_at TIMESTAMPTZ`).
- `identity.roles` & `identity.account_roles`: Role-based access control.

### 1.2 `crawl` & `index` Schemas
- `crawl.hosts`: Host metadata, protocol schemes (`http`, `https`), robots.txt status, crawl delay.
- `crawl.crawl_queue`: Frontier queue with priority scores and lease timers.
- `index.documents`: Crawled document records with canonical URL hashes, language tags, and content checksums.
- `index.tombstones`: Fast-lookup table for DMCA removals and legal take-downs.

### 1.3 `vertical` Schema
- `vertical.news_articles`: Syndicated news feeds, publisher attribution, published timestamps.
- `vertical.image_metadata`: Visual dimensions, aspect ratios, thumbnails, licensing.
- `vertical.video_metadata`: Streaming provider, video durations, embed URLs.
- `vertical.shopping_products`: Merchant ID, SKU, product titles, currency, inventory status.

---

## 2. API Gateway Routes & Security Matrix

| HTTP Method | Route Path | Access Level | Description |
|---|---|---|---|
| `GET` | `/healthz` | Public | Liveness probe (`{"status":"ok"}`) |
| `GET` | `/readyz` | Public | Readiness probe (verifies OpenSearch connectivity) |
| `GET` | `/api/v1/search` | Public | Universal search endpoint (BM25 + Ranking + SafeSearch) |
| `GET` | `/api/v1/suggest` | Public | Query autocomplete & suggestions |
| `GET` | `/api/v1/portal/config` | Public | Client environment flags & localized strings |
| `GET` | `/api/v1/vertical/search` | Public | Vertical-specific search (Images, Videos, News, Shopping) |
| `POST` | `/api/v1/auth/register` | Public | Open Scanner account registration & PIN dispatch |
| `POST` | `/api/v1/auth/email/verify` | Public | 6-digit email PIN verification & activation |
| `POST` | `/api/v1/auth/login` | Public | PBKDF2 local credential login & session generation |
| `POST` | `/api/v1/auth/password/reset-request` | Public | Password reset token request |
| `POST` | `/api/v1/auth/password/reset-confirm` | Public | Password reset confirmation with token |
| `GET` | `/api/v1/ops/idp-verify` | Public | Enterprise IdP configuration status probe |
| `GET` | `/admin` | Denied | Fail-closed administrative boundary (404/403) |
| `GET` | `/partner` | Denied | Fail-closed partner boundary (404/403) |
