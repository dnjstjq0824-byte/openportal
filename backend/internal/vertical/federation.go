package vertical

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// VerticalType designates a specialized search vertical.
type VerticalType string

const (
	VerticalWeb      VerticalType = "web"
	VerticalImage    VerticalType = "image"
	VerticalVideo    VerticalType = "video"
	VerticalNews     VerticalType = "news"
	VerticalMap      VerticalType = "map"
	VerticalShopping VerticalType = "shopping"
	VerticalBooks    VerticalType = "books"
)

// VerticalQuery carries universal search parameters for any vertical.
type VerticalQuery struct {
	Vertical   VerticalType      `json:"vertical"`
	Query      string            `json:"query"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	SafeSearch string            `json:"safe_search"`
	Filters    map[string]string `json:"filters,omitempty"`
}

// VerticalResponse wraps unified pagination and payload for a vertical search.
type VerticalResponse struct {
	Vertical     VerticalType `json:"vertical"`
	Query        string       `json:"query"`
	Page         int          `json:"page"`
	TotalHits    int          `json:"total_hits"`
	Data         any          `json:"data"`
	ExecutionMs  int64        `json:"execution_ms"`
	Cached       bool         `json:"cached"`
	Attribution  string       `json:"attribution,omitempty"`
}

// TokenBucketLimiter implements a per-vertical thread-safe rate limiter.
type TokenBucketLimiter struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	fillRate   float64 // tokens per second
	lastUpdate time.Time
}

// NewTokenBucketLimiter creates a limiter with capacity and refill rate.
func NewTokenBucketLimiter(capacity, refillRate float64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		capacity:   capacity,
		tokens:     capacity,
		fillRate:   refillRate,
		lastUpdate: time.Now(),
	}
}

// Allow checks and consumes 1 token if available.
func (tb *TokenBucketLimiter) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.tokens = tb.tokens + elapsed*tb.fillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastUpdate = now

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

type cacheItem struct {
	response  VerticalResponse
	expiresAt time.Time
}

// VerticalFederationCoordinator manages cross-vertical queries, rate limiting, and caching.
type VerticalFederationCoordinator struct {
	mu          sync.RWMutex
	limiters    map[VerticalType]*TokenBucketLimiter
	cache       map[string]cacheItem
	cacheTTL    time.Duration
	timeout     time.Duration
}

// NewVerticalFederationCoordinator initializes the federation coordinator.
func NewVerticalFederationCoordinator(cacheTTL, timeout time.Duration) *VerticalFederationCoordinator {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	if timeout <= 0 {
		timeout = 150 * time.Millisecond
	}

	limiters := map[VerticalType]*TokenBucketLimiter{
		VerticalImage:    NewTokenBucketLimiter(100, 50),
		VerticalVideo:    NewTokenBucketLimiter(80, 40),
		VerticalNews:     NewTokenBucketLimiter(120, 60),
		VerticalMap:      NewTokenBucketLimiter(60, 30),
		VerticalShopping: NewTokenBucketLimiter(80, 40),
	}

	return &VerticalFederationCoordinator{
		limiters: limiters,
		cache:    make(map[string]cacheItem),
		cacheTTL: cacheTTL,
		timeout:  timeout,
	}
}

// MakeCacheKey generates an idempotent cache key for a vertical query.
func MakeCacheKey(vq VerticalQuery) string {
	return fmt.Sprintf("%s:%s:%d:%d:%s", vq.Vertical, vq.Query, vq.Page, vq.PageSize, vq.SafeSearch)
}

// ExecuteWithTimeout dispatches a vertical retrieval function under rate-limit, cache, and context timeout constraints.
func (c *VerticalFederationCoordinator) Execute(
	ctx context.Context,
	vq VerticalQuery,
	fetcher func(ctx context.Context, vq VerticalQuery) (any, int, string, error),
) (*VerticalResponse, error) {
	// 1. Check rate limit
	if lim, ok := c.limiters[vq.Vertical]; ok {
		if !lim.Allow() {
			return nil, errors.New("rate limit exceeded for requested vertical")
		}
	}

	// 2. Check cache
	key := MakeCacheKey(vq)
	c.mu.RLock()
	item, found := c.cache[key]
	c.mu.RUnlock()
	if found && time.Now().Before(item.expiresAt) {
		cachedResp := item.response
		cachedResp.Cached = true
		return &cachedResp, nil
	}

	// 3. Execute with timeout
	queryCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	data, total, attribution, err := fetcher(queryCtx, vq)
	if err != nil {
		return nil, err
	}
	execMs := time.Since(start).Milliseconds()

	resp := VerticalResponse{
		Vertical:    vq.Vertical,
		Query:       vq.Query,
		Page:        vq.Page,
		TotalHits:   total,
		Data:        data,
		ExecutionMs: execMs,
		Cached:      false,
		Attribution: attribution,
	}

	// 4. Save to cache
	c.mu.Lock()
	c.cache[key] = cacheItem{
		response:  resp,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
	c.mu.Unlock()

	return &resp, nil
}
