package crawler

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrHostQuotaExceeded = errors.New("politeness: hourly request quota exceeded for host")
	ErrHostInBackoff     = errors.New("politeness: host is in active backoff period")
)

// HostState tracks concurrency, request timestamps, quotas, and backoff state for a specific host.
type HostState struct {
	mu           sync.Mutex
	HostKey      string
	CrawlDelay   time.Duration
	LastAccessAt time.Time
	ActiveReqs   int
	MaxReqs      int // concurrency limit (default: 1)
	BackoffUntil time.Time
	BackoffLevel int
	HourlyQuota  int
	HourlyCount  int
	QuotaResetAt time.Time
}

// PolitenessManager coordinates respectful, polite crawling across all target hosts.
type PolitenessManager struct {
	mu           sync.RWMutex
	hosts        map[string]*HostState
	DefaultDelay time.Duration
	MaxBackoff   time.Duration
	HourlyQuota  int
}

func NewPolitenessManager(defaultDelay time.Duration, hourlyQuota int) *PolitenessManager {
	if defaultDelay < 500*time.Millisecond {
		defaultDelay = 1000 * time.Millisecond // minimum 1s politeness
	}
	if hourlyQuota <= 0 {
		hourlyQuota = 3600 // max 1 request/sec average per host
	}
	return &PolitenessManager{
		hosts:        make(map[string]*HostState),
		DefaultDelay: defaultDelay,
		MaxBackoff:   60 * time.Second,
		HourlyQuota:  hourlyQuota,
	}
}

func (m *PolitenessManager) getOrCreateHost(hostKey string, customDelay time.Duration) *HostState {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.hosts[hostKey]
	if !ok {
		delay := m.DefaultDelay
		if customDelay > delay {
			delay = customDelay
		}
		state = &HostState{
			HostKey:      hostKey,
			CrawlDelay:   delay,
			MaxReqs:      1, // default 1 concurrent request per host
			HourlyQuota:  m.HourlyQuota,
			QuotaResetAt: time.Now().UTC().Add(time.Hour),
		}
		m.hosts[hostKey] = state
	} else if customDelay > state.CrawlDelay {
		state.CrawlDelay = customDelay
	}
	return state
}

// Acquire blocks until politeness delay elapses and concurrency slot is available for hostKey.
func (m *PolitenessManager) Acquire(ctx context.Context, hostKey string, customDelay time.Duration) error {
	state := m.getOrCreateHost(hostKey, customDelay)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		state.mu.Lock()
		now := time.Now().UTC()

		// 1. Quota check
		if now.After(state.QuotaResetAt) {
			state.HourlyCount = 0
			state.QuotaResetAt = now.Add(time.Hour)
		}
		if state.HourlyCount >= state.HourlyQuota {
			state.mu.Unlock()
			return ErrHostQuotaExceeded
		}

		// 2. Active backoff check
		if now.Before(state.BackoffUntil) {
			waitTime := state.BackoffUntil.Sub(now)
			state.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
				continue
			}
		}

		// 3. Concurrency check
		if state.ActiveReqs >= state.MaxReqs {
			state.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}

		// 4. Rate-limit delay check
		requiredWait := time.Duration(0)
		if !state.LastAccessAt.IsZero() {
			elapsed := now.Sub(state.LastAccessAt)
			if elapsed < state.CrawlDelay {
				requiredWait = state.CrawlDelay - elapsed
			}
		}

		if requiredWait > 0 {
			state.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(requiredWait):
				continue
			}
		}

		// Slot successfully acquired
		state.ActiveReqs++
		state.HourlyCount++
		state.LastAccessAt = time.Now().UTC()
		state.mu.Unlock()
		return nil
	}
}

// Release frees the active concurrency slot and adjusts backoff based on HTTP response.
func (m *PolitenessManager) Release(hostKey string, statusCode int, hasNetworkError bool) {
	m.mu.RLock()
	state, ok := m.hosts[hostKey]
	m.mu.RUnlock()
	if !ok {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.ActiveReqs > 0 {
		state.ActiveReqs--
	}

	now := time.Now().UTC()
	// Trigger exponential backoff on 429 Too Many Requests, 503 Unavailable, or network failures
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable || hasNetworkError {
		state.BackoffLevel++
		backoffSec := 1 << state.BackoffLevel // 2s, 4s, 8s, 16s...
		duration := time.Duration(backoffSec) * time.Second
		if duration > m.MaxBackoff {
			duration = m.MaxBackoff
		}
		state.BackoffUntil = now.Add(duration)
	} else if statusCode >= 200 && statusCode < 400 {
		// Successful request cools down backoff level
		if state.BackoffLevel > 0 {
			state.BackoffLevel--
		}
	}
}

// ConditionalHeaders attaches If-None-Match (ETag) and If-Modified-Since headers to an HTTP request.
func ConditionalHeaders(req *http.Request, etag string, lastModified string) {
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
}
