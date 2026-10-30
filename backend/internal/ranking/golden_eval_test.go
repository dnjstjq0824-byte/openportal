package ranking

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type GoldenQueryItem struct {
	Query             string `json:"query"`
	Language          string `json:"language"`
	ExpectedTopURL    string `json:"expected_top_url"`
	RelevanceRankings []struct {
		URL          string  `json:"url"`
		MinScore     float64 `json:"min_score"`
		ExpectedRank int     `json:"expected_rank"`
	} `json:"relevance_rankings"`
}

func TestGoldenDatasetNDCGAndMRR(t *testing.T) {
	goldenPath := filepath.Join("..", "search", "testdata", "ranking_golden_dataset.json")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden dataset file: %v", err)
	}

	var items []GoldenQueryItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("failed to parse golden dataset: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected non-empty golden dataset")
	}

	reranker := NewReranker(DefaultRerankerConfig())
	now := time.Now()

	var firstRelevantRanks []int
	var ndcgScores []float64

	for _, item := range items {
		// Construct candidate signals from the golden set relevance specification
		candidates := make([]DocumentSignals, 0, len(item.RelevanceRankings))
		truthGrades := make(map[string]int)

		for _, rel := range item.RelevanceRankings {
			grade := 1
			if rel.ExpectedRank == 1 {
				grade = 3 // Vital
			} else if rel.ExpectedRank == 2 {
				grade = 2 // Relevant
			}
			truthGrades[rel.URL] = grade

			candidates = append(candidates, DocumentSignals{
				DocumentID:           "doc-" + filepath.Base(rel.URL),
				URL:                  rel.URL,
				Title:                item.Query + " - " + rel.URL,
				Description:          "Snippet for " + item.Query,
				BaseScore:            rel.MinScore,
				PublishedAt:          now.Add(-24 * time.Hour),
				DomainAuthority:      0.8,
				TermFrequencyDensity: 0.05,
			})
		}

		// Execute re-ranking
		reranked := reranker.Rerank(candidates, now)
		if len(reranked) == 0 {
			t.Errorf("query %q returned no reranked results", item.Query)
			continue
		}

		// Check top URL
		if reranked[0].URL != item.ExpectedTopURL {
			t.Errorf("query %q: expected top URL %q, got %q", item.Query, item.ExpectedTopURL, reranked[0].URL)
		}

		// Calculate NDCG@10 for this query
		relevanceSequence := make([]int, len(reranked))
		firstRank := 0
		for idx, hit := range reranked {
			grade := truthGrades[hit.URL]
			relevanceSequence[idx] = grade
			if grade >= 2 && firstRank == 0 {
				firstRank = idx + 1 // 1-based rank
			}
		}
		firstRelevantRanks = append(firstRelevantRanks, firstRank)

		ndcg := ComputeNDCG(relevanceSequence, 10)
		ndcgScores = append(ndcgScores, ndcg)
		if ndcg < 0.80 {
			t.Errorf("query %q NDCG@10 too low: got %f, want >= 0.80", item.Query, ndcg)
		}
	}

	// Calculate overall MRR
	mrr := ComputeMRR(firstRelevantRanks)
	if mrr < 0.90 {
		t.Errorf("overall MRR@10 too low: got %f, want >= 0.90", mrr)
	}

	// Calculate Mean NDCG
	totalNDCG := 0.0
	for _, s := range ndcgScores {
		totalNDCG += s
	}
	meanNDCG := totalNDCG / float64(len(ndcgScores))
	if meanNDCG < 0.90 {
		t.Errorf("mean NDCG@10 too low: got %f, want >= 0.90", meanNDCG)
	}

	t.Logf("Golden Dataset Benchmark: %d queries, Mean NDCG@10 = %.4f, MRR@10 = %.4f (PASS)", len(items), meanNDCG, mrr)
}

func TestGoldenDatasetSpamDemotion(t *testing.T) {
	reranker := NewReranker(DefaultRerankerConfig())
	now := time.Now()

	candidates := []DocumentSignals{
		{
			DocumentID:           "doc-spam-stuffed",
			URL:                  "https://spam.test/casino",
			Title:                "Best Casino Online Casino Games Casino",
			Description:          "Keyword stuffing sample text with excessive repetition",
			BaseScore:            0.95, // Initially high text match score
			PublishedAt:          now,
			DomainAuthority:      0.2,
			TermFrequencyDensity: 0.28, // Over the 0.15 threshold!
		},
		{
			DocumentID:           "doc-quality-official",
			URL:                  "https://games.example.org/guide",
			Title:                "Official Online Game Rules & Guide",
			Description:          "Authoritative reference guide published by licensed editorial team.",
			BaseScore:            0.85,
			PublishedAt:          now.Add(-48 * time.Hour),
			DomainAuthority:      0.9,
			TermFrequencyDensity: 0.04,
		},
	}

	reranked := reranker.Rerank(candidates, now)
	if len(reranked) != 2 {
		t.Fatalf("expected 2 reranked items, got %d", len(reranked))
	}

	// The quality document must overcome the spam document despite lower initial base score
	if reranked[0].DocumentID != "doc-quality-official" {
		t.Errorf("expected quality doc to rank #1 after spam demotion, got %s (score: %.3f vs %.3f)",
			reranked[0].DocumentID, reranked[0].FinalScore, reranked[1].FinalScore)
	}
	if reranked[1].DocumentID != "doc-spam-stuffed" {
		t.Errorf("expected spam doc to be demoted to rank #2, got %s", reranked[1].DocumentID)
	}
}

