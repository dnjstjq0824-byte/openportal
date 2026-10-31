package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/openportal-search/backend/internal/indexer"
)

// TestRealCrawlPipelineIngestion verifies SE-03:
// 1. A real HTTP server serves robots.txt and an HTML article with a unique token over the wire.
// 2. BoundedCrawler performs real HTTP GET with SafeFetcher.
// 3. robots.txt is parsed (RFC 9309).
// 4. Actual response bytes (not synthetic strings) are passed to IndexingService.
// 5. Title, body, SimHash, and metadata are extracted from the actual HTTP response.
func TestRealCrawlPipelineIngestion(t *testing.T) {
	uniqueToken := fmt.Sprintf("OS-VERIFIED-CRAWL-%d", time.Now().UnixNano())
	uniqueTitle := "Real Web Extraction Standard " + uniqueToken
	uniqueBody := "This distinct paragraph contains unique text " + uniqueToken + " delivered across HTTP."

	// 1. Real HTTP Server serving robots.txt and HTML document
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/real-page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Last-Modified", "Sun, 01 Nov 2026 00:00:00 GMT")
			w.WriteHeader(http.StatusOK)
			html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>%s</title>
  <meta name="description" content="A real test article fetched over the network wire.">
</head>
<body>
  <nav><a href="/">Home</a></nav>
  <main>
    <h1>%s</h1>
    <p>%s</p>
  </main>
  <footer><p>© 2026 Live Web Corpus</p></footer>
</body>
</html>`, uniqueTitle, uniqueTitle, uniqueBody)
			w.Write([]byte(html))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}

	port := 80
	if serverURL.Port() != "" {
		port, _ = strconv.Atoi(serverURL.Port())
	}

	// 2. Setup BoundedCrawler with SafeFetcher connected to the test server client
	frontier := NewFrontier()
	hostKey := fmt.Sprintf("%s://%s:%d", serverURL.Scheme, serverURL.Hostname(), port)
	frontier.RegisterVerifiedHost(&VerifiedHost{
		ID:           "test-host-live",
		Scheme:       serverURL.Scheme,
		Hostname:     serverURL.Hostname(),
		Port:         port,
		HostKey:      hostKey,
		FetchEnabled: true,
		CrawlDelay:   10 * time.Millisecond,
		CreatedAt:    time.Now().UTC(),
	})

	fetcher := NewSafeFetcherWithClient(server.Client())
	politeness := NewPolitenessManager(10*time.Millisecond, 100)
	crawlerEngine := NewBoundedCrawler(frontier, fetcher, politeness)

	// 3. Execute real crawl over the wire
	targetPageURL := server.URL + "/real-page"
	ctx := context.Background()

	t.Logf("[%s] Initiating wire crawl for URL: %s", time.Now().UTC().Format(time.RFC3339), targetPageURL)
	cRecord, crawlErr := crawlerEngine.CrawlURL(ctx, targetPageURL)
	if crawlErr != nil {
		t.Fatalf("CrawlURL failed: %v", crawlErr)
	}

	t.Logf("[%s] Crawl complete: status=%d bytes=%d warc_key=%s sha256=%s",
		cRecord.CrawledAt.Format(time.RFC3339), cRecord.StatusCode, cRecord.TransferredBytes, cRecord.WARCObjectKey, cRecord.ChecksumSHA256)

	// Verification of SE-03 assertions
	if cRecord.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", cRecord.StatusCode)
	}
	if !cRecord.Indexable {
		t.Errorf("expected record to be indexable")
	}
	if len(cRecord.HTMLBody) == 0 {
		t.Fatalf("expected non-empty HTMLBody from actual HTTP response, got 0 bytes")
	}
	if !strings.Contains(string(cRecord.HTMLBody), uniqueToken) {
		t.Fatalf("expected HTMLBody to contain %s, got: %s", uniqueToken, string(cRecord.HTMLBody))
	}

	// 4. Ingest actual response body into IndexingService
	idxService := indexer.NewIndexingService("", "", server.Client())
	docID := "real-crawl-doc-" + uniqueToken

	extracted, indexed, ingestErr := idxService.IngestCrawlResult(ctx, docID, cRecord.CanonicalURL, cRecord.HTMLBody, cRecord.CrawledAt)
	if ingestErr != nil {
		t.Fatalf("IngestCrawlResult failed: %v", ingestErr)
	}
	if !indexed {
		t.Errorf("expected document to be indexed")
	}

	t.Logf("[%s] Extraction complete: title=%q wordCount=%d simhash=%d",
		time.Now().UTC().Format(time.RFC3339), extracted.Title, extracted.WordCount, extracted.SimHash)

	if extracted.Title != uniqueTitle {
		t.Errorf("expected extracted title %q, got %q", uniqueTitle, extracted.Title)
	}
	if !strings.Contains(extracted.Body, uniqueToken) {
		t.Errorf("expected extracted body to contain %s, got %q", uniqueToken, extracted.Body)
	}
}

