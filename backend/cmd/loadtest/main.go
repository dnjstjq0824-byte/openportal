// Command loadtest issues real HTTP search requests against a live target and
// reports measured throughput/latency. It replaces the previous fake benchmark
// that timed len(q) without any server call (A-4).
//
// Usage:
//
//	LOADTEST_TARGET_URL=http://127.0.0.1:8080/api/v1/search go run ./cmd/loadtest -qps 100 -duration 10 -concurrency 10
//
// Exit code is 1 when the target is missing/unreachable or when zero requests
// succeed. No pass/fail banner is printed against invented thresholds; the
// measured numbers are the result.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/launch"
)

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	return 1
}

func main() {
	os.Exit(run())
}

func run() int {
	target := flag.String("target", os.Getenv("LOADTEST_TARGET_URL"), "Live search endpoint base URL, e.g. http://127.0.0.1:8080/api/v1/search (or $LOADTEST_TARGET_URL)")
	targetQPS := flag.Int("qps", 100, "Target query throughput per second (pacing hint, not a claim)")
	durationSec := flag.Int("duration", 10, "Benchmark duration in seconds")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent worker threads")
	flag.Parse()

	if *target == "" {
		return fail("no target configured: pass -target or set LOADTEST_TARGET_URL to a live server URL")
	}
	if *targetQPS < 1 || *durationSec < 1 || *concurrency < 1 {
		return fail("qps, duration and concurrency must all be >= 1")
	}
	base, err := url.Parse(*target)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return fail("invalid target URL %q: want http(s)://host[:port]/path", *target)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// Preflight: one real request. A down target fails fast instead of
	// producing fabricated percentiles.
	preflightURL := *base
	preflightQuery := preflightURL.Query()
	preflightQuery.Set("q", "preflight")
	preflightQuery.Set("page", "1")
	preflightURL.RawQuery = preflightQuery.Encode()
	preflightReq, err := http.NewRequest(http.MethodGet, preflightURL.String(), nil)
	if err != nil {
		return fail("cannot build preflight request: %v", err)
	}
	preflightResp, err := client.Do(preflightReq)
	if err != nil {
		return fail("target %s unreachable: %v", *target, err)
	}
	_ = preflightResp.Body.Close()
	if preflightResp.StatusCode >= 500 {
		return fail("target %s preflight returned HTTP %d", *target, preflightResp.StatusCode)
	}

	fmt.Println("==================================================================")
	fmt.Printf(" OpenPortal Load Benchmark (live target: %s)\n", *target)
	fmt.Println("==================================================================")
	fmt.Printf("Workers: %d | Duration: %ds | Target Rate: %d req/s\n\n", *concurrency, *durationSec, *targetQPS)

	profile := launch.NewLatencyProfile()
	circuitBreaker := launch.NewCircuitBreaker(50, 100*time.Millisecond)

	queries := []string{
		"검색엔진",
		"OpenScanner",
		"한국어 Nori",
		"분산 아키텍처",
		"고가용성 클러스터",
		"BM25 랭킹",
		"RRF 알고리즘",
	}

	deadline := time.Now().Add(time.Duration(*durationSec) * time.Second)
	// Per-worker pacing toward the aggregate target rate.
	perWorkerInterval := time.Second * time.Duration(*concurrency) / time.Duration(*targetQPS)

	var wg sync.WaitGroup
	start := time.Now()

	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			var ticker *time.Ticker
			if perWorkerInterval > 0 {
				ticker = time.NewTicker(perWorkerInterval)
				defer ticker.Stop()
			}
			for time.Now().Before(deadline) {
				q := queries[rng.Intn(len(queries))]
				sampleStart := time.Now()

				err := circuitBreaker.Execute(func() error {
					requestURL := *base
					params := requestURL.Query()
					params.Set("q", q)
					params.Set("page", "1")
					requestURL.RawQuery = params.Encode()
					req, reqErr := http.NewRequest(http.MethodGet, requestURL.String(), nil)
					if reqErr != nil {
						return reqErr
					}
					resp, doErr := client.Do(req)
					if doErr != nil {
						return doErr
					}
					_ = resp.Body.Close()
					if resp.StatusCode >= 500 {
						return fmt.Errorf("upstream HTTP %d", resp.StatusCode)
					}
					return nil
				})

				latencyMs := float64(time.Since(sampleStart).Microseconds()) / 1000.0
				profile.RecordSample(latencyMs, err)
				if ticker != nil {
					<-ticker.C
				}
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(start)

	res := profile.CalculateResult(elapsed)

	fmt.Println("------------------------------------------------------------------")
	fmt.Println(" Measured Results (live traffic, no simulated samples)")
	fmt.Println("------------------------------------------------------------------")
	fmt.Printf(" Target:                  %s\n", *target)
	fmt.Printf(" Total Requests Executed: %d\n", res.TotalRequests)
	if res.TotalRequests > 0 {
		fmt.Printf(" Successful Requests:     %d (%.2f%%)\n", res.Successful, float64(res.Successful)/float64(res.TotalRequests)*100)
		fmt.Printf(" Failed Requests:         %d (%.2f%%)\n", res.Failed, float64(res.Failed)/float64(res.TotalRequests)*100)
	}
	fmt.Printf(" Error Rate:              %.4f\n", res.ErrorRate)
	fmt.Printf(" Elapsed Wall Clock:      %.3f s\n", elapsed.Seconds())
	fmt.Printf(" Achieved Throughput:     %.1f QPS\n", res.QPS)
	fmt.Printf(" Latency P50 (Median):    %.3f ms\n", res.P50Ms)
	fmt.Printf(" Latency P95:             %.3f ms\n", res.P95Ms)
	fmt.Printf(" Latency P99 (Tail):      %.3f ms\n", res.P99Ms)
	fmt.Println("------------------------------------------------------------------")

	if res.TotalRequests == 0 || res.Successful == 0 {
		return fail("zero successful responses from %s", *target)
	}
	fmt.Println("Done: numbers above are measured against the live target.")
	return 0
}
