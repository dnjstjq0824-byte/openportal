package indexer

import (
	"net/url"
	"sort"
	"strings"
	"time"
)

// DocCandidate represents a candidate document in a deduplication cluster.
type DocCandidate struct {
	ID             string
	URL            string
	CanonicalURL   string
	SimHash        uint64
	ContentHash    string
	CrawledAt      time.Time
	ContentLength  int
	DomainPriority int
}

// ClusterResult holds deduplication cluster decisions.
type ClusterResult struct {
	Representative *DocCandidate
	Aliases        []*DocCandidate
}

// SelectRepresentative picks the primary representative document from a cluster
// of near-duplicates based on deterministic criteria:
// 1. Explicit rel="canonical" match to own URL
// 2. HTTPS over HTTP
// 3. Shorter, cleaner path without tracking queries
// 4. Higher domain priority
// 5. Most recent crawl date
// 6. Lexicographical URL tie-break
func SelectRepresentative(cluster []*DocCandidate) *ClusterResult {
	if len(cluster) == 0 {
		return &ClusterResult{}
	}
	if len(cluster) == 1 {
		return &ClusterResult{Representative: cluster[0]}
	}

	// Sort candidates in-place using deterministic ranking
	candidates := make([]*DocCandidate, len(cluster))
	copy(candidates, cluster)

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]

		// 1. Matches own canonical
		aIsCanon := a.CanonicalURL != "" && (a.CanonicalURL == a.URL)
		bIsCanon := b.CanonicalURL != "" && (b.CanonicalURL == b.URL)
		if aIsCanon != bIsCanon {
			return aIsCanon
		}

		// 2. HTTPS preference
		aHTTPS := strings.HasPrefix(strings.ToLower(a.URL), "https://")
		bHTTPS := strings.HasPrefix(strings.ToLower(b.URL), "https://")
		if aHTTPS != bHTTPS {
			return aHTTPS
		}

		// 3. Domain priority
		if a.DomainPriority != b.DomainPriority {
			return a.DomainPriority > b.DomainPriority
		}

		// 4. Clean URL length (shorter path, fewer query params)
		aLen := urlComplexity(a.URL)
		bLen := urlComplexity(b.URL)
		if aLen != bLen {
			return aLen < bLen
		}

		// 5. Freshness
		if !a.CrawledAt.Equal(b.CrawledAt) {
			return a.CrawledAt.After(b.CrawledAt)
		}

		// 6. Tie-breaker
		return a.URL < b.URL
	})

	rep := candidates[0]
	aliases := candidates[1:]

	return &ClusterResult{
		Representative: rep,
		Aliases:        aliases,
	}
}

func urlComplexity(rawURL string) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return len(rawURL)
	}
	// Penalty for queries
	score := len(u.Path)
	if u.RawQuery != "" {
		score += len(u.RawQuery) * 2
	}
	return score
}

