package ranking

import (
	"fmt"
)

type QualityGateThresholds struct {
	MinNDCG10           float64 `json:"min_ndcg_10"`            // >= 0.70
	MinMRR10            float64 `json:"min_mrr_10"`             // >= 0.60
	MaxZeroResultsRate  float64 `json:"max_zero_results_rate"`  // <= 0.05 (5%)
	MinSafeSearchAccuracy float64 `json:"min_safesearch_accuracy"` // >= 0.95 (95%)
}

func DefaultQualityGateThresholds() QualityGateThresholds {
	return QualityGateThresholds{
		MinNDCG10:             0.70,
		MinMRR10:              0.60,
		MaxZeroResultsRate:    0.05,
		MinSafeSearchAccuracy: 0.95,
	}
}

type QualityEvaluationReport struct {
	Passed             bool              `json:"passed"`
	Decision           string            `json:"decision"` // "APPROVED (GO)" or "REJECTED (NO-GO)"
	NDCG10             float64           `json:"ndcg_10"`
	MRR10              float64           `json:"mrr_10"`
	ZeroResultsRate    float64           `json:"zero_results_rate"`
	SafeSearchAccuracy float64           `json:"safesearch_accuracy"`
	Violations         []string          `json:"violations,omitempty"`
	Thresholds         QualityGateThresholds `json:"thresholds"`
}

type QualitySignOffGate struct {
	thresholds QualityGateThresholds
}

func NewQualitySignOffGate(th QualityGateThresholds) *QualitySignOffGate {
	return &QualitySignOffGate{thresholds: th}
}

// Evaluate performs formal release gate verification.
// If any quality or safety threshold fails, the release is strictly blocked (fail-closed).
func (g *QualitySignOffGate) Evaluate(
	perQueryRelevances [][]int,
	firstRelevantRanks []int,
	hitCounts []int64,
	safesearchTested, safesearchViolations int,
) QualityEvaluationReport {
	var totalNDCG float64
	numQueries := len(perQueryRelevances)
	for _, rels := range perQueryRelevances {
		totalNDCG += ComputeNDCG(rels, 10)
	}
	meanNDCG := 0.0
	if numQueries > 0 {
		meanNDCG = totalNDCG / float64(numQueries)
	}

	mrr := ComputeMRR(firstRelevantRanks)
	zeroRate := ComputeZeroResultsRate(hitCounts)

	safeAccuracy := 1.0
	if safesearchTested > 0 {
		safeAccuracy = 1.0 - (float64(safesearchViolations) / float64(safesearchTested))
	}

	rep := QualityEvaluationReport{
		NDCG10:             meanNDCG,
		MRR10:              mrr,
		ZeroResultsRate:    zeroRate,
		SafeSearchAccuracy: safeAccuracy,
		Thresholds:         g.thresholds,
	}

	if meanNDCG < g.thresholds.MinNDCG10 {
		rep.Violations = append(rep.Violations, fmt.Sprintf("NDCG@10 %.3f below threshold %.3f", meanNDCG, g.thresholds.MinNDCG10))
	}
	if mrr < g.thresholds.MinMRR10 {
		rep.Violations = append(rep.Violations, fmt.Sprintf("MRR@10 %.3f below threshold %.3f", mrr, g.thresholds.MinMRR10))
	}
	if zeroRate > g.thresholds.MaxZeroResultsRate {
		rep.Violations = append(rep.Violations, fmt.Sprintf("ZeroResultsRate %.3f exceeded threshold %.3f", zeroRate, g.thresholds.MaxZeroResultsRate))
	}
	if safeAccuracy < g.thresholds.MinSafeSearchAccuracy {
		rep.Violations = append(rep.Violations, fmt.Sprintf("SafeSearchAccuracy %.3f below threshold %.3f", safeAccuracy, g.thresholds.MinSafeSearchAccuracy))
	}

	if len(rep.Violations) == 0 {
		rep.Passed = true
		rep.Decision = "APPROVED (GO)"
	} else {
		rep.Passed = false
		rep.Decision = "REJECTED (NO-GO)"
	}

	return rep
}

