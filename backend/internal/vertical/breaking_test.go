package vertical

import (
	"testing"
	"time"
)

func TestDetectBreakingBursts(t *testing.T) {
	now := time.Now()
	clusters := []StoryCluster{
		{
			ClusterID:         "cluster-1",
			LeadArticle:       NewsArticle{Title: "Earthquake reported near coast", PublishedAt: now.Add(-10 * time.Minute)},
			RelatedArticles:   []NewsArticle{{ID: "a2"}, {ID: "a3"}},
			PublisherCount:    3, // 3 independent publishers
			LatestPublishedAt: now.Add(-2 * time.Minute),
		},
		{
			ClusterID:         "cluster-2",
			LeadArticle:       NewsArticle{Title: "Unconfirmed blog rumor", PublishedAt: now.Add(-5 * time.Minute)},
			RelatedArticles:   []NewsArticle{{ID: "b2"}},
			PublisherCount:    1, // Only 1 publisher -> must NOT trigger alert
			LatestPublishedAt: now.Add(-1 * time.Minute),
		},
	}

	alerts := DetectBreakingBursts(clusters, 30*time.Minute, 2, 2)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 confirmed alert, got %d", len(alerts))
	}
	if alerts[0].StoryClusterID != "cluster-1" {
		t.Errorf("expected cluster-1 as breaking alert, got %s", alerts[0].StoryClusterID)
	}
}

func TestEvaluateStaleness(t *testing.T) {
	mgr := NewNewsLifecycleManager(365)

	fresh := &NewsArticle{
		ID:          "fresh-1",
		PublishedAt: time.Now().Add(-48 * time.Hour), // 2 days old
	}
	isStale, _ := mgr.EvaluateStaleness(fresh)
	if isStale {
		t.Errorf("expected fresh article not to be stale")
	}

	stale := &NewsArticle{
		ID:          "stale-1",
		PublishedAt: time.Now().Add(-500 * 24 * time.Hour), // > 500 days old
	}
	isStale, notice := mgr.EvaluateStaleness(stale)
	if !isStale {
		t.Errorf("expected 500-day old article to be marked stale")
	}
	if notice == "" {
		t.Errorf("expected staleness warning banner text")
	}
}

func TestProcessRetractionAndFilter(t *testing.T) {
	mgr := NewNewsLifecycleManager(365)

	articles := []NewsArticle{
		{ID: "art-ok", Title: "Valid factual report", PublishedAt: time.Now()},
		{ID: "art-retract", Title: "Erroneous financial forecast", PublishedAt: time.Now()},
	}

	// Before retraction: both active
	active := mgr.FilterActiveArticles(articles)
	if len(active) != 2 {
		t.Fatalf("expected 2 active articles, got %d", len(active))
	}

	// Register retraction for art-retract
	rec, err := mgr.ProcessRetraction("art-retract", "Factual error acknowledged by editorial board")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rec.IsTombstoned {
		t.Errorf("expected tombstoned record")
	}

	// Check retraction check
	if !mgr.IsRetracted("art-retract") {
		t.Errorf("expected art-retract to be reported as retracted")
	}

	// After retraction: art-retract is filtered out immediately
	activeAfter := mgr.FilterActiveArticles(articles)
	if len(activeAfter) != 1 || activeAfter[0].ID != "art-ok" {
		t.Fatalf("expected only art-ok in active feed, got %+v", activeAfter)
	}
}
