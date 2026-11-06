package main

import (
	"sync"
	"time"
)

// rateLimiter is a per-client fixed-window brake for the public read API.
// It complements the gateway and ingress limits; direct callers that spoof
// X-Forwarded-For only move themselves into another bucket.
type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string]*rateWindow
	limit   int
	window  time.Duration
	maxKeys int
}

type rateWindow struct {
	count int
	start time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: make(map[string]*rateWindow), limit: limit, window: window, maxKeys: 100000}
}

// allow reports whether the request fits the caller's current window and, when
// it does not, how long until the window resets.
func (l *rateLimiter) allow(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) >= l.maxKeys {
		l.hits = make(map[string]*rateWindow)
	}
	w, ok := l.hits[ip]
	if !ok || now.Sub(w.start) >= l.window {
		l.hits[ip] = &rateWindow{count: 1, start: now}
		return true, 0
	}
	if w.count >= l.limit {
		return false, l.window - now.Sub(w.start)
	}
	w.count++
	return true, 0
}
