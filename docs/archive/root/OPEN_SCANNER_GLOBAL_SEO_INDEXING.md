# OPEN SCANNER — GLOBAL SEO, CRAWLING & WEBMASTER INDEXING GUIDE

**Document Version:** 1.0.0-PROD  
**Bot User-Agent:** `OpenScannerBot/1.0 (+https://openscanner.internal/bot)`  
**Scope:** Global Publishers, Webmasters & Autonomous Indexing Pipeline  

---

## 1. OpenScannerBot Crawling Architecture

OpenScannerBot is an autonomous, polite web crawler designed to index the open web without degrading host servers.

### 1.1 Technical Identification
- **User-Agent String:** `Mozilla/5.0 (compatible; OpenScannerBot/1.0; +https://openscanner.internal/bot)`
- **IP Range Verification:** Cryptographic rDNS lookup matching `*.bot.openscanner.internal`.
- **Crawl Rate Control:** Defaults to a minimum 1.0 second delay between requests to the same host; honors host `Crawl-delay` directives up to 30.0 seconds.

### 1.2 Robots.txt Compliance (RFC 9309)

Publishers can manage access via `/robots.txt`:

```http
User-agent: OpenScannerBot
Allow: /public/
Disallow: /private/
Disallow: /checkout/
Crawl-delay: 2

Sitemap: https://example.com/sitemap.xml
Sitemap: https://example.com/sitemap-news.xml
```

---

## 2. Ingestion & Structured Data Standards

### 2.1 XML Sitemap Protocols
Open Scanner supports standard XML sitemaps as well as specialized vertical sitemaps:
- **Web Sitemap (`sitemap.xml`):** `<loc>`, `<lastmod>`, `<changefreq>`, `<priority>`.
- **News Sitemap:** `<news:publication>`, `<news:publication_date>`, `<news:title>`.
- **Video Sitemap:** `<video:thumbnail_loc>`, `<video:title>`, `<video:player_loc>`, `<video:duration>`.
- **Image Sitemap:** `<image:loc>`, `<image:caption>`, `<image:license>`.

### 2.2 Semantic & Structured Markup Extraction
Open Scanner automatically extracts and indexes:
1. **Schema.org JSON-LD / Microdata:** `Article`, `NewsArticle`, `Product`, `Organization`, `WebSite`, `BreadcrumbList`, `FAQPage`.
2. **OpenGraph & Social Meta:** `og:title`, `og:description`, `og:image`, `og:type`.
3. **Canonical Tags:** Strict obedience to `<link rel="canonical" href="...">`.
4. **Hreflang Internationalization:** `<link rel="alternate" hreflang="ko-KR" href="..." />` ensures regional searchers are served geographically relevant versions.

---

## 3. Webmaster Console & Domain Ownership Verification

Publishers can verify ownership and manage indexing through two automated methods:
1. **DNS TXT Record:**
   - Host: `@` or domain root.
   - Value: `openscanner-site-verification=os_verify_9981a2f...`
2. **HTML Meta Tag:**
   - `<meta name="openscanner-site-verification" content="os_verify_9981a2f..." />`
   - Located within the `<head>` section of the homepage.

---

## 4. Legal Takedowns, DMCA & Right-to-be-Forgotten

- Open Scanner maintains an automated tombstone mechanism.
- Upon receipt of a verified DMCA notification or court-ordered privacy takedown, the URL or domain fingerprint is immediately committed to the `index.tombstones` ledger.
- The inverted index filter prevents subsequent display of tombstoned records with zero downtime.
