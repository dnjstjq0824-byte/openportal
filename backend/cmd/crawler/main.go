package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"example.com/openportal-search/backend/internal/crawler"
	"example.com/openportal-search/backend/internal/indexer"
	_ "github.com/lib/pq"
)

func env(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val != "" {
		return val
	}
	return fallback
}

func main() {
	opensearchURL := flag.String("opensearch", env("OPENSEARCH_URL", "http://127.0.0.1:9200"), "OpenSearch base URL")
	indexName := flag.String("index", env("OPENSEARCH_INDEX", "documents_v2"), "OpenSearch target index")
	databaseURL := flag.String("database", env("DATABASE_URL", ""), "PostgreSQL connection string")
	targetURL := flag.String("url", "", "Single URL to crawl directly and exit")
	workerMode := flag.Bool("worker", false, "Run in continuous frontier worker loop")
	crawlDelay := flag.Duration("delay", 500*time.Millisecond, "Per-host crawl delay")
	flag.Parse()

	log.Printf("[Crawler] Starting OpenPortal Real Web Crawler & Ingestion Pipeline...")
	log.Printf("[Crawler] Target OpenSearch: %s, Index: %s", *opensearchURL, *indexName)

	var db *sql.DB
	if *databaseURL != "" {
		var err error
		db, err = sql.Open("postgres", *databaseURL)
		if err != nil {
			log.Fatalf("[Crawler] Failed to connect to PostgreSQL: %v", err)
		}
		defer db.Close()
		log.Printf("[Crawler] Connected to PostgreSQL at %s", *databaseURL)
	}

	var frontier crawler.FrontierQueue
	if db != nil {
		frontier = crawler.NewPostgresFrontier(db)
		log.Printf("[Crawler] Frontier initialized with PostgreSQL persistence (crawl.frontier, crawl.hosts)")
	} else {
		frontier = crawler.NewFrontier()
		log.Printf("[Crawler] Frontier initialized with in-memory queue (no DATABASE_URL provided)")
	}

	politeness := crawler.NewPolitenessManager(*crawlDelay, 3600)
	fetcher := crawler.NewSafeFetcher(crawler.NewSSRFGuard())
	bCrawler := crawler.NewBoundedCrawler(frontier, fetcher, politeness)

	// Register approved seed domains
	authorizedHosts := []struct {
		scheme   string
		hostname string
		port     int
	}{
		{"https", "rfc-editor.org", 443},
		{"https", "www.rfc-editor.org", 443},
		{"https", "w3.org", 443},
		{"https", "www.w3.org", 443},
		{"https", "iana.org", 443},
		{"https", "www.iana.org", 443},
		{"https", "developer.mozilla.org", 443},
		{"https", "ko.wikipedia.org", 443},
		{"https", "en.wikipedia.org", 443},
	}

	for _, h := range authorizedHosts {
		frontier.RegisterVerifiedHost(&crawler.VerifiedHost{
			ID:           fmt.Sprintf("host-%s", h.hostname),
			Scheme:       h.scheme,
			Hostname:     h.hostname,
			Port:         h.port,
			HostKey:      fmt.Sprintf("%s://%s:%d", h.scheme, h.hostname, h.port),
			FetchEnabled: true,
			CrawlDelay:   *crawlDelay,
			CreatedAt:    time.Now().UTC(),
		})
	}

	idxService := indexer.NewIndexingService(*opensearchURL, *indexName, &http.Client{Timeout: 10 * time.Second})
	workerID := fmt.Sprintf("crawler-worker-%d", os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Printf("[Crawler] Shutdown signal received, terminating gracefully...")
		cancel()
	}()

	// If a specific URL was requested, enqueue and crawl it directly
	if *targetURL != "" {
		cu, err := crawler.Canonicalize(*targetURL)
		if err != nil {
			log.Fatalf("[Crawler] Invalid target URL %q: %v", *targetURL, err)
		}

		// Register host dynamically if allowed
		frontier.RegisterVerifiedHost(&crawler.VerifiedHost{
			ID:           fmt.Sprintf("host-%s", cu.Hostname),
			Scheme:       cu.Scheme,
			Hostname:     cu.Hostname,
			Port:         cu.Port,
			HostKey:      cu.HostKey,
			FetchEnabled: true,
			CrawlDelay:   *crawlDelay,
			CreatedAt:    time.Now().UTC(),
		})

		_, submitErr := frontier.SubmitURL(*targetURL, 100)
		if submitErr != nil && submitErr != crawler.ErrFrontierURLAlreadyExists {
			log.Printf("[Crawler] SubmitURL error: %v", submitErr)
		}

		log.Printf("[Crawler] Executing real HTTP crawl for target URL: %s", *targetURL)
		crawlAndIndex(ctx, bCrawler, idxService, frontier, workerID, *targetURL, 1)
		return
	}

	// Default seed crawl if not in daemon mode
	initialSeeds := []string{
		"https://www.rfc-editor.org/rfc/rfc9309.html", // Robots Exclusion Protocol RFC
		"https://www.iana.org/domains/reserved",
	}

	for _, seedURL := range initialSeeds {
		_, _ = frontier.SubmitURL(seedURL, 80)
	}

	runCount := 0
	maxPages := 20
	if *workerMode {
		maxPages = 10000000 // effectively unbounded
	}

	for runCount < maxPages {
		select {
		case <-ctx.Done():
			log.Printf("[Crawler] Worker loop stopped.")
			return
		default:
		}

		items := frontier.LeaseAcquire(workerID, 5, 60*time.Second)
		if len(items) == 0 {
			if !*workerMode {
				log.Printf("[Crawler] Queue is empty. Single pass complete.")
				break
			}
			time.Sleep(2 * time.Second)
			continue
		}

		for _, item := range items {
			if ctx.Err() != nil {
				break
			}
			runCount++
			crawlAndIndex(ctx, bCrawler, idxService, frontier, workerID, item.URL, runCount)
		}
	}

	log.Printf("[Crawler] Finished processing. Total crawled in this session: %d", runCount)
}

func crawlAndIndex(ctx context.Context, bCrawler *crawler.BoundedCrawler, idxService *indexer.IndexingService, frontier crawler.FrontierQueue, workerID, rawURL string, seq int) {
	cu, err := crawler.Canonicalize(rawURL)
	if err != nil {
		log.Printf("[Crawler] [FAIL] Canonicalize %s: %v", rawURL, err)
		return
	}

	startTime := time.Now().UTC()
	log.Printf("[Crawler] [%d] Fetching over wire: %s (host=%s)", seq, cu.Canonical, cu.HostKey)

	// 1. Execute actual HTTP fetch through SafeFetcher with SSRF guard, RFC 9309 robots check, and politeness delay
	cRecord, crawlErr := bCrawler.CrawlURL(ctx, cu.Canonical)
	if crawlErr != nil {
		log.Printf("[Crawler] [%d] [FAIL] %s: %v", seq, cu.Canonical, crawlErr)
		_ = frontier.Nack(cu.URLHash, workerID, crawlErr.Error())
		return
	}

	// 2. Fail-closed: Never index empty or unindexable bodies
	if !cRecord.Indexable || len(cRecord.HTMLBody) == 0 {
		log.Printf("[Crawler] [%d] [SKIP] Non-indexable or empty response for %s (status=%d, indexable=%v, transferred=%d bytes)", seq, cu.Canonical, cRecord.StatusCode, cRecord.Indexable, cRecord.TransferredBytes)
		_ = frontier.Ack(cu.URLHash, workerID)
		return
	}

	// 3. Ingest the REAL decompressed HTML body received over the wire
	docID := fmt.Sprintf("crawl-%s", cu.URLHash[:16])
	extracted, indexed, err := idxService.IngestCrawlResult(ctx, docID, cRecord.CanonicalURL, cRecord.HTMLBody, cRecord.CrawledAt)
	if err != nil {
		log.Printf("[Crawler] [%d] [ERROR] Ingestion to OpenSearch failed for %s: %v", seq, cRecord.CanonicalURL, err)
		_ = frontier.Nack(cu.URLHash, workerID, err.Error())
		return
	}

	elapsed := time.Since(startTime)
	if indexed {
		log.Printf("[Crawler] [%d] [INDEXED] docID=%s title=%q lang=%s words=%d links=%d elapsed=%s",
			seq, docID, extracted.Title, extracted.Language, extracted.WordCount, cRecord.DiscoveredLinks, elapsed)
		_ = frontier.Ack(cu.URLHash, workerID)
	} else {
		log.Printf("[Crawler] [%d] [SKIP] Unchanged or near-duplicate docID=%s elapsed=%s", seq, docID, elapsed)
		_ = frontier.Ack(cu.URLHash, workerID)
	}
}
