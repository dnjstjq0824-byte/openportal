package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
	_ "github.com/lib/pq"
)

func okUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"timed_out":false,"_shards":{"failed":0},"hits":{"total":{"value":0},"hits":[]}}`))
	}))
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal readyz body: %v", err)
	}
	return body
}

// B-2: without a configured database, readiness reflects OpenSearch only.
func TestReadyzReadyWithoutDatabase(t *testing.T) {
	upstream := okUpstream(t)
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore()}

	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["status"] != "ready" || body["opensearch"] != "ok" || body["postgres"] != "not_configured" {
		t.Fatalf("unexpected ready body: %v", body)
	}
}

// B-2 red drill: OpenSearch down must yield 503 with the failing check named.
func TestReadyzFailsWhenOpenSearchDown(t *testing.T) {
	upstream := okUpstream(t)
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore()}
	upstream.Close() // kill the backend before the probe

	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["status"] != "unavailable" || body["check"] != "opensearch" || body["cause"] == "" {
		t.Fatalf("expected opensearch cause, got %v", body)
	}
}

// B-2 red drill: configured-but-unreachable PostgreSQL must yield 503 naming postgres.
func TestReadyzFailsWhenPostgresDown(t *testing.T) {
	upstream := okUpstream(t)
	defer upstream.Close()
	db, err := sql.Open("postgres", "postgres://127.0.0.1:1/blackhole?sslmode=disable&connect_timeout=2")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), db: db, idStore: identity.NewStore()}

	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["check"] != "postgres" || body["cause"] == "" || body["opensearch"] != "ok" {
		t.Fatalf("expected postgres cause with opensearch ok, got %v", body)
	}
}
