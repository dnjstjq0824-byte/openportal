package ranking

import (
	"fmt"
	"math"
	"testing"
)

func TestExperimentDeterministicAssignmentAndDistribution(t *testing.T) {
	p := NewExperimentPlatform()

	exp := ExperimentDef{
		ID:               "exp-ranking-bm25-rrf",
		Name:             "BM25 vs RRF Hybrid",
		Layer:            "ranking",
		Salt:             "random_salt_v1",
		SplitPercent:     50, // 50% treatment
		Status:           ExpStatusRunning,
		ControlProfile:   "bm25_baseline_v1",
		TreatmentProfile: "rrf_hybrid_candidate_v1",
	}

	if err := p.RegisterExperiment(exp); err != nil {
		t.Fatalf("RegisterExperiment failed: %v", err)
	}

	// 1. Verify idempotency / determinism for the same user
	userID := "user-alice-1234"
	v1, prof1, _ := p.AssignDeterministic(exp.ID, userID)
	v2, prof2, _ := p.AssignDeterministic(exp.ID, userID)
	if v1 != v2 || prof1 != prof2 {
		t.Fatalf("assignment must be strictly deterministic: (%v, %s) vs (%v, %s)", v1, prof1, v2, prof2)
	}

	// 2. Verify population distribution across 1000 users
	treatmentCount := 0
	total := 1000
	for i := 0; i < total; i++ {
		uid := fmt.Sprintf("sim-user-%d", i)
		v, _, err := p.AssignDeterministic(exp.ID, uid)
		if err != nil {
			t.Fatalf("AssignDeterministic error: %v", err)
		}
		if v == VariantTreatment {
			treatmentCount++
		}
	}

	ratio := float64(treatmentCount) / float64(total)
	// Expected ~0.50 (within +/- 0.05 margin)
	if math.Abs(ratio-0.50) > 0.05 {
		t.Errorf("expected 50%% split ratio ~0.50 (+/-0.05), got %f (%d/%d)", ratio, treatmentCount, total)
	}
}

func TestLayerMutualExclusion(t *testing.T) {
	p := NewExperimentPlatform()

	exp1 := ExperimentDef{
		ID:     "exp-ranking-1",
		Layer:  "ranking",
		Status: ExpStatusRunning,
	}
	if err := p.RegisterExperiment(exp1); err != nil {
		t.Fatalf("exp1 register failed: %v", err)
	}

	// exp2 on the same "ranking" layer while exp1 is running must fail
	exp2 := ExperimentDef{
		ID:     "exp-ranking-2",
		Layer:  "ranking",
		Status: ExpStatusRunning,
	}
	err := p.RegisterExperiment(exp2)
	if err == nil {
		t.Fatalf("expected layer mutual exclusion error for exp2 on occupied layer 'ranking'")
	}

	// exp3 on a different layer ("ui") should succeed
	exp3 := ExperimentDef{
		ID:     "exp-ui-1",
		Layer:  "ui",
		Status: ExpStatusRunning,
	}
	if err := p.RegisterExperiment(exp3); err != nil {
		t.Fatalf("exp3 on ui layer should succeed: %v", err)
	}
}

func TestAutomatedRollbackCircuitBreaker(t *testing.T) {
	p := NewExperimentPlatform()

	expID := "exp-safety-rollback"
	exp := ExperimentDef{
		ID:                    expID,
		Layer:                 "ranking",
		Salt:                  "salt_cb",
		SplitPercent:          50,
		Status:                ExpStatusRunning,
		ControlProfile:        "safe_control",
		TreatmentProfile:      "risky_treatment",
		MinSampleSize:         20,
		MaxErrorRateThreshold: 0.10, // 10% error limit
		MaxP99LatencyThreshold: 200.0,
	}

	if err := p.RegisterExperiment(exp); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Inject 15 normal treatment requests
	for i := 0; i < 15; i++ {
		p.RecordMetric(expID, VariantTreatment, false, 25.0)
	}

	// Status must still be running
	p.mu.RLock()
	if p.experiments[expID].Status != ExpStatusRunning {
		t.Fatalf("expected running status with 15 requests")
	}
	p.mu.RUnlock()

	// Inject 5 failed requests out of 25 total (5 / 25 = 20% error rate > 10% threshold)
	for i := 0; i < 5; i++ {
		p.RecordMetric(expID, VariantTreatment, false, 25.0)
	}
	for i := 0; i < 5; i++ {
		p.RecordMetric(expID, VariantTreatment, true, 25.0)
	}

	// Automated rollback should have triggered!
	p.mu.RLock()
	status := p.experiments[expID].Status
	reason := p.experiments[expID].RollbackReason
	p.mu.RUnlock()

	if status != ExpStatusRolledBack {
		t.Fatalf("expected status rolled_back, got %s", status)
	}
	if reason == "" {
		t.Fatalf("expected rollback reason recorded")
	}

	// Subsequent assignments must now fall back to Control profile
	v, prof, err := p.AssignDeterministic(expID, "any-user")
	if err != nil {
		t.Fatalf("assign after rollback error: %v", err)
	}
	if v != VariantControl || prof != "safe_control" {
		t.Fatalf("expected fallback to control after rollback, got (%v, %s)", v, prof)
	}
}

