package vertical

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"example.com/openportal-search/backend/internal/indexer"
)

// PublisherPolicy holds licensing bounds for a verified news publisher.
type PublisherPolicy struct {
	PublisherID     string `json:"publisher_id"`
	Name            string `json:"name"`
	Website         string `json:"website"`
	Licensed        bool   `json:"licensed"`
	MaxSummaryChars int    `json:"max_summary_chars"`
	AllowSyndication bool  `json:"allow_syndication"`
}

// NewsArticle represents an ingested licensed news article.
type NewsArticle struct {
	ID          string    `json:"id"`
	PublisherID string    `json:"publisher_id"`
	Publisher   string    `json:"publisher"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"` // Strictly enforced snippet
	CanonicalURL string   `json:"canonical_url"`
	PublishedAt time.Time `json:"published_at"`
	Section     string    `json:"section"`
	SimHash     uint64    `json:"simhash"`
	IsStale     bool      `json:"is_stale"`
}

// StoryCluster groups news articles covering the same event across publishers.
type StoryCluster struct {
	ClusterID        string        `json:"cluster_id"`
	LeadArticle      NewsArticle   `json:"lead_article"`
	RelatedArticles  []NewsArticle `json:"related_articles"`
	PublisherCount   int           `json:"publisher_count"`
	LatestPublishedAt time.Time   `json:"latest_published_at"`
}

// IngestNewsArticle validates publisher licensing and enforces copyright snippet limits.
func IngestNewsArticle(art NewsArticle, registry map[string]PublisherPolicy) (*NewsArticle, error) {
	policy, ok := registry[art.PublisherID]
	if !ok || !policy.Licensed {
		return nil, fmt.Errorf("publisher %s is not licensed for news syndication", art.PublisherID)
	}

	parsed, err := url.Parse(art.CanonicalURL)
	if err != nil || !strings.Contains(strings.ToLower(parsed.Host), strings.ToLower(policy.Website)) {
		// Verify canonical URL belongs to publisher domain
		if !strings.HasSuffix(strings.ToLower(parsed.Host), strings.TrimPrefix(strings.ToLower(policy.Website), "https://")) {
			return nil, errors.New("canonical URL domain does not match verified publisher domain")
		}
	}

	// Enforce copyright rights boundary: max characters for snippet/summary
	maxChars := policy.MaxSummaryChars
	if maxChars <= 0 || maxChars > 300 {
		maxChars = 300
	}

	summaryRunes := []rune(strings.TrimSpace(art.Summary))
	if len(summaryRunes) > maxChars {
		art.Summary = string(summaryRunes[:maxChars]) + "…"
	}

	// Compute SimHash on Title for deduplication
	art.SimHash = indexer.SimHash(art.Title)

	return &art, nil
}

// ClusterNewsStories groups articles covering the same breaking story across publishers within a time window (e.g. 6 hours)
// using title SimHash Hamming distance <= 12 bits.
func ClusterNewsStories(articles []NewsArticle, timeWindow time.Duration, maxHammingDist int) []StoryCluster {
	if len(articles) == 0 {
		return nil
	}

	// Sort chronologically descending
	sorted := make([]NewsArticle, len(articles))
	copy(sorted, articles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
	})

	visited := make(map[string]bool)
	var clusters []StoryCluster

	for i := 0; i < len(sorted); i++ {
		lead := sorted[i]
		if visited[lead.ID] {
			continue
		}

		visited[lead.ID] = true
		var related []NewsArticle
		pubSet := map[string]bool{lead.PublisherID: true}
		latestTime := lead.PublishedAt

		for j := i + 1; j < len(sorted); j++ {
			candidate := sorted[j]
			if visited[candidate.ID] {
				continue
			}

			// Time window condition
			timeDiff := lead.PublishedAt.Sub(candidate.PublishedAt)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			if timeDiff > timeWindow {
				continue
			}

			// SimHash Hamming distance check
			if indexer.HammingDistance(lead.SimHash, candidate.SimHash) <= maxHammingDist {
				related = append(related, candidate)
				visited[candidate.ID] = true
				pubSet[candidate.PublisherID] = true
				if candidate.PublishedAt.After(latestTime) {
					latestTime = candidate.PublishedAt
				}
			}
		}

		clusters = append(clusters, StoryCluster{
			ClusterID:         "story-" + lead.ID,
			LeadArticle:       lead,
			RelatedArticles:   related,
			PublisherCount:    len(pubSet),
			LatestPublishedAt: latestTime,
		})
	}

	return clusters
}

// SearchNews performs keyword search on news articles and groups them into story clusters.
func SearchNews(query string, articles []NewsArticle, sortBy string) []StoryCluster {
	qTokens := strings.Fields(strings.ToLower(query))

	var matched []NewsArticle
	for _, a := range articles {
		titleLow := strings.ToLower(a.Title)
		summaryLow := strings.ToLower(a.Summary)
		secLow := strings.ToLower(a.Section)

		hit := false
		for _, tok := range qTokens {
			if strings.Contains(titleLow, tok) || strings.Contains(summaryLow, tok) || strings.Contains(secLow, tok) {
				hit = true
				break
			}
		}
		if hit {
			matched = append(matched, a)
		}
	}

	clusters := ClusterNewsStories(matched, 6*time.Hour, 12)

	if sortBy == "date" {
		sort.Slice(clusters, func(i, j int) bool {
			return clusters[i].LatestPublishedAt.After(clusters[j].LatestPublishedAt)
		})
	}

	return clusters
}
