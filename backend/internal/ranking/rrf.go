package ranking

import (
	"math"
	"sort"
)

type RankedItem struct {
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Snippet    string  `json:"snippet"`
	Score      float64 `json:"score"`
}

type RankingSystem struct {
	Name   string       `json:"name"`
	Weight float64      `json:"weight"`
	Hits   []RankedItem `json:"hits"`
}

type FusedHit struct {
	DocumentID  string         `json:"document_id"`
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Snippet     string         `json:"snippet"`
	RRFScore    float64        `json:"rrf_score"`
	Rank        int            `json:"rank"`
	SystemRanks map[string]int `json:"system_ranks"`
}

type RRFConfig struct {
	K int `json:"k"` // Default 60
}

func DefaultRRFConfig() RRFConfig {
	return RRFConfig{K: 60}
}

// ReciprocalRankFusion combines multiple ranking lists into a single hybrid ranking.
func ReciprocalRankFusion(systems []RankingSystem, cfg RRFConfig) []FusedHit {
	k := cfg.K
	if k <= 0 {
		k = 60
	}

	docMap := make(map[string]*FusedHit)

	for _, sys := range systems {
		weight := sys.Weight
		if weight <= 0 {
			weight = 1.0
		}

		for rankIdx, hit := range sys.Hits {
			rank := rankIdx + 1 // 1-based rank
			fused, exists := docMap[hit.DocumentID]
			if !exists {
				fused = &FusedHit{
					DocumentID:  hit.DocumentID,
					Title:       hit.Title,
					URL:         hit.URL,
					Snippet:     hit.Snippet,
					SystemRanks: make(map[string]int),
				}
				docMap[hit.DocumentID] = fused
			} else {
				// Keep longest snippet or best title
				if len(hit.Snippet) > len(fused.Snippet) {
					fused.Snippet = hit.Snippet
				}
				if fused.Title == "" && hit.Title != "" {
					fused.Title = hit.Title
				}
			}

			fused.SystemRanks[sys.Name] = rank
			// RRF component: weight / (k + rank)
			fused.RRFScore += weight / float64(k+rank)
		}
	}

	results := make([]FusedHit, 0, len(docMap))
	for _, f := range docMap {
		results = append(results, *f)
	}

	// Sort descending by RRFScore
	sort.SliceStable(results, func(i, j int) bool {
		if math.Abs(results[i].RRFScore-results[j].RRFScore) > 1e-9 {
			return results[i].RRFScore > results[j].RRFScore
		}
		return results[i].DocumentID < results[j].DocumentID
	})

	for idx := range results {
		results[idx].Rank = idx + 1
	}

	return results
}

