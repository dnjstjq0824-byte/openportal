package indexer

import (
	"context"
	"fmt"
)

// RelayStore abstracts the memory and PostgreSQL outboxes for the relay worker.
type RelayStore interface {
	LeaseBatch(ctx context.Context, batchSize int) ([]*IndexEvent, error)
	Ack(ctx context.Context, eventID int64) error
	Nack(ctx context.Context, eventID int64) error
}

// memoryRelayStore adapts the in-memory OutboxQueue to RelayStore.
type memoryRelayStore struct {
	queue *OutboxQueue
}

// MemoryRelayStore wraps an in-memory queue for relay workers and tests.
func MemoryRelayStore(queue *OutboxQueue) RelayStore {
	return &memoryRelayStore{queue: queue}
}

func (s *memoryRelayStore) LeaseBatch(ctx context.Context, batchSize int) ([]*IndexEvent, error) {
	return s.queue.LeaseBatch(ctx, batchSize)
}

func (s *memoryRelayStore) Ack(_ context.Context, eventID int64) error {
	return s.queue.Ack(eventID)
}

func (s *memoryRelayStore) Nack(_ context.Context, eventID int64) error {
	s.queue.Nack(eventID, nil)
	return nil
}

// pgRelayStore adapts PGOutbox (whose Nack returns an error already).
type pgRelayStore struct {
	queue *PGOutbox
}

// PGRelayStore wraps a durable outbox for relay workers.
func PGRelayStore(queue *PGOutbox) RelayStore {
	return &pgRelayStore{queue: queue}
}

func (s *pgRelayStore) LeaseBatch(ctx context.Context, batchSize int) ([]*IndexEvent, error) {
	return s.queue.LeaseBatch(ctx, batchSize)
}

func (s *pgRelayStore) Ack(ctx context.Context, eventID int64) error {
	return s.queue.Ack(ctx, eventID)
}

func (s *pgRelayStore) Nack(ctx context.Context, eventID int64) error {
	return s.queue.Nack(ctx, eventID)
}

// Relay drains outbox events into OpenSearch: upserts go through one _bulk
// request per batch, deletes go through tombstone propagation (C-3).
type Relay struct {
	Store     RelayStore
	BatchSize int
	// Bulk indexes upsert payloads; Delete applies a tombstone for a delete event.
	Bulk   func(ctx context.Context, docs []BulkDoc) error
	Delete func(ctx context.Context, docID string, payload map[string]interface{}) error
}

func (r *Relay) batchSize() int {
	if r.BatchSize <= 0 {
		return 50
	}
	return r.BatchSize
}

// RunOnce leases one batch and delivers it. It returns the delivered upsert
// and delete counts; a delivery failure nacks the affected events and is
// returned so the worker loop can back off.
func (r *Relay) RunOnce(ctx context.Context) (upserts int, deletes int, err error) {
	events, err := r.Store.LeaseBatch(ctx, r.batchSize())
	if err != nil {
		return 0, 0, err
	}
	var pending []*IndexEvent
	var docs []BulkDoc
	for _, ev := range events {
		if ev.Operation == OpDelete {
			continue
		}
		pending = append(pending, ev)
		docs = append(docs, BulkDoc{ID: ev.DocumentID, Payload: ev.Payload})
	}
	if len(docs) > 0 {
		if err := r.Bulk(ctx, docs); err != nil {
			for _, ev := range pending {
				_ = r.Store.Nack(ctx, ev.ID)
			}
			return 0, 0, fmt.Errorf("relay bulk upsert failed: %w", err)
		}
		for _, ev := range pending {
			_ = r.Store.Ack(ctx, ev.ID)
		}
		upserts = len(pending)
	}
	for _, ev := range events {
		if ev.Operation != OpDelete {
			continue
		}
		if err := r.Delete(ctx, ev.DocumentID, ev.Payload); err != nil {
			_ = r.Store.Nack(ctx, ev.ID)
			return upserts, deletes, fmt.Errorf("relay delete failed for %s: %w", ev.DocumentID, err)
		}
		_ = r.Store.Ack(ctx, ev.ID)
		deletes++
	}
	return upserts, deletes, nil
}
