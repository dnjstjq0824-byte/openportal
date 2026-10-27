package launch

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

// LatencyProfile tracks request latencies and calculates percentiles.
type LatencyProfile struct {
	mu        sync.RWMutex
	samples   []float64 // latency in milliseconds
	failures  int
	totalReqs int
}

// BenchmarkResult summarizes load test metrics (Task 091).
type BenchmarkResult struct {
	TotalRequests int       `json:"total_requests"`
	Successful    int       `json:"successful"`
	Failed        int       `json:"failed"`
	QPS           float64   `json:"qps"`
	P50Ms         float64   `json:"p50_ms"`
	P95Ms         float64   `json:"p95_ms"`
	P99Ms         float64   `json:"p99_ms"`
	ErrorRate     float64   `json:"error_rate"`
	MeasuredAt    time.Time `json:"measured_at"`
}

// NewLatencyProfile creates an empty profiler.
func NewLatencyProfile() *LatencyProfile {
	return &LatencyProfile{
		samples: make([]float64, 0, 1024),
	}
}

// RecordSample records a single execution latency.
func (p *LatencyProfile) RecordSample(latencyMs float64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalReqs++
	if err != nil {
		p.failures++
	} else {
		p.samples = append(p.samples, latencyMs)
	}
}

// CalculateResult computes QPS and latency percentiles given total elapsed duration.
func (p *LatencyProfile) CalculateResult(duration time.Duration) BenchmarkResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := BenchmarkResult{
		TotalRequests: p.totalReqs,
		Successful:    len(p.samples),
		Failed:        p.failures,
		MeasuredAt:    time.Now().UTC(),
	}

	if duration.Seconds() > 0 {
		res.QPS = math.Round((float64(p.totalReqs)/duration.Seconds())*100) / 100
	}
	if p.totalReqs > 0 {
		res.ErrorRate = math.Round((float64(p.failures)/float64(p.totalReqs))*10000) / 10000
	}

	if len(p.samples) == 0 {
		return res
	}

	// Sort copy to calculate percentiles
	sorted := make([]float64, len(p.samples))
	copy(sorted, p.samples)
	sort.Float64s(sorted)

	res.P50Ms = percentile(sorted, 0.50)
	res.P95Ms = percentile(sorted, 0.95)
	res.P99Ms = percentile(sorted, 0.99)

	return res
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return math.Round(sorted[idx]*100) / 100
}

// ClusterCapacityModel plans OpenSearch shard & replica allocation (Task 092).
type ClusterCapacityModel struct {
	TotalDocCount      int64   `json:"total_doc_count"`
	AvgDocSizeBytes    int64   `json:"avg_doc_size_bytes"`
	TargetShardSizeGB  float64 `json:"target_shard_size_gb"` // recommended: 30 - 50 GB
	PrimaryShards      int     `json:"primary_shards"`
	ReplicaCount       int     `json:"replica_count"`
	TotalStorageGB     float64 `json:"total_storage_gb"`
	RecommendedNodeRAM string  `json:"recommended_node_ram"`
}

// PlanClusterCapacity computes required shards and capacity with safety headroom.
func PlanClusterCapacity(docCount int64, avgSizeBytes int64, replicaCount int) (*ClusterCapacityModel, error) {
	if docCount <= 0 || avgSizeBytes <= 0 {
		return nil, errors.New("doc count and average doc size must be positive")
	}
	if replicaCount < 0 {
		replicaCount = 1
	}

	totalDataBytes := float64(docCount * avgSizeBytes)
	// OpenSearch indexing overhead (~1.3x)
	totalIndexBytes := totalDataBytes * 1.3
	totalIndexGB := totalIndexBytes / (1024 * 1024 * 1024)

	targetShardGB := 30.0
	primaryShards := int(math.Ceil(totalIndexGB / targetShardGB))
	if primaryShards < 1 {
		primaryShards = 1
	}

	totalStorageGB := totalIndexGB * float64(1+replicaCount) * 1.25 // 25% disk watermark headroom

	ramTier := "16GB"
	if totalStorageGB > 500 {
		ramTier = "32GB"
	}
	if totalStorageGB > 2000 {
		ramTier = "64GB"
	}

	return &ClusterCapacityModel{
		TotalDocCount:      docCount,
		AvgDocSizeBytes:    avgSizeBytes,
		TargetShardSizeGB:  targetShardGB,
		PrimaryShards:      primaryShards,
		ReplicaCount:       replicaCount,
		TotalStorageGB:     math.Round(totalStorageGB*100) / 100,
		RecommendedNodeRAM: ramTier,
	}, nil
}
