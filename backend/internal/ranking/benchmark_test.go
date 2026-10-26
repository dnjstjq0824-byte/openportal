package ranking

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBenchmarkInstrumenter(t *testing.T) {
	instr := NewBenchmarkInstrumenter()

	queries := []string{"오픈포털", "검색엔진", "아키텍처", "캐시테스트"}
	var cacheMu sync.Mutex
	cache := make(map[string]bool)

	// Simulated query execution function with ~2ms latency on cache miss, ~0.1ms on hit
	queryFunc := func(q string) (bool, error) {
		if q == "error_query" {
			return false, errors.New("simulated error")
		}

		cacheMu.Lock()
		hit := cache[q]
		if !hit {
			cache[q] = true
		}
		cacheMu.Unlock()

		if hit {
			time.Sleep(100 * time.Microsecond)
		} else {
			time.Sleep(2 * time.Millisecond)
		}
		return hit, nil
	}

	totalRequests := 50
	concurrency := 4

	res := instr.RunLoadBenchmark(concurrency, totalRequests, queryFunc, queries)

	if res.TotalRequests != totalRequests {
		t.Errorf("expected %d total requests, got %d", totalRequests, res.TotalRequests)
	}
	if res.SuccessfulRequests != totalRequests {
		t.Errorf("expected %d successful requests, got %d", totalRequests, res.SuccessfulRequests)
	}
	if res.ErrorRate != 0.0 {
		t.Errorf("expected 0 error rate, got %f", res.ErrorRate)
	}
	if res.CacheHits <= 0 {
		t.Errorf("expected some cache hits across %d requests, got %d", totalRequests, res.CacheHits)
	}
	if res.CacheHitRatio <= 0.5 {
		t.Errorf("expected cache hit ratio > 0.5 for 4 repeated queries, got %f", res.CacheHitRatio)
	}
	if res.QPS <= 0.0 {
		t.Errorf("expected positive QPS, got %f", res.QPS)
	}

	// Verify percentile ordering: p50 <= p90 <= p95 <= p99 <= max
	if res.P50MS > res.P90MS || res.P90MS > res.P95MS || res.P95MS > res.P99MS || res.P99MS > res.MaxMS {
		t.Errorf("percentiles out of order: p50=%f, p90=%f, p95=%f, p99=%f, max=%f",
			res.P50MS, res.P90MS, res.P95MS, res.P99MS, res.MaxMS)
	}
}

