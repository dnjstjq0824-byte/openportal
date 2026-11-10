package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/crawler"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/indexer"
	_ "github.com/lib/pq"
	"golang.org/x/net/idna"
)

func env(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val != "" {
		return val
	}
	return fallback
}

func approvedHosts(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("CRAWL_ALLOWED_HOSTS is required; configure only domains with approved crawl rights")
	}
	pattern := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.(?:[a-z]{2,63}|xn--[a-z0-9-]{2,59})$`)
	seen := make(map[string]bool)
	var hosts []string
	for _, entry := range strings.Split(raw, ",") {
		ascii, err := idna.Lookup.ToASCII(strings.TrimSpace(entry))
		if err != nil {
			return nil, fmt.Errorf("crawl allowlist contains an invalid international hostname")
		}
		host := strings.ToLower(ascii)
		if len(host) > 253 || !pattern.MatchString(host) {
			return nil, fmt.Errorf("crawl allowlist contains an invalid hostname")
		}
		if !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	if len(hosts) > 100 {
		return nil, fmt.Errorf("crawl allowlist must contain at most 100 hosts")
	}
	return hosts, nil
}

func approvedTarget(raw string, hosts []string) error {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return fmt.Errorf("invalid crawl target")
	}
	cu, err := crawler.Canonicalize(raw)
	if err != nil || cu.Scheme != "https" || cu.Port != 443 {
		return fmt.Errorf("crawl target must use approved HTTPS port 443")
	}
	for _, host := range hosts {
		if cu.Hostname == host {
			return nil
		}
	}
	return fmt.Errorf("crawl target is outside the current operator allowlist")
}

func main() {
	opensearchURL := flag.String("opensearch", env("OPENSEARCH_URL", "http://127.0.0.1:9200"), "OpenSearch base URL")
	indexName := flag.String("index", env("OPENSEARCH_INDEX", "documents_v2"), "OpenSearch target index")
	databaseURL := flag.String("database", env("DATABASE_URL", ""), "PostgreSQL connection string")
	targetURL := flag.String("url", "", "Single URL to crawl directly and exit")
	workerMode := flag.Bool("worker", false, "Run in continuous frontier worker loop")
	pageLimit := flag.Int("max-pages", 20, "Maximum URLs processed per run (1-10000), including worker mode")
	crawlDelay := flag.Duration("delay", 500*time.Millisecond, "Per-host crawl delay")
	allowedHosts := flag.String("allow-hosts", env("CRAWL_ALLOWED_HOSTS", ""), "Comma-separated hostnames with operator-approved crawl rights")
	warcDir := flag.String("warc-dir", env("WARC_DIR", ""), "Local directory for WARC response archives (empty disables archiving)")
	flag.Parse()
	hostnames, err := approvedHosts(*allowedHosts)
	if err != nil {
		log.Fatal(err)
	}
	if *pageLimit < 1 || *pageLimit > 10000 {
		log.Fatal("max-pages must be between 1 and 10000")
	}
	if *targetURL != "" {
		if err := approvedTarget(*targetURL, hostnames); err != nil {
			log.Fatal(err)
		}
	}

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
		log.Printf("[Crawler] PostgreSQL frontier connection configured")
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
	if *warcDir != "" {
		store, err := crawler.NewFileWARCStore(*warcDir)
		if err != nil {
			log.Fatalf("[Crawler] WARC archive unavailable: %v", err)
		}
		bCrawler.SetWARCStore(store)
		log.Printf("[Crawler] WARC archiving to %s", *warcDir)
	}

	// Public standards sites are not implicitly authorized. Operators must
	// explicitly approve each host; robots and SSRF checks still apply.
	for _, hostname := range hostnames {
		frontier.RegisterVerifiedHost(&crawler.VerifiedHost{
			ID:           fmt.Sprintf("host-%s", hostname),
			Scheme:       "https",
			Hostname:     hostname,
			Port:         443,
			HostKey:      fmt.Sprintf("https://%s:443", hostname),
			FetchEnabled: true,
			CrawlDelay:   *crawlDelay,
			CreatedAt:    time.Now().UTC(),
		})
	}

	idxService := indexer.NewIndexingService(*opensearchURL, *indexName, &http.Client{Timeout: 10 * time.Second})
	// C-3: with PostgreSQL configured, indexing intent goes through the durable
	// outbox and this loop relays it to OpenSearch in bulk.
	var relay *indexer.Relay
	durable := db != nil
	if durable {
		relay = idxService.EnableDurableOutbox(db)
		log.Printf("[Crawler] Durable outbox relay enabled (catalog.index_events)")
	}
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
		_, submitErr := frontier.SubmitURL(*targetURL, 100)
		if submitErr != nil && submitErr != crawler.ErrFrontierURLAlreadyExists {
			log.Printf("[Crawler] SubmitURL error: %v", submitErr)
		}

		log.Printf("[Crawler] Executing real HTTP crawl for target URL: %s", *targetURL)
		crawlAndIndex(ctx, bCrawler, idxService, frontier, workerID, *targetURL, 1, durable)
		drainRelay(ctx, relay)
		return
	}

	// Default seed crawl if not in daemon mode
	for _, hostname := range hostnames {
		_, _ = frontier.SubmitURL("https://"+hostname+"/", 80)
	}

	// C-2: sitemap-driven URL discovery per allowlisted host (best-effort).
	if found := bCrawler.SeedSitemaps(ctx, hostnames); found > 0 {
		log.Printf("[Crawler] Sitemap discovery enqueued %d URLs", found)
	}

	// C-1: freshness pass — re-queue completions older than 7 days.
	rescheduleCutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	if revived, err := frontier.RescheduleStale(rescheduleCutoff, 1000); err != nil {
		log.Printf("[Crawler] Freshness reschedule failed: %v", err)
	} else if revived > 0 {
		log.Printf("[Crawler] Freshness reschedule re-queued %d URLs", revived)
	}
	lastReschedule := time.Now().UTC()

	runCount := 0
	maxPages := *pageLimit

	for runCount < maxPages {
		select {
		case <-ctx.Done():
			log.Printf("[Crawler] Worker loop stopped.")
			return
		default:
		}

		batchSize := min(5, maxPages-runCount)
		items := frontier.LeaseAcquire(workerID, batchSize, 60*time.Second)
		if len(items) == 0 {
			if !*workerMode {
				log.Printf("[Crawler] Queue is empty. Single pass complete.")
				break
			}
			// C-1: periodic freshness pass while idling in worker mode.
			if time.Since(lastReschedule) > 10*time.Minute {
				if revived, err := frontier.RescheduleStale(time.Now().UTC().Add(-7*24*time.Hour), 1000); err != nil {
					log.Printf("[Crawler] Freshness reschedule failed: %v", err)
				} else if revived > 0 {
					log.Printf("[Crawler] Freshness reschedule re-queued %d URLs", revived)
				}
				lastReschedule = time.Now().UTC()
			}
			time.Sleep(2 * time.Second)
			continue
		}

		for _, item := range items {
			if ctx.Err() != nil {
				break
			}
			runCount++
			if err := approvedTarget(item.URL, hostnames); err != nil {
				_ = frontier.Block(item.URLHash, "outside current operator allowlist")
				continue
			}
			crawlAndIndex(ctx, bCrawler, idxService, frontier, workerID, item.URL, runCount, durable)
		}
		drainRelay(ctx, relay)
	}

	log.Printf("[Crawler] Finished processing. Total crawled in this session: %d", runCount)
}

// drainRelay delivers one durable-outbox batch; nil relay (memory mode) is a no-op.
func drainRelay(ctx context.Context, relay *indexer.Relay) {
	if relay == nil {
		return
	}
	upserts, deletes, err := relay.RunOnce(ctx)
	if err != nil {
		log.Printf("[Crawler] Outbox relay: %v (events stay queued for retry)", err)
		return
	}
	if upserts+deletes > 0 {
		log.Printf("[Crawler] Outbox relay delivered upserts=%d deletes=%d", upserts, deletes)
	}
}

func crawlAndIndex(ctx context.Context, bCrawler *crawler.BoundedCrawler, idxService *indexer.IndexingService, frontier crawler.FrontierQueue, workerID, rawURL string, seq int, durable bool) {
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
		verb := "INDEXED"
		if durable {
			verb = "QUEUED"
		}
		log.Printf("[Crawler] [%d] [%s] docID=%s title=%q lang=%s words=%d links=%d elapsed=%s",
			seq, verb, docID, extracted.Title, extracted.Language, extracted.WordCount, cRecord.DiscoveredLinks, elapsed)
		_ = frontier.Ack(cu.URLHash, workerID)
	} else {
		log.Printf("[Crawler] [%d] [SKIP] Unchanged or near-duplicate docID=%s elapsed=%s", seq, docID, elapsed)
		_ = frontier.Ack(cu.URLHash, workerID)
	}
}
