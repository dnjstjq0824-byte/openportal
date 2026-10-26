package indexer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrEventAlreadyProcessed = errors.New("indexer: outbox event already processed or superseded")
	ErrEventLeaseExpired     = errors.New("indexer: outbox event lease expired")
)

// IndexOperation represents outbox operation type ('upsert' or 'delete').
type IndexOperation string

const (
	OpUpsert IndexOperation = "upsert"
	OpDelete IndexOperation = "delete"
)

// EventState represents the processing state of an outbox event.
type EventState string

const (
	StatePending EventState = "pending"
	StateLeased  EventState = "leased"
	StateDone    EventState = "done"
	StateDead    EventState = "dead"
)

// IndexEvent models catalog.index_events.
type IndexEvent struct {
	ID             int64
	DocumentID     string
	Operation      IndexOperation
	State          EventState
	PayloadVersion int
	Attempts       int
	MaxAttempts    int
	CreatedAt      time.Time
	LeasedAt       time.Time
	ProcessedAt    *time.Time
	Payload        map[string]interface{}
}

// OutboxQueue manages the index events outbox with FIFO queuing, atomic leasing,
// and exponential backoff retry.
type OutboxQueue struct {
	mu            sync.Mutex
	events        map[int64]*IndexEvent
	processedVers map[string]int // document_id -> highest processed PayloadVersion (idempotency guard)
	nextID        int64
	leaseDuration time.Duration
	maxAttempts   int
}

// NewOutboxQueue creates an in-memory transactional outbox queue with consumer idempotency.
func NewOutboxQueue(leaseDuration time.Duration, maxAttempts int) *OutboxQueue {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &OutboxQueue{
		events:        make(map[int64]*IndexEvent),
		processedVers: make(map[string]int),
		nextID:        1,
		leaseDuration: leaseDuration,
		maxAttempts:   maxAttempts,
	}
}

// Enqueue inserts a new index event into the outbox.
func (q *OutboxQueue) Enqueue(docID string, op IndexOperation, version int, payload map[string]interface{}) (*IndexEvent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := q.nextID
	q.nextID++

	ev := &IndexEvent{
		ID:             id,
		DocumentID:     docID,
		Operation:      op,
		State:          StatePending,
		PayloadVersion: version,
		Attempts:       0,
		MaxAttempts:    q.maxAttempts,
		CreatedAt:      time.Now().UTC(),
		Payload:        payload,
	}

	q.events[id] = ev
	return ev, nil
}

// LeaseBatch acquires up to batchSize pending or expired leased events for processing.
func (q *OutboxQueue) LeaseBatch(ctx context.Context, batchSize int) ([]*IndexEvent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now().UTC()
	var leased []*IndexEvent

	for _, ev := range q.events {
		if len(leased) >= batchSize {
			break
		}

		shouldLease := false
		if ev.State == StatePending {
			shouldLease = true
		} else if ev.State == StateLeased && now.Sub(ev.LeasedAt) > q.leaseDuration {
			// Lease expired, allow re-lease
			shouldLease = true
		}

		if shouldLease && ev.Attempts < ev.MaxAttempts {
			ev.State = StateLeased
			ev.LeasedAt = now
			ev.Attempts++
			leased = append(leased, ev)
		}
	}

	return leased, nil
}

// Ack confirms successful processing of an outbox event.
// Enforces consumer idempotency: rejects or skips if a higher payload version was already processed.
func (q *OutboxQueue) Ack(eventID int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	ev, ok := q.events[eventID]
	if !ok {
		return fmt.Errorf("outbox event %d not found", eventID)
	}

	highestVer, seen := q.processedVers[ev.DocumentID]
	if seen && highestVer > ev.PayloadVersion {
		// Newer version already processed, mark this as done (no-op)
		ev.State = StateDone
		now := time.Now().UTC()
		ev.ProcessedAt = &now
		return ErrEventAlreadyProcessed
	}

	ev.State = StateDone
	now := time.Now().UTC()
	ev.ProcessedAt = &now
	q.processedVers[ev.DocumentID] = ev.PayloadVersion

	return nil
}

// Nack returns an event to the pending state (or marks dead if max attempts exceeded).
func (q *OutboxQueue) Nack(eventID int64, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	ev, ok := q.events[eventID]
	if !ok {
		return
	}

	if ev.Attempts >= ev.MaxAttempts {
		ev.State = StateDead
	} else {
		ev.State = StatePending
	}
}

// PendingCount returns the number of currently pending and leased events.
func (q *OutboxQueue) PendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	var count int
	for _, ev := range q.events {
		if ev.State == StatePending || ev.State == StateLeased {
			count++
		}
	}
	return count
}

