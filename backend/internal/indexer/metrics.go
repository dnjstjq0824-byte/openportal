package indexer

import (
	"sort"
	"sync"
	"time"
)

// IndexerSnapshot captures telemetry for crawler health, indexing lag, and tombstone propagation.
type IndexerSnapshot struct {
	TotalCrawled           int64   `json:"total_crawled"`
	SuccessfulCrawls       int64   `json:"successful_crawls"`
	FailedCrawls           int64   `json:"failed_crawls"`
	SuccessRate            float64 `json:"success_rate"`
	LatencyP50Ms           int64   `json:"latency_p50_ms"`
	LatencyP95Ms           int64   `json:"latency_p95_ms"`
	TotalIndexed           int64   `json:"total_indexed"`
	AverageIndexLagSec     float64 `json:"average_index_lag_sec"`
	AverageFreshnessHours  float64 `json:"average_freshness_hours"`
	OutboxPendingEvents    int     `json:"outbox_pending_events"`
	TombstonesApplied      int64   `json:"tombstones_applied"`
	TombstonesPurged       int64   `json:"tombstones_purged"`
	NearDuplicatesDetected int64   `json:"near_duplicates_detected"`
}

// MetricsCollector tracks operational latency, freshness, lag, and outbox depth.
type MetricsCollector struct {
	mu                     sync.RWMutex
	totalCrawled           int64
	successfulCrawls       int64
	failedCrawls           int64
	latencies              []int64
	totalIndexed           int64
	totalIndexLagSec       float64
	totalFreshnessHours    float64
	tombstonesApplied      int64
	tombstonesPurged       int64
	nearDuplicatesDetected int64
	outbox                 *OutboxQueue
}

// NewMetricsCollector creates an in-memory metrics collector.
func NewMetricsCollector(outbox *OutboxQueue) *MetricsCollector {
	return &MetricsCollector{
		latencies: make([]int64, 0, 1000),
		outbox:    outbox,
	}
}

// RecordCrawl records a crawl outcome with latency in milliseconds.
func (m *MetricsCollector) RecordCrawl(success bool, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalCrawled++
	if success {
		m.successfulCrawls++
	} else {
		m.failedCrawls++
	}

	if len(m.latencies) < 5000 {
		m.latencies = append(m.latencies, latencyMs)
	}
}

// RecordIndexed records successful indexing of a document with lag and freshness.
func (m *MetricsCollector) RecordIndexed(crawledAt time.Time, publishedAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	lag := now.Sub(crawledAt).Seconds()
	if lag < 0 {
		lag = 0
	}

	freshnessHours := 0.0
	if !publishedAt.IsZero() {
		freshnessHours = now.Sub(publishedAt).Hours()
		if freshnessHours < 0 {
			freshnessHours = 0
		}
	}

	m.totalIndexed++
	m.totalIndexLagSec += lag
	m.totalFreshnessHours += freshnessHours
}

// RecordTombstone records an applied or purged tombstone.
func (m *MetricsCollector) RecordTombstone(purged bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if purged {
		m.tombstonesPurged++
	} else {
		m.tombstonesApplied++
	}
}

// RecordNearDuplicate records a detected near-duplicate document.
func (m *MetricsCollector) RecordNearDuplicate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nearDuplicatesDetected++
}

// Snapshot returns the current aggregated telemetry snapshot.
func (m *MetricsCollector) Snapshot() IndexerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rate := 0.0
	if m.totalCrawled > 0 {
		rate = float64(m.successfulCrawls) / float64(m.totalCrawled)
	}

	p50, p95 := calculatePercentiles(m.latencies)

	avgLag := 0.0
	avgFresh := 0.0
	if m.totalIndexed > 0 {
		avgLag = m.totalIndexLagSec / float64(m.totalIndexed)
		avgFresh = m.totalFreshnessHours / float64(m.totalIndexed)
	}

	outboxCount := 0
	if m.outbox != nil {
		outboxCount = m.outbox.PendingCount()
	}

	return IndexerSnapshot{
		TotalCrawled:           m.totalCrawled,
		SuccessfulCrawls:       m.successfulCrawls,
		FailedCrawls:           m.failedCrawls,
		SuccessRate:            rate,
		LatencyP50Ms:           p50,
		LatencyP95Ms:           p95,
		TotalIndexed:           m.totalIndexed,
		AverageIndexLagSec:     avgLag,
		AverageFreshnessHours:  avgFresh,
		OutboxPendingEvents:    outboxCount,
		TombstonesApplied:      m.tombstonesApplied,
		TombstonesPurged:       m.tombstonesPurged,
		NearDuplicatesDetected: m.nearDuplicatesDetected,
	}
}

func calculatePercentiles(samples []int64) (p50 int64, p95 int64) {
	if len(samples) == 0 {
		return 0, 0
	}
	sorted := make([]int64, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	p50Idx := len(sorted) * 50 / 100
	p95Idx := len(sorted) * 95 / 100

	if p50Idx >= len(sorted) {
		p50Idx = len(sorted) - 1
	}
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}

	return sorted[p50Idx], sorted[p95Idx]
}

