package crawler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrArbitraryCrawlDenied = errors.New("engine: arbitrary crawling denied; host is not in the approved bounded corpus")
	ErrRobotsDisallowed     = errors.New("engine: url is disallowed by robots.txt policy")
	ErrPageNoIndex          = errors.New("engine: page specifies noindex directive")
)

// CrawlRecord represents the full processed audit record of a crawled URL.
type CrawlRecord struct {
	HTMLBody         []byte             `json:"-"` // Returned only for indexable HTML; never retained in audit records.
	URL              string             `json:"url"`
	CanonicalURL     string             `json:"canonical_url"`
	URLHash          string             `json:"url_hash"`
	HostKey          string             `json:"host_key"`
	StatusCode       int                `json:"status_code"`
	Directives       DocumentDirectives `json:"directives"`
	WARCObjectKey    string             `json:"warc_object_key"`
	ChecksumSHA256   string             `json:"checksum_sha256"`
	DiscoveredLinks  int                `json:"discovered_links"`
	TransferredBytes int64              `json:"transferred_bytes"`
	CrawledAt        time.Time          `json:"crawled_at"`
	Indexable        bool               `json:"indexable"`
	FailureReason    string             `json:"failure_reason,omitempty"`
}

// ComplianceReport aggregates traffic, robots adherence, and deletion metrics for Task 030 release gate.
type ComplianceReport struct {
	TotalRequests      int64            `json:"total_requests"`
	TotalBytes         int64            `json:"total_bytes"`
	RobotsAllowedCount int64            `json:"robots_allowed_count"`
	RobotsBlockedCount int64            `json:"robots_blocked_count"`
	NoIndexCount       int64            `json:"noindex_count"`
	HTTP2xxCount       int64            `json:"http_2xx_count"`
	HTTP3xxCount       int64            `json:"http_3xx_count"`
	HTTP4xxCount       int64            `json:"http_4xx_count"`
	HTTP5xxCount       int64            `json:"http_5xx_count"`
	HostTraffic        map[string]int64 `json:"host_traffic"` // hostKey -> bytes
}

// BoundedCrawler coordinates the full Phase 2 crawling lifecycle under strict compliance gates.
type BoundedCrawler struct {
	mu          sync.RWMutex
	frontier    *Frontier
	robotsCache *RobotsCache
	politeness  *PolitenessManager
	fetcher     *SafeFetcher
	ssrfGuard   *SSRFGuard
	userAgents  string
	records     map[string]*CrawlRecord
	report      ComplianceReport
}

func NewBoundedCrawler(frontier *Frontier, fetcher *SafeFetcher, politeness *PolitenessManager) *BoundedCrawler {
	if frontier == nil {
		frontier = NewFrontier()
	}
	if fetcher == nil {
		fetcher = NewSafeFetcher(NewSSRFGuard())
	}
	if politeness == nil {
		politeness = NewPolitenessManager(1*time.Second, 3600)
	}
	return &BoundedCrawler{
		frontier:    frontier,
		robotsCache: NewRobotsCache(),
		politeness:  politeness,
		fetcher:     fetcher,
		ssrfGuard:   NewSSRFGuard(),
		userAgents:  DefaultCrawlerUserAgent,
		records:     make(map[string]*CrawlRecord),
		report: ComplianceReport{
			HostTraffic: make(map[string]int64),
		},
	}
}

// CrawlURL executes the compliance-checked, bounded crawl pipeline for a single target URL.
func (c *BoundedCrawler) CrawlURL(ctx context.Context, rawURL string) (*CrawlRecord, error) {
	opts := DefaultFetchOptions()
	// Redirect targets require their own authorization and robots checks.
	// Until that traversal is implemented, do not request them automatically.
	opts.RejectRedirects = true
	cu, err := Canonicalize(rawURL)
	if err != nil {
		return nil, err
	}

	// 1. Task 030 Bounded Corpus Gate: Verify host authorization
	host, allowed := c.frontier.IsHostAllowed(cu.HostKey)
	if !allowed || host == nil {
		return nil, fmt.Errorf("%w: %s", ErrArbitraryCrawlDenied, cu.HostKey)
	}

	// 2. Task 024 Robots.txt Check (RFC 9309)
	robotsPolicy, found := c.robotsCache.Get(cu.HostKey)
	if !found {
		robotsURL, rErr := RobotsURL(cu.Canonical)
		if rErr == nil {
			// Fetch robots.txt with safe fetcher
			rRes, fErr := c.fetcher.Fetch(ctx, robotsURL, opts)
			if fErr == nil {
				robotsPolicy = ParseRobotsTXT(string(rRes.DecompressedBody), rRes.StatusCode)
			} else {
				// Network error / unreachable -> treat as 503 fail-closed
				robotsPolicy = ParseRobotsTXT("", 503)
			}
			c.robotsCache.Set(cu.HostKey, robotsPolicy)
		}
	}

	robotsPath := cu.Path
	if cu.Query != "" {
		robotsPath += "?" + cu.Query
	}
	if robotsPolicy == nil || !robotsPolicy.IsAllowed(robotsPath, c.userAgents) {
		c.mu.Lock()
		c.report.RobotsBlockedCount++
		c.mu.Unlock()
		_ = c.frontier.Block(cu.URLHash, "robots.txt disallow")
		return nil, fmt.Errorf("%w: path %s is forbidden by %s", ErrRobotsDisallowed, cu.Path, cu.HostKey)
	}

	c.mu.Lock()
	c.report.RobotsAllowedCount++
	c.mu.Unlock()

	// 3. Task 026 Politeness & Concurrency acquire
	customDelay := time.Duration(0)
	if robotsPolicy != nil {
		customDelay = robotsPolicy.GetCrawlDelay(c.userAgents)
	}
	if host.CrawlDelay > customDelay {
		customDelay = host.CrawlDelay
	}

	err = c.politeness.Acquire(ctx, cu.HostKey, customDelay)
	if err != nil {
		return nil, err
	}

	// 4. Task 025 & Task 027 Safe Fetch
	fetchRes, fetchErr := c.fetcher.Fetch(ctx, cu.Canonical, opts)
	statusCode := 0
	if fetchRes != nil {
		statusCode = fetchRes.StatusCode
	}
	c.politeness.Release(cu.HostKey, statusCode, fetchErr != nil)

	if fetchErr != nil {
		return nil, fmt.Errorf("crawl fetch error: %w", fetchErr)
	}

	now := time.Now().UTC()

	// 5. Update compliance traffic report
	c.mu.Lock()
	c.report.TotalRequests++
	c.report.TotalBytes += fetchRes.TransferredBytes
	c.report.HostTraffic[cu.HostKey] += fetchRes.TransferredBytes
	switch {
	case fetchRes.StatusCode >= 200 && fetchRes.StatusCode < 300:
		c.report.HTTP2xxCount++
	case fetchRes.StatusCode >= 300 && fetchRes.StatusCode < 400:
		c.report.HTTP3xxCount++
	case fetchRes.StatusCode >= 400 && fetchRes.StatusCode < 500:
		c.report.HTTP4xxCount++
	case fetchRes.StatusCode >= 500:
		c.report.HTTP5xxCount++
	}
	c.mu.Unlock()

	// 6. Task 028 WARC Serialization & SHA-256 Digest
	warcRecord := CreateWARCResponseRecord(cu.Canonical, cu.URLHash, fetchRes.StatusCode, fetchRes.Header, fetchRes.DecompressedBody, now)

	// 7. Task 029 Directives & Link Extraction
	headerDirectives := ParseXRobotsTag(fetchRes.Header)
	directives, discoveredLinks, _ := ParseHTMLDirectives(fetchRes.DecompressedBody, cu.Canonical, headerDirectives)

	isIndexable := true
	if directives.NoIndex || fetchRes.StatusCode != 200 || (fetchRes.ContentType != "text/html" && fetchRes.ContentType != "application/xhtml+xml") {
		isIndexable = false
	}
	if directives.NoIndex {
		c.mu.Lock()
		c.report.NoIndexCount++
		c.mu.Unlock()
	}

	// Discovered links enqueued to frontier if followable
	newLinksCount := 0
	if !directives.NoFollow {
		for _, link := range discoveredLinks {
			if !link.NoFollow {
				_, submitErr := c.frontier.SubmitURL(link.CanonicalURL, 20)
				if submitErr == nil {
					newLinksCount++
				}
			}
		}
	}

	record := &CrawlRecord{
		URL:              cu.Original,
		CanonicalURL:     cu.Canonical,
		URLHash:          cu.URLHash,
		HostKey:          cu.HostKey,
		StatusCode:       fetchRes.StatusCode,
		Directives:       directives,
		WARCObjectKey:    warcRecord.ObjectKey,
		ChecksumSHA256:   warcRecord.ChecksumSHA,
		DiscoveredLinks:  len(discoveredLinks),
		TransferredBytes: fetchRes.TransferredBytes,
		CrawledAt:        now,
		Indexable:        isIndexable,
	}

	c.mu.Lock()
	c.records[cu.URLHash] = record
	c.mu.Unlock()

	if isIndexable {
		// Return the fetched bytes without retaining page content in the audit map.
		result := *record
		result.HTMLBody = fetchRes.DecompressedBody
		return &result, nil
	}
	return record, nil
}

// GetReport returns a snapshot of the current crawler compliance report.
func (c *BoundedCrawler) GetReport() ComplianceReport {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rep := c.report
	rep.HostTraffic = make(map[string]int64, len(c.report.HostTraffic))
	for k, v := range c.report.HostTraffic {
		rep.HostTraffic[k] = v
	}
	return rep
}

// GetRecord retrieves a processed crawl record by URL hash.
func (c *BoundedCrawler) GetRecord(urlHash string) (*CrawlRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.records[urlHash]
	return rec, ok
}

