package crawler

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// Sitemap limits keep URL discovery bounded per host.
const (
	maxSitemapBytes   = 2 << 20
	maxSitemapFetches = 6 // sitemap.xml + robots-declared maps + one nested level
	maxSitemapURLs    = 500
)

// RobotsSitemapRefs extracts Sitemap: directives from a robots.txt body.
func RobotsSitemapRefs(robotsBody []byte) []string {
	var refs []string
	for _, line := range strings.Split(string(robotsBody), "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 9 || !strings.EqualFold(line[:8], "sitemap:") {
			continue
		}
		if ref := strings.TrimSpace(line[8:]); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

// SeedSitemaps discovers URLs from each allowlisted host's sitemap.xml (plus
// robots-declared maps) and enqueues same-host HTTPS URLs into the frontier
// (C-2). It returns the number of newly enqueued URLs. Fetch and parse
// failures are skipped per host: sitemaps are a discovery hint, never a gate.
func (c *BoundedCrawler) SeedSitemaps(ctx context.Context, hostnames []string) int {
	total := 0
	for _, host := range hostnames {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		total += c.seedHostSitemaps(ctx, "https://"+host, host)
	}
	return total
}

// seedHostSitemaps discovers URLs under base (normally https://host) and
// enqueues same-host HTTPS URLs. base is a parameter so tests can point it at
// a local TLS fixture.
func (c *BoundedCrawler) seedHostSitemaps(ctx context.Context, base, host string) int {
	if host == "" {
		return 0
	}
	candidates := []string{strings.TrimSuffix(base, "/") + "/sitemap.xml"}
	if robots := c.fetchText(ctx, strings.TrimSuffix(base, "/")+"/robots.txt"); robots != nil {
		for _, ref := range RobotsSitemapRefs(robots) {
			if sameHost(ref, host) {
				candidates = append(candidates, ref)
			}
		}
	}
	enqueued := 0
	fetches := 0
	seen := map[string]bool{}
	queue := candidates
	for len(queue) > 0 && fetches < maxSitemapFetches && enqueued < maxSitemapURLs {
		raw := queue[0]
		queue = queue[1:]
		if seen[raw] {
			continue
		}
		seen[raw] = true
		body := c.fetchText(ctx, raw)
		fetches++
		if body == nil {
			continue
		}
		entries, nested, err := ParseSitemapXML(body)
		if err != nil {
			continue
		}
		for _, loc := range nested {
			if sameSchemeHost(loc, "https", host) {
				queue = append(queue, loc)
			}
		}
		for _, entry := range entries {
			loc := strings.TrimSpace(entry.Loc)
			if loc == "" {
				continue
			}
			if enqueued >= maxSitemapURLs {
				break
			}
			if !sameSchemeHost(loc, "https", host) {
				continue
			}
			if _, err := c.frontier.SubmitURL(loc, 60); err == nil {
				enqueued++
			}
		}
	}
	return enqueued
}

func (c *BoundedCrawler) fetchText(ctx context.Context, target string) []byte {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := c.fetcher.Fetch(ctx, target, DefaultFetchOptions())
	if err != nil || res.StatusCode != 200 || len(res.DecompressedBody) == 0 {
		return nil
	}
	if len(res.DecompressedBody) > maxSitemapBytes {
		return nil
	}
	return res.DecompressedBody
}

func sameHost(raw, host string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), host)
}

func sameSchemeHost(raw, scheme, host string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, scheme) && strings.EqualFold(u.Hostname(), host)
}
