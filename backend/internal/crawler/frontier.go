package crawler

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrFrontierURLAlreadyExists = errors.New("frontier: url already exists")
	ErrFrontierItemNotFound     = errors.New("frontier: item not found")
	ErrFrontierItemNotLeased    = errors.New("frontier: item is not currently leased by this worker")
	ErrHostNotRegistered       = errors.New("frontier: host is not registered or verified")
	ErrHostCrawlDisabled       = errors.New("frontier: host crawling is disabled")
)

// FrontierState defines the lifecycle state of a URL in the frontier.
type FrontierState string

const (
	StateQueued  FrontierState = "queued"
	StateLeased  FrontierState = "leased"
	StateDone    FrontierState = "done"
	StateBlocked FrontierState = "blocked"
	StateFailed  FrontierState = "failed"
)

// VerifiedHost represents a verified site eligible for crawling.
type VerifiedHost struct {
	ID           string        `json:"id"`
	Scheme       string        `json:"scheme"`
	Hostname     string        `json:"hostname"`
	Port         int           `json:"port"`
	HostKey      string        `json:"host_key"`
	FetchEnabled bool          `json:"fetch_enabled"`
	CrawlDelay   time.Duration `json:"crawl_delay"`
	CreatedAt    time.Time     `json:"created_at"`
}

// FrontierItem represents a single URL item in the crawl frontier queue.
type FrontierItem struct {
	ID           string        `json:"id"`
	HostID       string        `json:"host_id"`
	HostKey      string        `json:"host_key"`
	URL          string        `json:"url"`
	URLHash      string        `json:"url_hash"`
	Priority     int           `json:"priority"` // Higher value = higher priority (0-100)
	State        FrontierState `json:"state"`
	ScheduledAt  time.Time     `json:"scheduled_at"`
	LeaseOwner   string        `json:"lease_owner,omitempty"`
	LeaseUntil   time.Time     `json:"lease_until,omitempty"`
	RetryCount   int           `json:"retry_count"`
	MaxRetries   int           `json:"max_retries"`
	DiscoveredAt time.Time     `json:"discovered_at"`
	LastError    string        `json:"last_error,omitempty"`
}

// Frontier manages partitioned URL queues, priority scheduling, and atomic leases.
type Frontier struct {
	mu          sync.RWMutex
	hosts       map[string]*VerifiedHost
	itemsByHash map[string]*FrontierItem
	queues      map[string][]*FrontierItem // hostKey -> sorted slice of queued items
}

func NewFrontier() *Frontier {
	return &Frontier{
		hosts:       make(map[string]*VerifiedHost),
		itemsByHash: make(map[string]*FrontierItem),
		queues:      make(map[string][]*FrontierItem),
	}
}

// RegisterVerifiedHost provisions an authorized domain for bounded crawling (Task 021).
func (f *Frontier) RegisterVerifiedHost(host *VerifiedHost) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hosts[host.HostKey] = host
}

// IsHostAllowed checks whether the host is registered and fetch-enabled.
func (f *Frontier) IsHostAllowed(hostKey string) (*VerifiedHost, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	h, ok := f.hosts[hostKey]
	if !ok || !h.FetchEnabled {
		return nil, false
	}
	return h, true
}

// SubmitURL normalizes and enqueues a URL with domain partitioning and priority (Task 021 / 023).
func (f *Frontier) SubmitURL(rawURL string, priority int) (*FrontierItem, error) {
	cu, err := Canonicalize(rawURL)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if host is registered and verified
	host, ok := f.hosts[cu.HostKey]
	if !ok {
		return nil, ErrHostNotRegistered
	}
	if !host.FetchEnabled {
		return nil, ErrHostCrawlDisabled
	}

	// Check for duplicates
	if existing, ok := f.itemsByHash[cu.URLHash]; ok {
		// If already done or queued, return existing item
		return existing, ErrFrontierURLAlreadyExists
	}

	if priority < 0 {
		priority = 0
	} else if priority > 100 {
		priority = 100
	}

	now := time.Now().UTC()
	item := &FrontierItem{
		ID:           cu.URLHash[:32],
		HostID:       host.ID,
		HostKey:      cu.HostKey,
		URL:          cu.Canonical,
		URLHash:      cu.URLHash,
		Priority:     priority,
		State:        StateQueued,
		ScheduledAt:  now,
		RetryCount:   0,
		MaxRetries:   3,
		DiscoveredAt: now,
	}

	f.itemsByHash[cu.URLHash] = item
	f.queues[cu.HostKey] = append(f.queues[cu.HostKey], item)

	return item, nil
}

// LeaseAcquire acquires a batch of ready items for workerID across domain partitions.
func (f *Frontier) LeaseAcquire(workerID string, maxBatch int, leaseDuration time.Duration) []*FrontierItem {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now().UTC()
	var leased []*FrontierItem

	// Round-robin or partition across hosts
	for hostKey, queue := range f.queues {
		if len(leased) >= maxBatch {
			break
		}

		// Re-sort host queue by Priority DESC, ScheduledAt ASC
		sort.SliceStable(queue, func(i, j int) bool {
			if queue[i].Priority != queue[j].Priority {
				return queue[i].Priority > queue[j].Priority
			}
			return queue[i].ScheduledAt.Before(queue[j].ScheduledAt)
		})

		var remaining []*FrontierItem
		for _, item := range queue {
			if len(leased) < maxBatch && item.State == StateQueued && !now.Before(item.ScheduledAt) {
				item.State = StateLeased
				item.LeaseOwner = workerID
				item.LeaseUntil = now.Add(leaseDuration)
				leased = append(leased, item)
			} else {
				remaining = append(remaining, item)
			}
		}
		f.queues[hostKey] = remaining
	}

	return leased
}

// Ack transitions an item to StateDone upon successful crawling.
func (f *Frontier) Ack(urlHash string, workerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.itemsByHash[urlHash]
	if !ok {
		return ErrFrontierItemNotFound
	}
	if item.State != StateLeased || item.LeaseOwner != workerID {
		return ErrFrontierItemNotLeased
	}

	item.State = StateDone
	item.LeaseOwner = ""
	item.LeaseUntil = time.Time{}
	item.LastError = ""
	return nil
}

// Nack handles failure by scheduling backoff retry or transitioning to StateFailed if max retries reached.
func (f *Frontier) Nack(urlHash string, workerID string, errReason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.itemsByHash[urlHash]
	if !ok {
		return ErrFrontierItemNotFound
	}
	if item.State != StateLeased || item.LeaseOwner != workerID {
		return ErrFrontierItemNotLeased
	}

	item.RetryCount++
	item.LastError = errReason
	item.LeaseOwner = ""
	item.LeaseUntil = time.Time{}

	now := time.Now().UTC()
	if item.RetryCount > item.MaxRetries {
		item.State = StateFailed
		return nil
	}

	// Exponential backoff: 2s, 4s, 8s...
	backoffSec := 1 << item.RetryCount
	item.ScheduledAt = now.Add(time.Duration(backoffSec) * time.Second)
	item.State = StateQueued

	// Put back in host queue
	f.queues[item.HostKey] = append(f.queues[item.HostKey], item)
	return nil
}

// Block marks an item as StateBlocked (e.g. disallowed by robots.txt or policy rule).
func (f *Frontier) Block(urlHash string, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.itemsByHash[urlHash]
	if !ok {
		return ErrFrontierItemNotFound
	}

	item.State = StateBlocked
	item.LastError = reason
	item.LeaseOwner = ""
	item.LeaseUntil = time.Time{}
	return nil
}

// QueueDepth returns the count of items in a specific state.
func (f *Frontier) QueueDepth(state FrontierState) int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	count := 0
	for _, item := range f.itemsByHash {
		if item.State == state {
			count++
		}
	}
	return count
}

