package ranking

import (
	"math"
	"testing"
)

func TestReciprocalRankFusion(t *testing.T) {
	// Lexical BM25 candidate list
	lexical := RankingSystem{
		Name:   "lexical_bm25",
		Weight: 1.0,
		Hits: []RankedItem{
			{DocumentID: "doc-lexical-only", Title: "Lexical Only Title", Score: 18.5},
			{DocumentID: "doc-hybrid-both", Title: "Hybrid Document", Score: 15.2},
			{DocumentID: "doc-stale", Title: "Stale Doc", Score: 9.0},
		},
	}

	// Semantic embedding candidate list
	semantic := RankingSystem{
		Name:   "semantic_embedding",
		Weight: 1.0,
		Hits: []RankedItem{
			{DocumentID: "doc-hybrid-both", Title: "Hybrid Document", Score: 0.89},
			{DocumentID: "doc-semantic-only", Title: "Semantic Only Title", Score: 0.82},
			{DocumentID: "doc-lexical-only", Title: "Lexical Only Title", Score: 0.70},
		},
	}

	fused := ReciprocalRankFusion([]RankingSystem{lexical, semantic}, DefaultRRFConfig())

	if len(fused) != 4 {
		t.Fatalf("expected 4 distinct documents after fusion, got %d", len(fused))
	}

	// doc-hybrid-both is rank 2 in lexical and rank 1 in semantic:
	// RRF = 1/(60+2) + 1/(60+1) = 1/62 + 1/61 = 0.016129 + 0.016393 = 0.032522
	// doc-lexical-only is rank 1 in lexical and rank 3 in semantic:
	// RRF = 1/61 + 1/63 = 0.016393 + 0.015873 = 0.032266
	// Therefore, doc-hybrid-both should take Rank 1!
	if fused[0].DocumentID != "doc-hybrid-both" {
		t.Errorf("expected Rank 1 to be doc-hybrid-both, got %s (RRFScore: %f)", fused[0].DocumentID, fused[0].RRFScore)
	}
	if fused[0].Rank != 1 {
		t.Errorf("expected Rank field to be 1, got %d", fused[0].Rank)
	}

	// doc-lexical-only should be Rank 2
	if fused[1].DocumentID != "doc-lexical-only" {
		t.Errorf("expected Rank 2 to be doc-lexical-only, got %s", fused[1].DocumentID)
	}

	// Check system ranks mapping
	hybridSysRanks := fused[0].SystemRanks
	if hybridSysRanks["lexical_bm25"] != 2 || hybridSysRanks["semantic_embedding"] != 1 {
		t.Errorf("unexpected system ranks: %+v", hybridSysRanks)
	}

	// Test weighted RRF: giving higher weight to lexical
	lexicalWeighted := lexical
	lexicalWeighted.Weight = 2.0
	fusedWeighted := ReciprocalRankFusion([]RankingSystem{lexicalWeighted, semantic}, DefaultRRFConfig())
	// In this case, doc-lexical-only RRF = 2/61 + 1/63 = 0.032786 + 0.015873 = 0.048659
	// doc-hybrid-both RRF = 2/62 + 1/61 = 0.032258 + 0.016393 = 0.048651
	// doc-lexical-only should now be rank 1 due to 2x weight!
	if fusedWeighted[0].DocumentID != "doc-lexical-only" {
		t.Errorf("expected doc-lexical-only at rank 1 with 2x lexical weight, got %s", fusedWeighted[0].DocumentID)
	}
}

func TestRRFDefaultK(t *testing.T) {
	sys := RankingSystem{
		Name: "single",
		Hits: []RankedItem{{DocumentID: "doc-1", Score: 1.0}},
	}
	fused := ReciprocalRankFusion([]RankingSystem{sys}, RRFConfig{K: 0})
	// K <= 0 should default to 60: score = 1/(60+1) = 1/61
	expected := 1.0 / 61.0
	if math.Abs(fused[0].RRFScore-expected) > 1e-6 {
		t.Errorf("expected default k=60 score %f, got %f", expected, fused[0].RRFScore)
	}
}

