package ranking

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type BenchmarkResult struct {
	TotalRequests      int           `json:"total_requests"`
	SuccessfulRequests int           `json:"successful_requests"`
	ErrorRequests      int           `json:"error_requests"`
	CacheHits          int           `json:"cache_hits"`
	CacheMisses        int           `json:"cache_misses"`
	CacheHitRatio      float64       `json:"cache_hit_ratio"`
	Duration           time.Duration `json:"duration"`
	QPS                float64       `json:"qps"`
	ErrorRate          float64       `json:"error_rate"`
	P50MS              float64       `json:"p50_ms"`
	P90MS              float64       `json:"p90_ms"`
	P95MS              float64       `json:"p95_ms"`
	P99MS              float64       `json:"p99_ms"`
	MaxMS              float64       `json:"max_ms"`
	MeanMS             float64       `json:"mean_ms"`
}

type BenchmarkInstrumenter struct{}

func NewBenchmarkInstrumenter() *BenchmarkInstrumenter {
	return &BenchmarkInstrumenter{}
}

// RunLoadBenchmark simulates concurrent search queries and records detailed latency SLO metrics.
func (b *BenchmarkInstrumenter) RunLoadBenchmark(
	concurrency int,
	totalRequests int,
	fn func(query string) (cacheHit bool, err error),
	queries []string,
) BenchmarkResult {
	if concurrency <= 0 {
		concurrency = 1
	}
	if totalRequests <= 0 {
		totalRequests = len(queries)
	}
	if len(queries) == 0 {
		queries = []string{"default"}
	}

	latencies := make([]float64, 0, totalRequests)
	var latMu sync.Mutex

	var (
		successCount int64
		errorCount   int64
		cacheHitCount int64
		cacheMissCount int64
	)

	queryChan := make(chan string, totalRequests)
	for i := 0; i < totalRequests; i++ {
		queryChan <- queries[i%len(queries)]
	}
	close(queryChan)

	var wg sync.WaitGroup
	start := time.Now()

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localLats := make([]float64, 0, totalRequests/concurrency+1)

			for q := range queryChan {
				t0 := time.Now()
				hit, err := fn(q)
				elapsed := time.Since(t0).Seconds() * 1000.0 // ms
				localLats = append(localLats, elapsed)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}

				if hit {
					atomic.AddInt64(&cacheHitCount, 1)
				} else {
					atomic.AddInt64(&cacheMissCount, 1)
				}
			}

			latMu.Lock()
			latencies = append(latencies, localLats...)
			latMu.Unlock()
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	res := BenchmarkResult{
		TotalRequests:      totalRequests,
		SuccessfulRequests: int(successCount),
		ErrorRequests:      int(errorCount),
		CacheHits:          int(cacheHitCount),
		CacheMisses:        int(cacheMissCount),
		Duration:           duration,
	}

	if totalRequests > 0 {
		res.ErrorRate = float64(errorCount) / float64(totalRequests)
		res.CacheHitRatio = float64(cacheHitCount) / float64(totalRequests)
	}

	if duration.Seconds() > 0 {
		res.QPS = float64(totalRequests) / duration.Seconds()
	}

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		n := len(latencies)
		var totalLat float64
		for _, l := range latencies {
			totalLat += l
		}
		res.MeanMS = totalLat / float64(n)
		res.P50MS = latencies[int(float64(n)*0.50)]
		res.P90MS = latencies[int(float64(n)*0.90)]
		res.P95MS = latencies[int(float64(n)*0.95)]
		p99Idx := int(float64(n) * 0.99)
		if p99Idx >= n {
			p99Idx = n - 1
		}
		res.P99MS = latencies[p99Idx]
		res.MaxMS = latencies[n-1]
	}

	return res
}

