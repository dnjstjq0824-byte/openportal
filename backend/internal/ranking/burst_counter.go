package ranking

import (
	"sync"
	"time"
)

// BurstCounter records rate events per key and reports budget verdicts. Memory
// and Redis implementations share sliding-window semantics: the current event
// is recorded first, then the key is over budget when more than max events
// fall inside window (C-6).
type BurstCounter interface {
	// Allow records an event for key and returns false when the key exceeds
	// max events inside window (including this one).
	Allow(key string, window time.Duration, max int) bool
}

// MemoryBurstCounter is the process-local BurstCounter.
type MemoryBurstCounter struct {
	mu      sync.Mutex
	history map[string][]time.Time
}

// NewMemoryBurstCounter creates a process-local counter.
func NewMemoryBurstCounter() *MemoryBurstCounter {
	return &MemoryBurstCounter{history: map[string][]time.Time{}}
}

// Allow implements BurstCounter.
func (c *MemoryBurstCounter) Allow(key string, window time.Duration, max int) bool {
	now := time.Now()
	cutoff := now.Add(-window)
	c.mu.Lock()
	defer c.mu.Unlock()
	valid := make([]time.Time, 0, len(c.history[key])+1)
	for _, t := range c.history[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	c.history[key] = valid
	return len(valid) <= max
}
