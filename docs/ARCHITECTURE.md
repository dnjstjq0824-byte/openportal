# Technical architecture decisions (baseline)

## Product boundary
- OpenPortal Search is an independent search portal **inspired by** mainstream search patterns. Do not copy Google marks, protected visuals, proprietary back-end code or extract/republish Google SERPs unlawfully.
- Real independent search requires rights-compliant discovery -> crawling -> extraction -> deduplication -> indexing -> ranking -> serving. A UI returning externally sourced results is only a metasearch frontend and must be labelled as such.
- First market: Korean (`ko-KR`), support English (`en-US`) at MVP. Later add multilingual.
- Do not secretly personalize: personalization requires consent, opt-out and deletion.

## Data plane
1. Seeds from verified user submitted sitemaps, sites permitted to crawl, approved feeds and Common Crawl subject to its data usage requirements.
2. URL normalize (RFC3986, IDN, path, query parameter policy); host partition, hostname/IP verification and rebinding defenses, robots RFC9309 parsing, no private network/metadata access, proper fetch quotas.
3. Persist WARC/raw to object storage with legal basis, object-key metadata in PostgreSQL; fetch, parse, canonical, `noindex`/`nofollow`/`X-Robots-Tag`, language detection, near-duplicate simhash, removal tombstones.
4. Queue indexing event by transactional outbox + Kafka-compatible transport. Consumers must be idempotent; OpenSearch alias blue/green reindex; deletion propagation verified.
5. Core lexical inverted index and BM25 on OpenSearch; Korean morphology requires validated Nori plugin/custom analyzer rather than claiming standard analyzer handles Korean well; optional embeddings, vector retrieval and hybrid RRF later.
6. Query router: spelling/autocomplete, safe search, query analysis, retrieval, rerank, answer features, organic results vs strictly disclosed ads.
7. AI answers only on grounded licensed/retrieved sources and show citations; decline unsupported answer rather than invent.

## Control plane
- PostgreSQL `database/migrations/001_initial.sql` stores 86 logical baseline tables across 8 schemas; application migrations add constraints, partitions, RLS, indexes, and performance improvements. This schema is **not** the final feature-complete implementation.
- Redis for quota/cache but NOT durable event log. Kafka-compatible broker at scale. ClickHouse only when aggregate event load justifies it. Raw events must not include clear-text queries by default.
- User web Vue 3 + Nuxt + TypeScript with SSR where relevant; Flutter iOS/Android; optional desktop Tauri. Initial starter uses Vue + Vite, and Codex migrates to Nuxt in Phase 1.
- Go services: gateway, identity, search, suggest, crawler-coordinator, fetcher, extractor, indexer, query-analytics, ads, billing, webmaster, moderation. Avoid creating empty microservices as a substitute for real bounded contexts.
- Python AI workers use model registry and evaluation, not safety-critical account/billing authorization.

## Isolation & permissions
- Separate origins and deployments for user/admin/partner. Admin requires MFA, short session, device posture as needed and audit trails. Partner must never have admin permissions or query another tenant.
- Default-deny roles and organization membership checks at the API and data layers. Never trust role claims in client-controlled JWT alone. Service tokens scoped + rotated.
- Ads have dedicated auctions and sponsored containers. Ads must never rewrite organic rank or be falsely presented as unsponsored.
- Privacy requests, takedowns and search removal tombstones are first-class operational flows, not a backlog afterthought.

## Ranking and evaluation
- Baseline: BM25 on title/description/body + site-quality/recency features; avoid hard-coded per-site boosts. Pairwise/learning-to-rank later with reviewed judgments.
- Define Korean & English judgment sets, spam/adversarial cases, NDCG@10, MRR@10, zero-results %, latency p50/p95/p99, index freshness, false positives in safe search, ad invalid-traffic rate.
- Target performance is **a hypothesis** to benchmark and tune on provisioned hardware; never claim Google parity. Each phase has its own measured threshold.

## Engineering decisions and production gap
- Starter code intentionally provides only a demo OpenSearch-based search vertical slice. No real crawl pipeline, auth, admin/partner, AI, native build or payment stack is implemented yet.
- Compose file is for local development and disables OpenSearch security; MUST NOT be exposed publicly. Production needs TLS, auth, key management, WAF, SSRF firewalls, budgets and independent audit.
