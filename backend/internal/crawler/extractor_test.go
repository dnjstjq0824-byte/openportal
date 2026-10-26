package crawler

import (
	"net/http"
	"testing"
)

func TestParseXRobotsTag(t *testing.T) {
	header := make(http.Header)
	header.Add("X-Robots-Tag", "noindex, nofollow")
	header.Add("X-Robots-Tag", "noarchive")

	dir := ParseXRobotsTag(header)
	if !dir.NoIndex {
		t.Errorf("expected NoIndex=true")
	}
	if !dir.NoFollow {
		t.Errorf("expected NoFollow=true")
	}
	if !dir.NoArchive {
		t.Errorf("expected NoArchive=true")
	}
	if dir.NoSnippet {
		t.Errorf("expected NoSnippet=false")
	}
}

func TestParseHTMLDirectivesAndLinks(t *testing.T) {
	htmlContent := `
<!DOCTYPE html>
<html>
<head>
  <title>OpenPortal Search Documentation</title>
  <meta name="robots" content="noindex, nosnippet">
  <meta name="description" content="Official documentation for OpenPortal search engine">
  <link rel="canonical" href="/official/docs/search">
</head>
<body>
  <h1>Documentation</h1>
  <p>Welcome to OpenPortal</p>
  <a href="/guide/intro" rel="noopener">Getting Started Guide</a>
  <a href="https://partner.example.com/login" rel="nofollow">Partner Login</a>
  <a href="javascript:void(0)">Ignored JS Link</a>
</body>
</html>
`

	baseURL := "https://example.com/temp/doc.html"
	dir, links, err := ParseHTMLDirectives([]byte(htmlContent), baseURL, DocumentDirectives{})
	if err != nil {
		t.Fatalf("ParseHTMLDirectives failed: %v", err)
	}

	if dir.Title != "OpenPortal Search Documentation" {
		t.Errorf("Title = %q, want %q", dir.Title, "OpenPortal Search Documentation")
	}
	if dir.Description != "Official documentation for OpenPortal search engine" {
		t.Errorf("Description = %q", dir.Description)
	}
	if !dir.NoIndex {
		t.Errorf("expected NoIndex=true from meta robots")
	}
	if !dir.NoSnippet {
		t.Errorf("expected NoSnippet=true from meta robots")
	}
	if dir.NoFollow {
		t.Errorf("expected NoFollow=false (only noindex and nosnippet in meta)")
	}

	// Canonical resolution
	expectedCanonical := "https://example.com/official/docs/search"
	if dir.Canonical != expectedCanonical {
		t.Errorf("Canonical = %q, want %q", dir.Canonical, expectedCanonical)
	}

	// Links extraction
	if len(links) != 2 {
		t.Fatalf("expected 2 discovered links, got %d", len(links))
	}

	if links[0].CanonicalURL != "https://example.com/guide/intro" {
		t.Errorf("link 0 CanonicalURL = %q", links[0].CanonicalURL)
	}
	if links[0].AnchorText != "Getting Started Guide" {
		t.Errorf("link 0 AnchorText = %q", links[0].AnchorText)
	}
	if links[0].NoFollow {
		t.Errorf("link 0 NoFollow should be false")
	}

	if links[1].CanonicalURL != "https://partner.example.com/login" {
		t.Errorf("link 1 CanonicalURL = %q", links[1].CanonicalURL)
	}
	if !links[1].NoFollow {
		t.Errorf("link 1 NoFollow should be true due to rel=\"nofollow\"")
	}
}

func TestParseSitemapXML(t *testing.T) {
	urlSetXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/page1</loc>
    <lastmod>2026-10-20</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>https://example.com/page2</loc>
    <lastmod>2026-10-21</lastmod>
  </url>
</urlset>`

	urls, _, err := ParseSitemapXML([]byte(urlSetXML))
	if err != nil {
		t.Fatalf("ParseSitemapXML urlset failed: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(urls))
	}
	if urls[0].Loc != "https://example.com/page1" || urls[0].Priority != 0.8 {
		t.Errorf("url 0 loc=%s priority=%f", urls[0].Loc, urls[0].Priority)
	}

	indexXML := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://example.com/sitemap1.xml</loc>
    <lastmod>2026-10-20</lastmod>
  </sitemap>
  <sitemap>
    <loc>https://example.com/sitemap2.xml</loc>
  </sitemap>
</sitemapindex>`

	_, sitemaps, err := ParseSitemapXML([]byte(indexXML))
	if err != nil {
		t.Fatalf("ParseSitemapXML index failed: %v", err)
	}
	if len(sitemaps) != 2 {
		t.Fatalf("expected 2 child sitemaps, got %d", len(sitemaps))
	}
	if sitemaps[0] != "https://example.com/sitemap1.xml" {
		t.Errorf("sitemap 0 loc=%s", sitemaps[0])
	}
}

