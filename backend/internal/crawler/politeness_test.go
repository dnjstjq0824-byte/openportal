package crawler

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestPolitenessManagerRateLimit(t *testing.T) {
	manager := NewPolitenessManager(100*time.Millisecond, 100)
	ctx := context.Background()
	hostKey := "https://example.com:443"

	start := time.Now()
	// First acquire should be immediate
	err := manager.Acquire(ctx, hostKey, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	manager.Release(hostKey, 200, false)

	// Second acquire must wait at least 100ms
	err = manager.Acquire(ctx, hostKey, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 90*time.Millisecond {
		t.Errorf("expected politeness delay >= 90ms, got %v", elapsed)
	}
	manager.Release(hostKey, 200, false)
}

func TestPolitenessManagerBackoffOn429(t *testing.T) {
	manager := NewPolitenessManager(50*time.Millisecond, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	hostKey := "https://rate-limited.com:443"

	// Acquire and release with HTTP 429 (Too Many Requests)
	err := manager.Acquire(ctx, hostKey, 0)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	manager.Release(hostKey, http.StatusTooManyRequests, false)

	// Next acquire must wait for backoff (at least 2 seconds), so our 200ms context should timeout
	err = manager.Acquire(ctx, hostKey, 0)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded during backoff, got %v", err)
	}
}

func TestConditionalHeaders(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://example.com/page", nil)
	ConditionalHeaders(req, `"etag-hash-123"`, "Mon, 20 Oct 2026 12:00:00 GMT")

	if req.Header.Get("If-None-Match") != `"etag-hash-123"` {
		t.Errorf("If-None-Match header mismatch: %s", req.Header.Get("If-None-Match"))
	}
	if req.Header.Get("If-Modified-Since") != "Mon, 20 Oct 2026 12:00:00 GMT" {
		t.Errorf("If-Modified-Since header mismatch: %s", req.Header.Get("If-Modified-Since"))
	}
}

