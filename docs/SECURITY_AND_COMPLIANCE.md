# P0 safety, security and compliance requirements

- Respect RFC 9309 `robots.txt`, page-level `noindex`, `nofollow`, X-Robots-Tag, copyrights, source licenses, authentication and website terms. When robots fetch fails, honor the RFC fail-closed/fail-open rules precisely, with tests. Do not crawl private content or bypass access controls.
- Strict SSRF defense: allow only HTTP/HTTPS; reject loopback, link-local, RFC1918, unique-local IPv6, metadata IP ranges, `.local`, localhost, embedded credentials, non-approved ports. Re-evaluate each redirect and DNS resolution; pin resolved public IP on connect; egress firewall; block response decompression bombs, oversized payloads and loops.
- Per-host crawl politeness and adaptive scheduling, explicit agent contact, robots cache, cache expiry, conditional GET, retry backoff. No unlimited crawling or Google SERP scraping.
- Takedown: legal notices, privacy delisting, verified publisher removal, tombstones and re-crawl/restore rule. Test deletion across PostgreSQL, OpenSearch, Redis, object storage, backups per retention/hold policy.
- OWASP API Top 10: BOLA, broken auth, object property authorization, rate limits, SSRF, unsafe downstream APIs. Admin login with MFA, security keys where possible, operation approvals and immutable audit trails.
- User privacy: purpose-specific consent, minimal event retention, delete/export request workflow; PIPA/GDPR/local requirements reviewed by counsel before launch. Do not store precise user locations, raw query history or personal identifiers without justified purposes.
- Ads and commerce: disclosure, fraud/invalid-traffic controls, brand safety, signed idempotent billing webhooks, ledger reconciliation, refunds, timezone/currency/tax correctness. No real money in test/staging.
- Media and news: explicit data licenses for thumbs and full text; only index/snippets where permitted. Maps/geocoding requires properly licensed provider or independent compliant datasets.
- Supply chain: pin images/deps by digest/lockfile, SAST/DAST, SBOM, secret scanning, license scanning and provenance.
- Every production release needs incident plan, backups and actual restore test, disaster recovery, full audit evidence, independent red-team on authentication and crawler network boundaries.
