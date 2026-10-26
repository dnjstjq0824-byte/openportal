package ranking

import (
	"testing"
	"time"
)

func TestRerankerSignalsAndDemotion(t *testing.T) {
	reranker := NewReranker(DefaultRerankerConfig())
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

	candidates := []DocumentSignals{
		{
			DocumentID:           "doc-spam-stuffing",
			URL:                  "http://spammy-site.com/seo",
			Title:                "Spammy SEO Keyword Stuffing",
			BaseScore:            15.0, // High initial BM25 score
			PublishedAt:          now.Add(-2 * 24 * time.Hour),
			DomainAuthority:      0.1,
			TermFrequencyDensity: 0.35, // severe keyword stuffing
			HasHiddenText:        true,
			IsSpamCluster:        true,
		},
		{
			DocumentID:           "doc-official-fresh",
			URL:                  "https://search.gov.kr/guide",
			Title:                "Official Government Search Guide",
			BaseScore:            12.0, // Slightly lower initial BM25 score
			PublishedAt:          now.Add(-24 * time.Hour), // Fresh
			DomainAuthority:      0.9,
			TermFrequencyDensity: 0.04,
			HasHiddenText:        false,
			IsSpamCluster:        false,
		},
		{
			DocumentID:           "doc-standard-stale",
			URL:                  "https://example.org/archive",
			Title:                "Archived Old Guide",
			BaseScore:            12.0,
			PublishedAt:          now.Add(-400 * 24 * time.Hour), // Very old
			DomainAuthority:      0.5,
			TermFrequencyDensity: 0.05,
			HasHiddenText:        false,
		},
	}

	reranked := reranker.Rerank(candidates, now)

	if len(reranked) != 3 {
		t.Fatalf("expected 3 reranked hits, got %d", len(reranked))
	}

	// doc-official-fresh should take rank 1 due to HTTPS, .go.kr domain boost, freshness, and clean content
	if reranked[0].DocumentID != "doc-official-fresh" {
		t.Errorf("expected rank 1 to be doc-official-fresh, got %s (score: %f)", reranked[0].DocumentID, reranked[0].FinalScore)
	}

	// doc-spam-stuffing started at rank 1 with base score 15.0, but should be demoted to last place due to spam penalties
	if reranked[2].DocumentID != "doc-spam-stuffing" {
		t.Errorf("expected spam document to be demoted to rank 3, got rank %d (%s, score: %f)",
			reranked[2].FinalRank, reranked[2].DocumentID, reranked[2].FinalScore)
	}

	// Check signals map
	officialSignals := reranked[0].Signals
	if officialSignals["https_boost"] <= 0 {
		t.Errorf("expected positive https_boost, got %f", officialSignals["https_boost"])
	}
	if officialSignals["freshness_boost"] <= 0 {
		t.Errorf("expected positive freshness_boost, got %f", officialSignals["freshness_boost"])
	}
	if officialSignals["domain_authority_boost"] <= 0 {
		t.Errorf("expected positive domain_authority_boost, got %f", officialSignals["domain_authority_boost"])
	}

	spamSignals := reranked[2].Signals
	if spamSignals["spam_penalty"] >= 0 {
		t.Errorf("expected negative spam_penalty, got %f", spamSignals["spam_penalty"])
	}
}

func TestFreshnessExponentialDecay(t *testing.T) {
	reranker := NewReranker(DefaultRerankerConfig())
	now := time.Now()

	// 0 days old -> maximum boost
	freshBoost := reranker.ComputeFreshnessBoost(now, now)
	if freshBoost <= 0.24 || freshBoost > 0.250001 {
		t.Errorf("expected fresh boost close to 0.25, got %f", freshBoost)
	}

	// 60 days old (one half life) -> half of max boost (0.125)
	halfLifeDate := now.Add(-60 * 24 * time.Hour)
	halfBoost := reranker.ComputeFreshnessBoost(halfLifeDate, now)
	if halfBoost < 0.12 || halfBoost > 0.13 {
		t.Errorf("expected half life boost close to 0.125, got %f", halfBoost)
	}

	// 360 days old (6 half lives) -> very small boost
	oldDate := now.Add(-360 * 24 * time.Hour)
	oldBoost := reranker.ComputeFreshnessBoost(oldDate, now)
	if oldBoost > 0.01 {
		t.Errorf("expected very small old boost, got %f", oldBoost)
	}
}

