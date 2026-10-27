package vertical

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(2.0, 10.0) // 2 tokens capacity, refills 10/s

	if !limiter.Allow() {
		t.Errorf("expected 1st token allowed")
	}
	if !limiter.Allow() {
		t.Errorf("expected 2nd token allowed")
	}
	if limiter.Allow() {
		t.Errorf("expected 3rd token rejected (exhausted)")
	}

	// Sleep 250ms -> should refill ~2.5 tokens
	time.Sleep(250 * time.Millisecond)
	if !limiter.Allow() {
		t.Errorf("expected refilled token allowed")
	}
}

func TestFederationCoordinatorCachingAndTimeout(t *testing.T) {
	coord := NewVerticalFederationCoordinator(1*time.Minute, 100*time.Millisecond)

	vq := VerticalQuery{
		Vertical: VerticalImage,
		Query:    "mountains",
		Page:     1,
		PageSize: 10,
	}

	callCount := 0
	fetcher := func(ctx context.Context, q VerticalQuery) (any, int, string, error) {
		callCount++
		return []string{"img1.jpg", "img2.jpg"}, 2, "© Verified", nil
	}

	// 1st call -> Cache miss
	resp1, err := coord.Execute(context.Background(), vq, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp1.Cached {
		t.Errorf("expected 1st call to be cache miss")
	}
	if callCount != 1 {
		t.Errorf("expected callCount 1, got %d", callCount)
	}

	// 2nd call -> Cache hit
	resp2, err := coord.Execute(context.Background(), vq, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp2.Cached {
		t.Errorf("expected 2nd call to be cached")
	}
	if callCount != 1 {
		t.Errorf("expected callCount still 1 on cache hit, got %d", callCount)
	}

	// 3rd call with slow fetcher exceeding timeout (100ms)
	slowFetcher := func(ctx context.Context, q VerticalQuery) (any, int, string, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return "done", 1, "", nil
		case <-ctx.Done():
			return nil, 0, "", ctx.Err()
		}
	}
	slowVQ := VerticalQuery{Vertical: VerticalVideo, Query: "slow video", Page: 1}
	_, err = coord.Execute(context.Background(), slowVQ, slowFetcher)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context deadline exceeded error, got: %v", err)
	}
}
