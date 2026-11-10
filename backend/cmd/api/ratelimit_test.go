package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
)

func TestRateLimiterWindow(t *testing.T) {
	limit := newRateLimiter(3, time.Minute)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if ok, _ := limit.allow("198.51.100.7", now); !ok {
			t.Fatalf("request %d rejected inside the limit", i+1)
		}
	}
	ok, retry := limit.allow("198.51.100.7", now)
	if ok || retry <= 0 || retry > time.Minute {
		t.Fatalf("expected rejection with 0 < retry <= window, got ok=%v retry=%v", ok, retry)
	}
	if ok, _ := limit.allow("198.51.100.8", now); !ok {
		t.Fatalf("a different client must not share the window")
	}
	if ok, _ := limit.allow("198.51.100.7", now.Add(time.Minute)); !ok {
		t.Fatalf("window must reset after expiry")
	}
}

func TestClientIPSelection(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/search?q=x", nil)
	r.RemoteAddr = "203.0.113.9:4444"
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("RemoteAddr host = %q", got)
	}
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 203.0.113.9")
	if got := clientIP(r); got != "198.51.100.7" {
		t.Fatalf("X-Forwarded-For first entry = %q", got)
	}
	r.Header.Set("X-Forwarded-For", "   ")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("blank forwarded header must fall back, got %q", got)
	}
}

func TestProductionSearchRateLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
	h := a.productionRoutes()

	for i := 0; i < 130; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("healthz must stay unlimited, rejected at iteration %d", i)
		}
	}
	rejected := 0
	var retryAfter string
	for i := 0; i < 121; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=x", nil))
		if w.Code == http.StatusTooManyRequests {
			rejected++
			retryAfter = w.Header().Get("Retry-After")
			if body := w.Body.String(); len(body) == 0 || !strings.Contains(body, "RATE_LIMITED") {
				t.Fatalf("429 body must carry RATE_LIMITED, got %q", body)
			}
		}
	}
	if rejected != 1 {
		t.Fatalf("expected exactly 1 rejection in 121 requests, got %d", rejected)
	}
	if retryAfter == "" {
		t.Fatalf("429 must set Retry-After")
	}
	other := httptest.NewRequest("GET", "/api/v1/search?q=x", nil)
	other.Header.Set("X-Forwarded-For", "198.51.100.99")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, other)
	if w.Code == http.StatusTooManyRequests {
		t.Fatalf("a different client IP must not be limited")
	}
}
