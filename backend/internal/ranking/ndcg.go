package ranking

import (
	"math"
	"sort"
)

// ComputeDCG computes Discounted Cumulative Gain at rank k.
// DCG@k = \sum_{i=1}^k \frac{2^{rel_i} - 1}{\log_2(i + 1)}
func ComputeDCG(relevance []int, k int) float64 {
	if k <= 0 || len(relevance) == 0 {
		return 0.0
	}
	limit := k
	if len(relevance) < limit {
		limit = len(relevance)
	}

	var dcg float64
	for i := 0; i < limit; i++ {
		rel := relevance[i]
		if rel < 0 {
			rel = 0
		}
		gain := math.Pow(2, float64(rel)) - 1.0
		discount := math.Log2(float64(i + 2)) // i=0 -> log2(2) = 1
		dcg += gain / discount
	}
	return dcg
}

// ComputeIDCG computes Ideal Discounted Cumulative Gain at rank k.
func ComputeIDCG(relevance []int, k int) float64 {
	if k <= 0 || len(relevance) == 0 {
		return 0.0
	}
	sorted := make([]int, len(relevance))
	copy(sorted, relevance)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] > sorted[j]
	})
	return ComputeDCG(sorted, k)
}

// ComputeNDCG computes Normalized Discounted Cumulative Gain at rank k (NDCG@k).
func ComputeNDCG(relevance []int, k int) float64 {
	idcg := ComputeIDCG(relevance, k)
	if idcg <= 0.0 {
		// If there are no relevant documents in ground truth, check if predictions also 0
		dcg := ComputeDCG(relevance, k)
		if dcg <= 0.0 {
			return 1.0 // Empty or all-irrelevant matches ideal
		}
		return 0.0
	}
	dcg := ComputeDCG(relevance, k)
	ndcg := dcg / idcg
	if ndcg > 1.0 {
		return 1.0
	}
	return ndcg
}

// ComputeMRR computes Mean Reciprocal Rank for a set of queries.
// firstRelevantRanks: 1-based rank of the first relevant document per query (0 if none found).
func ComputeMRR(firstRelevantRanks []int) float64 {
	if len(firstRelevantRanks) == 0 {
		return 0.0
	}
	var totalRR float64
	for _, rank := range firstRelevantRanks {
		if rank > 0 {
			totalRR += 1.0 / float64(rank)
		}
	}
	return totalRR / float64(len(firstRelevantRanks))
}

// ComputeZeroResultsRate computes fraction of queries that yielded 0 search hits.
func ComputeZeroResultsRate(hitCounts []int64) float64 {
	if len(hitCounts) == 0 {
		return 0.0
	}
	zeros := 0
	for _, count := range hitCounts {
		if count <= 0 {
			zeros++
		}
	}
	return float64(zeros) / float64(len(hitCounts))
}

// DefaultPositionPropensities returns eye-tracking position bias propensities for ranks 1..10.
// Based on power-law examination model: P(examine | rank) = 1 / rank^0.65
func DefaultPositionPropensities(k int) []float64 {
	if k <= 0 {
		k = 10
	}
	prop := make([]float64, k)
	for i := 0; i < k; i++ {
		rank := float64(i + 1)
		prop[i] = 1.0 / math.Pow(rank, 0.65)
	}
	return prop
}

// ComputeDebiasedCTR applies Inverse Propensity Scoring (IPS) to clicks.
// Returns de-biased relevance estimate per position.
func ComputeDebiasedCTR(impressions, clicks []int64, propensities []float64) []float64 {
	n := len(clicks)
	if len(impressions) < n {
		n = len(impressions)
	}
	if len(propensities) < n {
		n = len(propensities)
	}

	debiased := make([]float64, n)
	for i := 0; i < n; i++ {
		if impressions[i] <= 0 || propensities[i] <= 0 {
			continue
		}
		rawCTR := float64(clicks[i]) / float64(impressions[i])
		// IPS de-biasing: CTR / Propensity
		debiased[i] = rawCTR / propensities[i]
	}
	return debiased
}

