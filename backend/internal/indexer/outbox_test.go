package indexer

import (
	"context"
	"testing"
	"time"
)

func TestOutboxQueueAndIdempotentConsumer(t *testing.T) {
	ctx := context.Background()
	q := NewOutboxQueue(5*time.Second, 3)

	// 1. Enqueue 2 events
	ev1, err := q.Enqueue("doc-1", OpUpsert, 1, map[string]interface{}{"title": "v1"})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	_, err = q.Enqueue("doc-2", OpUpsert, 1, map[string]interface{}{"title": "doc2"})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if q.PendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", q.PendingCount())
	}

	// 2. Lease batch of 2
	leased, err := q.LeaseBatch(ctx, 2)
	if err != nil {
		t.Fatalf("LeaseBatch failed: %v", err)
	}
	if len(leased) != 2 {
		t.Fatalf("expected 2 leased, got %d", len(leased))
	}

	// 3. Confirm processing ev1 (v1)
	if err := q.Ack(ev1.ID); err != nil {
		t.Errorf("Ack failed: %v", err)
	}

	// 4. Test idempotency: enqueuing and processing an older/superseded version (v1) when v2 has already occurred
	// Simulate v2 processed first:
	evV2, _ := q.Enqueue("doc-1", OpUpsert, 2, map[string]interface{}{"title": "v2"})
	leasedV2, _ := q.LeaseBatch(ctx, 1)
	if len(leasedV2) != 1 || leasedV2[0].ID != evV2.ID {
		t.Fatalf("expected evV2 leased")
	}
	if err := q.Ack(evV2.ID); err != nil {
		t.Fatalf("Ack v2 failed: %v", err)
	}

	// Now try to process an older event (v1) for doc-1:
	evOld, _ := q.Enqueue("doc-1", OpUpsert, 1, map[string]interface{}{"title": "stale v1"})
	_ = q.Ack(evOld.ID)
	// Must return ErrEventAlreadyProcessed or mark as no-op done
	if evOld.State != StateDone {
		t.Errorf("expected obsolete event to be marked done without overwriting newer state")
	}
}

func TestTombstoneManager(t *testing.T) {
	ctx := context.Background()
	q := NewOutboxQueue(10*time.Second, 3)
	mgr := NewTombstoneManager("", "", nil, q)

	rec, err := mgr.ApplyTombstone(ctx, "doc-to-delete", TombstoneReasonLegalTakedown, "legal-team")
	if err != nil {
		t.Fatalf("ApplyTombstone failed: %v", err)
	}

	if rec.Reason != TombstoneReasonLegalTakedown {
		t.Errorf("unexpected reason: %s", rec.Reason)
	}

	if q.PendingCount() != 1 {
		t.Fatalf("expected 1 pending outbox delete event, got %d", q.PendingCount())
	}

	leased, _ := q.LeaseBatch(ctx, 1)
	if len(leased) != 1 {
		t.Fatalf("expected 1 leased delete event")
	}
	if leased[0].Operation != OpDelete {
		t.Errorf("expected OpDelete, got %s", leased[0].Operation)
	}
}

