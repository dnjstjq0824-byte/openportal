package ranking

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type ExperimentStatus string

const (
	ExpStatusDraft      ExperimentStatus = "draft"
	ExpStatusRunning    ExperimentStatus = "running"
	ExpStatusPaused     ExperimentStatus = "paused"
	ExpStatusRolledBack ExperimentStatus = "rolled_back"
	ExpStatusConcluded  ExperimentStatus = "concluded"
)

type Variant string

const (
	VariantControl   Variant = "control"
	VariantTreatment Variant = "treatment"
)

type ExperimentDef struct {
	ID                     string           `json:"id"`
	Name                   string           `json:"name"`
	Layer                  string           `json:"layer"` // e.g. "ranking", "ui", "crawler"
	Salt                   string           `json:"salt"`
	SplitPercent           int              `json:"split_percent"` // 0..100 for treatment
	Status                 ExperimentStatus `json:"status"`
	ControlProfile         string           `json:"control_profile"`
	TreatmentProfile       string           `json:"treatment_profile"`
	MaxErrorRateThreshold  float64          `json:"max_error_rate_threshold"`  // e.g. 0.05 (5%)
	MaxP99LatencyThreshold float64          `json:"max_p99_latency_threshold"` // e.g. 500.0 ms
	MinSampleSize          int              `json:"min_sample_size"`
	RollbackReason         string           `json:"rollback_reason,omitempty"`
	RolledBackAt           *time.Time       `json:"rolled_back_at,omitempty"`
}

type ExperimentMetrics struct {
	mu           sync.Mutex
	totalReqs    int
	errorReqs    int
	latenciesMS  []float64
}

type ExperimentPlatform struct {
	mu          sync.RWMutex
	experiments map[string]*ExperimentDef
	layerLock   map[string]string // layer -> active experimentID (mutual exclusion)
	metrics     map[string]*ExperimentMetrics // expID:variant -> metrics
}

func NewExperimentPlatform() *ExperimentPlatform {
	return &ExperimentPlatform{
		experiments: make(map[string]*ExperimentDef),
		layerLock:   make(map[string]string),
		metrics:     make(map[string]*ExperimentMetrics),
	}
}

// RegisterExperiment registers an experiment and enforces layer mutual exclusion.
func (p *ExperimentPlatform) RegisterExperiment(exp ExperimentDef) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if exp.ID == "" || exp.Layer == "" {
		return errors.New("experiment ID and Layer are required")
	}
	if exp.SplitPercent < 0 || exp.SplitPercent > 100 {
		return errors.New("split_percent must be 0..100")
	}
	if exp.MinSampleSize <= 0 {
		exp.MinSampleSize = 20
	}
	if exp.MaxErrorRateThreshold <= 0 {
		exp.MaxErrorRateThreshold = 0.05
	}
	if exp.MaxP99LatencyThreshold <= 0 {
		exp.MaxP99LatencyThreshold = 500.0
	}

	// Layer mutual exclusion: ensure no other running experiment occupies the same layer
	if exp.Status == ExpStatusRunning {
		if activeID, exists := p.layerLock[exp.Layer]; exists && activeID != exp.ID {
			return fmt.Errorf("layer %q already occupied by running experiment %q (mutual exclusion)", exp.Layer, activeID)
		}
		p.layerLock[exp.Layer] = exp.ID
	}

	p.experiments[exp.ID] = &exp
	p.metrics[exp.ID+":control"] = &ExperimentMetrics{}
	p.metrics[exp.ID+":treatment"] = &ExperimentMetrics{}
	return nil
}

// AssignDeterministic calculates a deterministic bucket (0..99) using SHA-256.
func (p *ExperimentPlatform) AssignDeterministic(expID, identifier string) (Variant, string, error) {
	p.mu.RLock()
	exp, exists := p.experiments[expID]
	p.mu.RUnlock()

	if !exists {
		return VariantControl, "", errors.New("experiment not found")
	}

	// If experiment is paused, rolled back, or not running, always fallback to control profile
	if exp.Status != ExpStatusRunning {
		return VariantControl, exp.ControlProfile, nil
	}

	h := sha256.New()
	h.Write([]byte(exp.ID + ":" + exp.Salt + ":" + identifier))
	sum := h.Sum(nil)

	bucketVal := binary.BigEndian.Uint32(sum[:4])
	bucket := int(bucketVal % 100) // 0..99

	// 0 .. (splitPercent - 1) assigned to Treatment, remainder to Control
	if bucket < exp.SplitPercent {
		return VariantTreatment, exp.TreatmentProfile, nil
	}
	return VariantControl, exp.ControlProfile, nil
}

// RecordMetric records telemetry for a variant and checks the automated rollback circuit breaker.
func (p *ExperimentPlatform) RecordMetric(expID string, variant Variant, isError bool, latencyMS float64) {
	p.mu.RLock()
	exp, exists := p.experiments[expID]
	p.mu.RUnlock()

	if !exists || exp.Status != ExpStatusRunning {
		return
	}

	key := expID + ":" + string(variant)
	p.mu.RLock()
	m, mOk := p.metrics[key]
	p.mu.RUnlock()
	if !mOk {
		return
	}

	m.mu.Lock()
	m.totalReqs++
	if isError {
		m.errorReqs++
	}
	m.latenciesMS = append(m.latenciesMS, latencyMS)
	total := m.totalReqs
	errs := m.errorReqs
	lats := make([]float64, len(m.latenciesMS))
	copy(lats, m.latenciesMS)
	m.mu.Unlock()

	// Circuit breaker evaluation only triggers on treatment anomalies
	if variant == VariantTreatment && total >= exp.MinSampleSize {
		errRate := float64(errs) / float64(total)
		sort.Float64s(lats)
		p99Idx := int(float64(len(lats)) * 0.99)
		if p99Idx >= len(lats) {
			p99Idx = len(lats) - 1
		}
		p99Lat := lats[p99Idx]

		if errRate > exp.MaxErrorRateThreshold {
			p.TriggerEmergencyRollback(expID, fmt.Sprintf("error rate %.2f%% exceeded threshold %.2f%%", errRate*100, exp.MaxErrorRateThreshold*100))
		} else if p99Lat > exp.MaxP99LatencyThreshold {
			p.TriggerEmergencyRollback(expID, fmt.Sprintf("latency p99 %.1fms exceeded threshold %.1fms", p99Lat, exp.MaxP99LatencyThreshold))
		}
	}
}

// TriggerEmergencyRollback automatically deactivates treatment and returns traffic to control.
func (p *ExperimentPlatform) TriggerEmergencyRollback(expID, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	exp, ok := p.experiments[expID]
	if !ok || exp.Status == ExpStatusRolledBack {
		return
	}

	now := time.Now()
	exp.Status = ExpStatusRolledBack
	exp.RollbackReason = reason
	exp.RolledBackAt = &now

	// Release layer lock
	delete(p.layerLock, exp.Layer)
}

