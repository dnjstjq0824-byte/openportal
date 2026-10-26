package ranking

import (
	"math"
	"testing"
)

func TestNDCGComputation(t *testing.T) {
	// Ideal ranking: 3, 2, 2, 1, 0
	ideal := []int{3, 2, 2, 1, 0}
	ndcgIdeal := ComputeNDCG(ideal, 5)
	if math.Abs(ndcgIdeal-1.0) > 1e-6 {
		t.Errorf("ideal ranking NDCG@5 should be 1.0, got %f", ndcgIdeal)
	}

	// Imperfect ranking: 0, 1, 2, 3, 2
	imperfect := []int{0, 1, 2, 3, 2}
	ndcgImperfect := ComputeNDCG(imperfect, 5)
	if ndcgImperfect >= 1.0 || ndcgImperfect <= 0.0 {
		t.Errorf("imperfect ranking NDCG@5 should be between 0 and 1, got %f", ndcgImperfect)
	}
	// Specifically, with top rank getting 0, NDCG should be notably penalized (< 0.75)
	if ndcgImperfect > 0.75 {
		t.Errorf("expected imperfect NDCG to be < 0.75, got %f", ndcgImperfect)
	}

	// All zeros
	allZero := []int{0, 0, 0}
	ndcgZero := ComputeNDCG(allZero, 3)
	if ndcgZero != 1.0 {
		t.Errorf("all zeros should return 1.0 match with ideal, got %f", ndcgZero)
	}
}

func TestMRRComputation(t *testing.T) {
	// Query 1: first relevant at rank 1 (RR = 1.0)
	// Query 2: first relevant at rank 2 (RR = 0.5)
	// Query 3: first relevant at rank 4 (RR = 0.25)
	// Query 4: no relevant in top k (RR = 0.0)
	ranks := []int{1, 2, 4, 0}
	// Expected MRR = (1.0 + 0.5 + 0.25 + 0.0) / 4 = 1.75 / 4 = 0.4375
	mrr := ComputeMRR(ranks)
	expected := 0.4375
	if math.Abs(mrr-expected) > 1e-4 {
		t.Errorf("expected MRR = %f, got %f", expected, mrr)
	}
}

func TestZeroResultsRate(t *testing.T) {
	hits := []int64{150, 0, 42, 0, 12, 5, 0, 89, 100, 3}
	// 3 out of 10 queries have 0 hits -> 30%
	rate := ComputeZeroResultsRate(hits)
	if math.Abs(rate-0.30) > 1e-4 {
		t.Errorf("expected zero results rate 0.30, got %f", rate)
	}
}

func TestClickPositionBiasIPS(t *testing.T) {
	impressions := []int64{1000, 1000, 1000}
	// Rank 1: 300 clicks (30% raw CTR)
	// Rank 2: 150 clicks (15% raw CTR)
	// Rank 3: 100 clicks (10% raw CTR)
	clicks := []int64{300, 150, 100}

	propensities := DefaultPositionPropensities(3)
	if len(propensities) != 3 {
		t.Fatalf("expected 3 propensities, got %d", len(propensities))
	}
	if propensities[0] != 1.0 {
		t.Errorf("rank 1 propensity should be 1.0, got %f", propensities[0])
	}
	if propensities[1] >= propensities[0] || propensities[2] >= propensities[1] {
		t.Errorf("propensities should strictly decay with rank: %v", propensities)
	}

	debiased := ComputeDebiasedCTR(impressions, clicks, propensities)
	if len(debiased) != 3 {
		t.Fatalf("expected 3 debiased values, got %d", len(debiased))
	}
	// At rank 1, debiased = 0.30 / 1.0 = 0.30
	if math.Abs(debiased[0]-0.30) > 1e-4 {
		t.Errorf("expected rank 1 debiased CTR 0.30, got %f", debiased[0])
	}
	// At rank 2, raw CTR is 0.15, but propensity is ~0.637 -> debiased should be ~0.235, higher than raw CTR
	if debiased[1] <= 0.15 {
		t.Errorf("debiased CTR at rank 2 should be higher than raw 0.15, got %f", debiased[1])
	}
}

