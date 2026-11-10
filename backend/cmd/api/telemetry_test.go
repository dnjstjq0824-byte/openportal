package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
)

// B-5: /metrics exposes Prometheus counters for served production routes.
func TestMetricsExposition(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"timed_out":false,"_shards":{"failed":0},"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore()}
	handler := a.productionRoutes()

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatalf("readyz: got %d", w.Code)
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 200 {
		t.Fatalf("metrics: got %d", w.Code)
	}
	if ctype := w.Header().Get("Content-Type"); !strings.Contains(ctype, "text/plain") {
		t.Fatalf("metrics content type: %q", ctype)
	}
	body := w.Body.String()
	for _, want := range []string{
		"# HELP openportal_api_requests_total",
		"# TYPE openportal_api_requests_total counter",
		`openportal_api_requests_total{method="GET",path="/readyz",code="200"}`,
		"openportal_api_request_duration_seconds_count",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics exposition missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `path="/metrics"`) {
		t.Fatal("metrics must not count scrapes of itself")
	}
}

// B-5: each request emits one structured JSON log line.
func TestTelemetryJSONLog(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(original)
	defer log.SetFlags(originalFlags)
	a := &app{idStore: identity.NewStore()}
	w := httptest.NewRecorder()
	a.productionRoutes().ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 {
		t.Fatalf("healthz: got %d", w.Code)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one log line, got %d: %q", len(lines), buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, lines[0])
	}
	for _, key := range []string{"ts", "request_id", "method", "path", "status", "latency_ms"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("log entry missing %q: %v", key, entry)
		}
	}
	if entry["method"] != "GET" || entry["path"] != "/healthz" || entry["status"] != float64(200) {
		t.Fatalf("unexpected log entry: %v", entry)
	}
}
