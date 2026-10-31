package crawler

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// PostgresFrontier persists crawler state, leases, and host policies in PostgreSQL.
type PostgresFrontier struct {
	mu          sync.RWMutex
	db          *sql.DB
	cachedHosts map[string]*VerifiedHost
}

// NewPostgresFrontier constructs a database-backed frontier.
func NewPostgresFrontier(db *sql.DB) *PostgresFrontier {
	return &PostgresFrontier{
		db:          db,
		cachedHosts: make(map[string]*VerifiedHost),
	}
}

// RegisterVerifiedHost provisions an authorized domain in the PostgreSQL crawl.hosts table.
func (pf *PostgresFrontier) RegisterVerifiedHost(host *VerifiedHost) {
	pf.mu.Lock()
	pf.cachedHosts[host.HostKey] = host
	pf.mu.Unlock()

	if pf.db == nil {
		return
	}

	query := `
		INSERT INTO crawl.hosts (scheme, hostname, port, fetch_enabled, crawl_delay_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (scheme, hostname, port) DO UPDATE
		SET fetch_enabled = EXCLUDED.fetch_enabled,
		    crawl_delay_ms = EXCLUDED.crawl_delay_ms
		RETURNING id;
	`
	delayMs := int(host.CrawlDelay / time.Millisecond)
	if delayMs < 0 {
		delayMs = 1000
	}
	createdAt := host.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	var hostID string
	err := pf.db.QueryRow(query, host.Scheme, host.Hostname, host.Port, host.FetchEnabled, delayMs, createdAt).Scan(&hostID)
	if err == nil && hostID != "" {
		pf.mu.Lock()
		host.ID = hostID
		pf.cachedHosts[host.HostKey] = host
		pf.mu.Unlock()
	}
}

// IsHostAllowed checks whether a host is provisioned and enabled in PostgreSQL.
func (pf *PostgresFrontier) IsHostAllowed(hostKey string) (*VerifiedHost, bool) {
	pf.mu.RLock()
	h, ok := pf.cachedHosts[hostKey]
	pf.mu.RUnlock()
	if ok && h != nil {
		return h, h.FetchEnabled
	}

	if pf.db == nil {
		return nil, false
	}

	// Format: scheme://hostname:port
	parts := strings.Split(hostKey, "://")
	if len(parts) != 2 {
		return nil, false
	}
	scheme := parts[0]
	subparts := strings.Split(parts[1], ":")
	hostname := subparts[0]
	port := 443
	if len(subparts) > 1 {
		fmt.Sscanf(subparts[1], "%d", &port)
	}

	query := `
		SELECT id, scheme, hostname, port, fetch_enabled, crawl_delay_ms, created_at
		FROM crawl.hosts
		WHERE scheme = $1 AND hostname = $2 AND port = $3;
	`
	var host VerifiedHost
	var delayMs int
	err := pf.db.QueryRow(query, scheme, hostname, port).Scan(
		&host.ID, &host.Scheme, &host.Hostname, &host.Port, &host.FetchEnabled, &delayMs, &host.CreatedAt,
	)
	if err != nil {
		return nil, false
	}

	host.HostKey = hostKey
	host.CrawlDelay = time.Duration(delayMs) * time.Millisecond

	pf.mu.Lock()
	pf.cachedHosts[hostKey] = &host
	pf.mu.Unlock()

	return &host, host.FetchEnabled
}

// SubmitURL enqueues a URL to the crawl.frontier PostgreSQL table.
func (pf *PostgresFrontier) SubmitURL(rawURL string, priority int) (*FrontierItem, error) {
	cu, err := Canonicalize(rawURL)
	if err != nil {
		return nil, err
	}

	host, allowed := pf.IsHostAllowed(cu.HostKey)
	if !allowed || host == nil {
		return nil, ErrHostNotRegistered
	}
	if !host.FetchEnabled {
		return nil, ErrHostCrawlDisabled
	}

	if priority < 0 {
		priority = 0
	} else if priority > 100 {
		priority = 100
	}

	now := time.Now().UTC()
	item := &FrontierItem{
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

	if pf.db == nil {
		item.ID = cu.URLHash[:32]
		return item, nil
	}

	query := `
		INSERT INTO crawl.frontier (host_id, url, url_hash, priority, state, scheduled_at, retry_count, discovered_at)
		VALUES ($1, $2, $3, $4, 'queued', $5, 0, $6)
		ON CONFLICT (url) DO UPDATE
		SET priority = GREATEST(crawl.frontier.priority, EXCLUDED.priority)
		RETURNING id, host_id, url, url_hash, priority, state, scheduled_at, retry_count, discovered_at;
	`
	err = pf.db.QueryRow(query, host.ID, cu.Canonical, cu.URLHash, priority, now, now).Scan(
		&item.ID, &item.HostID, &item.URL, &item.URLHash, &item.Priority, &item.State, &item.ScheduledAt, &item.RetryCount, &item.DiscoveredAt,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres frontier submit error: %w", err)
	}

	return item, nil
}

// LeaseAcquire atomically leases a batch of ready URLs from PostgreSQL using FOR UPDATE SKIP LOCKED.
func (pf *PostgresFrontier) LeaseAcquire(workerID string, maxBatch int, leaseDuration time.Duration) []*FrontierItem {
	if pf.db == nil || maxBatch <= 0 {
		return nil
	}

	durationSec := int(leaseDuration.Seconds())
	if durationSec < 1 {
		durationSec = 30
	}

	query := fmt.Sprintf(`
		UPDATE crawl.frontier
		SET state = 'leased',
		    lease_owner = $1,
		    lease_until = now() + interval '%d seconds'
		WHERE id IN (
			SELECT f.id
			FROM crawl.frontier f
			JOIN crawl.hosts h ON f.host_id = h.id
			WHERE f.state = 'queued'
			  AND f.scheduled_at <= now()
			  AND h.fetch_enabled = true
			ORDER BY f.priority DESC, f.scheduled_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, host_id, url, url_hash, priority, state, scheduled_at, retry_count, discovered_at;
	`, durationSec)

	rows, err := pf.db.Query(query, workerID, maxBatch)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var leased []*FrontierItem
	for rows.Next() {
		var item FrontierItem
		err := rows.Scan(
			&item.ID, &item.HostID, &item.URL, &item.URLHash, &item.Priority, &item.State, &item.ScheduledAt, &item.RetryCount, &item.DiscoveredAt,
		)
		if err == nil {
			item.LeaseOwner = workerID
			item.LeaseUntil = time.Now().UTC().Add(leaseDuration)
			item.MaxRetries = 3
			cu, _ := Canonicalize(item.URL)
			if cu != nil {
				item.HostKey = cu.HostKey
			}
			leased = append(leased, &item)
		}
	}

	return leased
}

// Ack marks a URL as done upon successful crawling and clears the lease.
func (pf *PostgresFrontier) Ack(urlHash string, workerID string) error {
	if pf.db == nil {
		return nil
	}
	query := `
		UPDATE crawl.frontier
		SET state = 'done',
		    lease_owner = NULL,
		    lease_until = NULL,
		    last_error = NULL
		WHERE url_hash = $1 AND state = 'leased' AND lease_owner = $2;
	`
	res, err := pf.db.Exec(query, urlHash, workerID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrFrontierItemNotFound
	}
	return nil
}

// Nack handles failure by scheduling backoff retry or transitioning to StateFailed.
func (pf *PostgresFrontier) Nack(urlHash string, workerID string, errReason string) error {
	if pf.db == nil {
		return nil
	}
	query := `
		UPDATE crawl.frontier
		SET retry_count = retry_count + 1,
		    state = CASE WHEN retry_count + 1 >= 3 THEN 'failed' ELSE 'queued' END,
		    scheduled_at = now() + (interval '1 second' * power(2, LEAST(retry_count + 1, 6))),
		    lease_owner = NULL,
		    lease_until = NULL,
		    last_error = $1
		WHERE url_hash = $2 AND state = 'leased' AND lease_owner = $3;
	`
	_, err := pf.db.Exec(query, errReason, urlHash, workerID)
	return err
}

// Block marks an item as StateBlocked (e.g. disallowed by robots.txt or policy rule).
func (pf *PostgresFrontier) Block(urlHash string, reason string) error {
	if pf.db == nil {
		return nil
	}
	query := `
		UPDATE crawl.frontier
		SET state = 'blocked',
		    last_error = $1,
		    lease_owner = NULL,
		    lease_until = NULL
		WHERE url_hash = $2;
	`
	_, err := pf.db.Exec(query, reason, urlHash)
	return err
}

// QueueDepth returns the count of items in a specific state from PostgreSQL.
func (pf *PostgresFrontier) QueueDepth(state FrontierState) int {
	if pf.db == nil {
		return 0
	}
	var count int
	err := pf.db.QueryRow("SELECT count(*) FROM crawl.frontier WHERE state = $1;", string(state)).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// ReclaimExpiredLeases returns expired leased URLs back to StateQueued.
func (pf *PostgresFrontier) ReclaimExpiredLeases() int {
	if pf.db == nil {
		return 0
	}
	query := `
		UPDATE crawl.frontier
		SET state = 'queued',
		    lease_owner = NULL,
		    lease_until = NULL
		WHERE state = 'leased' AND lease_until < now();
	`
	res, err := pf.db.Exec(query)
	if err != nil {
		return 0
	}
	affected, _ := res.RowsAffected()
	return int(affected)
}
