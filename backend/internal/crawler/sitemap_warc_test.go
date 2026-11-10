package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRobotsSitemapRefs(t *testing.T) {
	refs := RobotsSitemapRefs([]byte("User-agent: *\nAllow: /\nSitemap: https://example.test/sm.xml\nsitemap: https://example.test/other.xml\n"))
	if len(refs) != 2 || refs[0] != "https://example.test/sm.xml" {
		t.Fatalf("refs=%v", refs)
	}
}

// C-2: sitemap discovery enqueues same-host URLs and ignores foreign ones.
func TestSeedHostSitemaps(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\nSitemap: https://127.0.0.1/extra.xml\n"))
		case "/sitemap.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://127.0.0.1/page-a</loc></url><url><loc>https://evil.test/stolen</loc></url></urlset>`))
		case "/extra.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://127.0.0.1/page-b</loc></url></urlset>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	frontier := NewFrontier()
	// Sitemap locs carry the bare host (implicit 443); the fetch base still
	// points at the fixture's test port.
	frontier.RegisterVerifiedHost(&VerifiedHost{
		ID: "test-sitemap-host", Scheme: "https", Hostname: "127.0.0.1", Port: 443,
		HostKey:      "https://127.0.0.1:443",
		FetchEnabled: true, CrawlDelay: 10 * time.Millisecond, CreatedAt: time.Now().UTC(),
	})
	engine := NewBoundedCrawler(frontier, NewSafeFetcherWithClient(server.Client()), NewPolitenessManager(10*time.Millisecond, 100))
	// robots.txt advertises /extra.xml with a bare host (no test port), so only
	// /sitemap.xml's same-host URL is discoverable in this fixture.
	found := engine.seedHostSitemaps(context.Background(), server.URL, "127.0.0.1")
	if found != 1 {
		t.Fatalf("expected 1 enqueued URL, got %d", found)
	}
	items := frontier.LeaseAcquire("tester", 10, time.Minute)
	if len(items) != 1 || !strings.HasSuffix(items[0].URL, "/page-a") {
		t.Fatalf("frontier=%v", items)
	}
}

// C-2: WARC records persist under the partitioned object key and stay parseable.
func TestFileWARCStoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileWARCStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	header := http.Header{"Content-Type": []string{"text/html"}}
	record := CreateWARCResponseRecord("https://example.test/x", strings.Repeat("ab", 32), 200, header, []byte("<html></html>"), time.Now().UTC())
	if err := store.Write(record); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, record.ObjectKey))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseWARCRecordHeader(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed["warc-target-uri"] != "https://example.test/x" {
		t.Fatalf("headers=%v", parsed)
	}
	if err := store.Write(&WARCRecord{ObjectKey: "../../escape.warc", RawData: []byte("x")}); err == nil {
		t.Fatal("expected path-escape rejection")
	}
}

// C-2 red drill: archive failures are counted, never fatal to the crawl.
func TestWARCFailureIsCounted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>WARC drill</title></head><body><p>archive failure drill body</p></body></html>`))
	}))
	defer server.Close()
	frontier := registerLoopbackHost(t, server.URL)
	engine := NewBoundedCrawler(frontier, NewSafeFetcherWithClient(server.Client()), NewPolitenessManager(10*time.Millisecond, 100))
	engine.SetWARCStore(failingWARCStore{})
	record, err := engine.CrawlURL(context.Background(), server.URL+"/drill")
	if err != nil {
		t.Fatalf("crawl must succeed despite archive failure: %v", err)
	}
	if record == nil || !record.Indexable {
		t.Fatalf("record=%+v", record)
	}
	if engine.WARCErrors() != 1 {
		t.Fatalf("expected 1 counted archive failure, got %d", engine.WARCErrors())
	}
}

// C-2: a working store receives the fetched record during a live crawl.
func TestWARCStoredOnCrawl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Stored</title></head><body><p>stored body</p></body></html>`))
	}))
	defer server.Close()
	store, err := NewFileWARCStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	frontier := registerLoopbackHost(t, server.URL)
	engine := NewBoundedCrawler(frontier, NewSafeFetcherWithClient(server.Client()), NewPolitenessManager(10*time.Millisecond, 100))
	engine.SetWARCStore(store)
	record, err := engine.CrawlURL(context.Background(), server.URL+"/stored")
	if err != nil {
		t.Fatal(err)
	}
	if engine.WARCErrors() != 0 {
		t.Fatalf("errors=%d", engine.WARCErrors())
	}
	if record.WARCObjectKey == "" {
		t.Fatal("expected WARC object key on record")
	}
}

type failingWARCStore struct{}

func (failingWARCStore) Write(*WARCRecord) error { return context.DeadlineExceeded }

func registerLoopbackHost(t *testing.T, rawURL string) *Frontier {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(parsed.Port())
	frontier := NewFrontier()
	frontier.RegisterVerifiedHost(&VerifiedHost{
		ID: "test-loopback-host", Scheme: parsed.Scheme, Hostname: parsed.Hostname(), Port: port,
		HostKey:      fmt.Sprintf("%s://%s:%d", parsed.Scheme, parsed.Hostname(), port),
		FetchEnabled: true, CrawlDelay: 10 * time.Millisecond, CreatedAt: time.Now().UTC(),
	})
	return frontier
}
