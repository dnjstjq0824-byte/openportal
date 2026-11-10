package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// B-5: dependency-free request telemetry. Counters live in process memory (one
// replica = one series set; aggregate in Prometheus). The exposition format is
// Prometheus text 0.0.4.

type methodPathCode struct {
	method string
	path   string
	code   int
}

type latencyCounts struct {
	count uint64
	sum   float64
}

var (
	telemetryMu      sync.Mutex
	telemetryCounts  = map[methodPathCode]*latencyCounts{}
	telemetrySkipped = map[string]bool{"/metrics": true}
)

func telemetryRecord(method, path string, code int, elapsed time.Duration) {
	if telemetrySkipped[path] {
		return
	}
	telemetryMu.Lock()
	defer telemetryMu.Unlock()
	key := methodPathCode{method: method, path: path, code: code}
	counts, ok := telemetryCounts[key]
	if !ok {
		counts = &latencyCounts{}
		telemetryCounts[key] = counts
	}
	counts.count++
	counts.sum += elapsed.Seconds()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withTelemetry records Prometheus counters and emits one structured JSON log
// line per request: request_id, method, path, status, latency_ms.
func (a *app) withTelemetry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := resolveTraceID(r)
		recorder := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(recorder, r)
		elapsed := time.Since(start)
		telemetryRecord(r.Method, r.URL.Path, recorder.status, elapsed)
		log.Printf(`{"ts":%q,"request_id":%q,"method":%q,"path":%q,"status":%d,"latency_ms":%.3f}`,
			start.UTC().Format(time.RFC3339Nano), requestID, r.Method, r.URL.Path, recorder.status, float64(elapsed.Microseconds())/1000.0)
	})
}

func (a *app) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	telemetryMu.Lock()
	keys := make([]methodPathCode, 0, len(telemetryCounts))
	for key := range telemetryCounts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].code < keys[j].code
	})
	var totals, durations strings.Builder
	totals.WriteString("# HELP openportal_api_requests_total API requests served.\n# TYPE openportal_api_requests_total counter\n")
	durations.WriteString("# HELP openportal_api_request_duration_seconds API request latency.\n# TYPE openportal_api_request_duration_seconds summary\n")
	for _, key := range keys {
		counts := telemetryCounts[key]
		fmt.Fprintf(&totals, "openportal_api_requests_total{method=%q,path=%q,code=%q} %d\n",
			key.method, key.path, fmt.Sprint(key.code), counts.count)
		fmt.Fprintf(&durations, "openportal_api_request_duration_seconds_sum{method=%q,path=%q,code=%q} %f\n",
			key.method, key.path, fmt.Sprint(key.code), counts.sum)
		fmt.Fprintf(&durations, "openportal_api_request_duration_seconds_count{method=%q,path=%q,code=%q} %d\n",
			key.method, key.path, fmt.Sprint(key.code), counts.count)
	}
	telemetryMu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(totals.String() + durations.String()))
}
