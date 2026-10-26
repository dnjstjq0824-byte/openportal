package search

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type GoldenQuery struct {
	Query             string `json:"query"`
	Language          string `json:"language"`
	ExpectedTopURL    string `json:"expected_top_url"`
	RelevanceRankings []struct {
		URL          string  `json:"url"`
		MinScore     float64 `json:"min_score"`
		ExpectedRank int     `json:"expected_rank"`
	} `json:"relevance_rankings"`
}

func TestGoldenRankingDatasetFormat(t *testing.T) {
	path := filepath.Join("testdata", "ranking_golden_dataset.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden dataset: %v", err)
	}

	var queries []GoldenQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		t.Fatalf("failed to parse golden dataset JSON: %v", err)
	}

	if len(queries) < 3 {
		t.Errorf("expected at least 3 queries in golden dataset, got %d", len(queries))
	}

	for _, q := range queries {
		if q.Query == "" || q.ExpectedTopURL == "" || len(q.RelevanceRankings) == 0 {
			t.Errorf("invalid query entry in golden dataset: %+v", q)
		}
	}
}

