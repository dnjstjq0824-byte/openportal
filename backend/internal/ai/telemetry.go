package ai

import (
	"sync"
	"time"
)

// QueryTelemetry records latency, token count, and cost for an AI query invocation.
type QueryTelemetry struct {
	QueryHash        string    `json:"query_hash"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	LatencyMs        int64     `json:"latency_ms"`
	Timestamp        time.Time `json:"timestamp"`
}

// AITelemetryTracker aggregates real-time usage metrics and enforces global monthly/daily spend caps.
type AITelemetryTracker struct {
	mu           sync.RWMutex
	records      []QueryTelemetry
	totalCostUSD float64
	dailyCapUSD  float64
}

// NewAITelemetryTracker creates an instance with a daily cost ceiling.
func NewAITelemetryTracker(dailyCapUSD float64) *AITelemetryTracker {
	if dailyCapUSD <= 0 {
		dailyCapUSD = 100.0 // Default $100 daily limit
	}
	return &AITelemetryTracker{
		dailyCapUSD: dailyCapUSD,
	}
}

// RecordQuery adds an execution record and updates total expenditure.
func (t *AITelemetryTracker) RecordQuery(qHash string, pTokens, cTokens int, latencyMs int64) (QueryTelemetry, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cost := float64(pTokens+cTokens) * 0.000002 // $0.002 per 1,000 tokens
	rec := QueryTelemetry{
		QueryHash:        qHash,
		PromptTokens:     pTokens,
		CompletionTokens: cTokens,
		CostUSD:          cost,
		LatencyMs:        latencyMs,
		Timestamp:        time.Now(),
	}

	t.records = append(t.records, rec)
	t.totalCostUSD += cost

	exceeded := t.totalCostUSD >= t.dailyCapUSD
	return rec, exceeded
}

// IsDailyCapReached returns whether spend exceeds cap.
func (t *AITelemetryTracker) IsDailyCapReached() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalCostUSD >= t.dailyCapUSD
}

// GetSummary returns total spend and count.
func (t *AITelemetryTracker) GetSummary() (int, float64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.records), t.totalCostUSD
}
