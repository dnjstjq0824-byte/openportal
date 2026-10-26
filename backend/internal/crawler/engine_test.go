package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBoundedCrawlerEngine(t *testing.T) {
	// Mock HTTP server acting as an authorized website
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`
User-agent: *
Disallow: /admin/
Disallow: /private/
Allow: /
`))

		case "/page1":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
  <title>Approved Page 1</title>
  <meta name="description" content="Test document">
</head>
<body>
  <h1>Welcome</h1>
  <a href="/page2">Go to Page 2</a>
  <a href="/private/secret">Secret link</a>
</body>
</html>
`))

		case "/page2":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Robots-Tag", "noindex")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><h1>NoIndex Page</h1></body></html>`))

		case "/admin/dashboard":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><h1>Admin Dashboard</h1></body></html>`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cu, _ := Canonicalize(server.URL + "/page1")

	frontier := NewFrontier()
	politeness := NewPolitenessManager(10*time.Millisecond, 100)
	fetcher := &SafeFetcher{client: server.Client()}

	engine := NewBoundedCrawler(frontier, fetcher, politeness)
	ctx := context.Background()

	// 1. Unregistered arbitrary host crawl attempt MUST be rejected
	_, err := engine.CrawlURL(ctx, "https://arbitrary-unapproved-site.com/index.html")
	if err == nil || !strings.Contains(err.Error(), "arbitrary crawling denied") {
		t.Errorf("expected ErrArbitraryCrawlDenied, got: %v", err)
	}

	// 2. Register authorized test server host
	host := &VerifiedHost{
		ID:           "test-host-01",
		Scheme:       cu.Scheme,
		Hostname:     cu.Hostname,
		Port:         cu.Port,
		HostKey:      cu.HostKey,
		FetchEnabled: true,
		CrawlDelay:   10 * time.Millisecond,
	}
	frontier.RegisterVerifiedHost(host)

	// 3. Attempting to crawl a robots.txt-disallowed path should be blocked
	adminURL := fmt.Sprintf("%s/admin/dashboard", server.URL)
	_, err = engine.CrawlURL(ctx, adminURL)
	if err == nil || !strings.Contains(err.Error(), "disallowed by robots.txt") {
		t.Errorf("expected robots disallow error for /admin/dashboard, got: %v", err)
	}

	// 4. Crawl allowed page 1
	rec1, err := engine.CrawlURL(ctx, server.URL+"/page1")
	if err != nil {
		t.Fatalf("CrawlURL page1 failed: %v", err)
	}

	if !rec1.Indexable {
		t.Errorf("page1 should be Indexable=true")
	}
	if rec1.DiscoveredLinks != 2 {
		t.Errorf("expected 2 discovered links from page1, got %d", rec1.DiscoveredLinks)
	}
	if rec1.ChecksumSHA256 == "" || len(rec1.ChecksumSHA256) != 64 {
		t.Errorf("invalid ChecksumSHA256: %s", rec1.ChecksumSHA256)
	}
	if !strings.HasPrefix(rec1.WARCObjectKey, "raw/") {
		t.Errorf("invalid WARCObjectKey: %s", rec1.WARCObjectKey)
	}

	// 5. Crawl page 2 (has X-Robots-Tag: noindex)
	rec2, err := engine.CrawlURL(ctx, server.URL+"/page2")
	if err != nil {
		t.Fatalf("CrawlURL page2 failed: %v", err)
	}
	if rec2.Indexable {
		t.Errorf("page2 should be Indexable=false due to noindex")
	}

	// 6. Verify Compliance Report
	rep := engine.GetReport()
	if rep.TotalRequests != 2 { // page1 + page2 (admin was blocked before fetch)
		t.Errorf("TotalRequests = %d, want 2", rep.TotalRequests)
	}
	if rep.RobotsBlockedCount != 1 {
		t.Errorf("RobotsBlockedCount = %d, want 1", rep.RobotsBlockedCount)
	}
	if rep.RobotsAllowedCount != 2 {
		t.Errorf("RobotsAllowedCount = %d, want 2 (page1 and page2)", rep.RobotsAllowedCount)
	}
	if rep.NoIndexCount != 1 {
		t.Errorf("NoIndexCount = %d, want 1", rep.NoIndexCount)
	}
	if rep.TotalBytes <= 0 {
		t.Errorf("TotalBytes should be > 0, got %d", rep.TotalBytes)
	}
	if rep.HostTraffic[cu.HostKey] <= 0 {
		t.Errorf("HostTraffic for %s should be > 0", cu.HostKey)
	}
}
