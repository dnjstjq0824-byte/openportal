package launch

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// SLOTarget defines a Service Level Objective (Task 096).
type SLOTarget struct {
	Name               string    `json:"name"`
	TargetThreshold    float64   `json:"target_threshold"` // e.g. 99.9 for availability, 200.0 for p95 latency
	MeasuredValue      float64   `json:"measured_value"`
	Unit               string    `json:"unit"` // "%", "ms"
	BudgetRemainingPct float64   `json:"budget_remaining_pct"`
	Status             string    `json:"status"` // HEALTHY, WARNING, BREACHED
	EvaluatedAt        time.Time `json:"evaluated_at"`
}

// SLOTracker monitors system reliability against defined error budgets.
type SLOTracker struct {
	mu      sync.RWMutex
	targets map[string]SLOTarget
}

// NewSLOTracker initializes standard production SLO targets.
func NewSLOTracker() *SLOTracker {
	tracker := &SLOTracker{
		targets: make(map[string]SLOTarget),
	}
	tracker.targets["search_availability"] = SLOTarget{
		Name:               "search_availability",
		TargetThreshold:    99.9,
		MeasuredValue:      100.0,
		Unit:               "%",
		BudgetRemainingPct: 100.0,
		Status:             "HEALTHY",
		EvaluatedAt:        time.Now().UTC(),
	}
	tracker.targets["search_latency_p95"] = SLOTarget{
		Name:               "search_latency_p95",
		TargetThreshold:    200.0,
		MeasuredValue:      35.0,
		Unit:               "ms",
		BudgetRemainingPct: 100.0,
		Status:             "HEALTHY",
		EvaluatedAt:        time.Now().UTC(),
	}
	return tracker
}

// EvaluateAvailability updates availability SLO and computes consumed error budget.
func (t *SLOTracker) EvaluateAvailability(totalRequests, successfulRequests int) SLOTarget {
	t.mu.Lock()
	defer t.mu.Unlock()

	var avail float64 = 100.0
	if totalRequests > 0 {
		avail = (float64(successfulRequests) / float64(totalRequests)) * 100.0
		avail = math.Round(avail*1000) / 1000
	}

	target := 99.9
	allowedErrorPct := 100.0 - target // 0.1%
	actualErrorPct := 100.0 - avail

	budgetConsumedPct := (actualErrorPct / allowedErrorPct) * 100.0
	if budgetConsumedPct < 0 {
		budgetConsumedPct = 0
	}
	budgetRemaining := 100.0 - budgetConsumedPct
	if budgetRemaining < 0 {
		budgetRemaining = 0
	}

	status := "HEALTHY"
	if budgetRemaining < 20.0 {
		status = "WARNING"
	}
	if avail < target {
		status = "BREACHED"
	}

	slo := SLOTarget{
		Name:               "search_availability",
		TargetThreshold:    target,
		MeasuredValue:      avail,
		Unit:               "%",
		BudgetRemainingPct: math.Round(budgetRemaining*10) / 10,
		Status:             status,
		EvaluatedAt:        time.Now().UTC(),
	}
	t.targets["search_availability"] = slo
	return slo
}

// EvaluateLatency updates P95 latency SLO.
func (t *SLOTracker) EvaluateLatency(p95Ms float64) SLOTarget {
	t.mu.Lock()
	defer t.mu.Unlock()

	target := 200.0
	status := "HEALTHY"
	var budgetRemaining float64 = 100.0

	if p95Ms > target*0.8 {
		status = "WARNING"
		budgetRemaining = 50.0
	}
	if p95Ms > target {
		status = "BREACHED"
		budgetRemaining = 0.0
	}

	slo := SLOTarget{
		Name:               "search_latency_p95",
		TargetThreshold:    target,
		MeasuredValue:      p95Ms,
		Unit:               "ms",
		BudgetRemainingPct: budgetRemaining,
		Status:             status,
		EvaluatedAt:        time.Now().UTC(),
	}
	t.targets["search_latency_p95"] = slo
	return slo
}

// AllSLOs returns current snapshot of all SLO targets.
func (t *SLOTracker) AllSLOs() []SLOTarget {
	t.mu.RLock()
	defer t.mu.RUnlock()

	list := make([]SLOTarget, 0, len(t.targets))
	for _, s := range t.targets {
		list = append(list, s)
	}
	return list
}

// CheckBreaches reports any breached or warning SLOs.
func (t *SLOTracker) CheckBreaches() (hasBreach bool, alerts []string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, s := range t.targets {
		if s.Status == "BREACHED" {
			hasBreach = true
			alerts = append(alerts, fmt.Sprintf("SLO %s breached: measured %.2f%s (target %.2f%s)", s.Name, s.MeasuredValue, s.Unit, s.TargetThreshold, s.Unit))
		} else if s.Status == "WARNING" {
			alerts = append(alerts, fmt.Sprintf("SLO %s error budget warning: %.1f%% budget remaining", s.Name, s.BudgetRemainingPct))
		}
	}
	return hasBreach, alerts
}

