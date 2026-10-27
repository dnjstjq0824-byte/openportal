package vertical

import (
	"strings"
	"testing"
	"time"
)

func TestIngestNewsArticle(t *testing.T) {
	registry := map[string]PublisherPolicy{
		"pub-tech": {
			PublisherID:      "pub-tech",
			Name:             "Tech Chronicle",
			Website:          "https://tech.openportal.org",
			Licensed:         true,
			MaxSummaryChars:  150,
			AllowSyndication: true,
		},
		"pub-unlicensed": {
			PublisherID: "pub-unlicensed",
			Name:        "Random Blog",
			Website:     "https://blog.test",
			Licensed:    false,
		},
	}

	// 1. Unlicensed publisher rejected
	unlicensedArt := NewsArticle{
		ID:           "art-1",
		PublisherID:  "pub-unlicensed",
		Title:        "New breakthrough",
		Summary:      "Short snippet",
		CanonicalURL: "https://blog.test/post/1",
	}
	if _, err := IngestNewsArticle(unlicensedArt, registry); err == nil {
		t.Errorf("expected error for unlicensed publisher, got nil")
	}

	// 2. Licensed publisher with snippet truncation (> 150 chars)
	longSummary := strings.Repeat("OpenPortal search engine architecture. ", 10) // > 300 runes
	licensedArt := NewsArticle{
		ID:           "art-2",
		PublisherID:  "pub-tech",
		Title:        "Distributed search scaling at OpenPortal",
		Summary:      longSummary,
		CanonicalURL: "https://tech.openportal.org/articles/scaling",
		PublishedAt:  time.Now(),
		Section:      "Technology",
	}
	ingested, err := IngestNewsArticle(licensedArt, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(ingested.Summary)) > 152 { // 150 + ellipsis
		t.Errorf("summary snippet was not truncated: len=%d", len([]rune(ingested.Summary)))
	}
	if ingested.SimHash == 0 {
		t.Errorf("expected non-zero SimHash")
	}
}

func TestClusterNewsStories(t *testing.T) {
	now := time.Now()
	articles := []NewsArticle{
		{
			ID:           "art-1",
			PublisherID:  "pub-a",
			Title:        "Spacecraft successfully lands on the Moon in historic mission",
			PublishedAt:  now,
			SimHash:      0x1111222233334444,
		},
		{
			ID:           "art-2",
			PublisherID:  "pub-b",
			Title:        "Historic lunar mission: Spacecraft lands safely on the Moon",
			PublishedAt:  now.Add(-30 * time.Minute),
			SimHash:      0x1111222233334446, // 1 bit diff
		},
		{
			ID:           "art-3",
			PublisherID:  "pub-c",
			Title:        "Global stock markets surge amid tech earnings reports",
			PublishedAt:  now,
			SimHash:      0xF0F0F0F0F0F0F0F0, // completely different
		},
	}

	clusters := ClusterNewsStories(articles, 6*time.Hour, 5)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	// First cluster covers Moon landing by 2 publishers
	if clusters[0].PublisherCount != 2 {
		t.Errorf("expected 2 publishers in Moon landing cluster, got %d", clusters[0].PublisherCount)
	}
	if len(clusters[0].RelatedArticles) != 1 {
		t.Errorf("expected 1 related article, got %d", len(clusters[0].RelatedArticles))
	}
}
