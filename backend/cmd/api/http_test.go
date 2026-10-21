package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/openportal-search/backend/internal/search"
)

func TestHTTPBoundary(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), corsOrigin: "http://127.0.0.1:5173"}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/search", 400}, {"GET", "/api/v1/search?q=x&page=101", 400},
		{"GET", "/api/v1/search?q=x&lang=xx", 400}, {"POST", "/api/v1/search?q=x", 405},
		{"POST", "/api/v1/suggest?q=x", 405}, {"GET", "/api/v1/suggest?q=", 400},
		{"GET", "/admin", 404}, {"GET", "/api/v1/admin", 404}, {"GET", "/partner", 404},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
	}
	if calls != 0 {
		t.Fatal("invalid requests reached upstream")
	}
	for _, path := range []string{"/api/v1/search?q=한글", "/api/v1/suggest?q=Open", "/readyz"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Origin", "https://untrusted.example")
		a.routes().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("untrusted origin allowed")
		}
		if !json.Valid(w.Body.Bytes()) {
			t.Fatal("invalid JSON")
		}
	}
	upstream.Close()
	for _, path := range []string{"/api/v1/search?q=test", "/api/v1/suggest?q=test", "/readyz"} {
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 503 {
			t.Fatalf("%s unavailable returned %d", path, w.Code)
		}
	}
}
