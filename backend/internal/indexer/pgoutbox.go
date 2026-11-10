package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// PGOutbox is a PostgreSQL-backed implementation of the index-event outbox on
// catalog.index_events (C-3). Unlike the in-memory OutboxQueue it survives
// restarts and coordinates multiple relay workers via SELECT ... FOR UPDATE
// SKIP LOCKED.
type PGOutbox struct {
	db          *sql.DB
	lease       time.Duration
	maxAttempts int
}

// NewPGOutbox creates a durable outbox. lease bounds how long a worker may hold
// events before they become eligible for re-lease; maxAttempts bounds retries
// before an event is marked dead.
func NewPGOutbox(db *sql.DB, lease time.Duration, maxAttempts int) *PGOutbox {
	if lease <= 0 {
		lease = 30 * time.Second
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &PGOutbox{db: db, lease: lease, maxAttempts: maxAttempts}
}

// Enqueue inserts a new index event.
func (q *PGOutbox) Enqueue(ctx context.Context, docID string, op IndexOperation, version int, payload map[string]interface{}) (*IndexEvent, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var ev IndexEvent
	err = q.db.QueryRowContext(ctx,
		`INSERT INTO catalog.index_events (document_id, operation, payload_version, payload)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, document_id, operation, state, payload_version, attempts, created_at`,
		docID, string(op), version, string(raw),
	).Scan(&ev.ID, &ev.DocumentID, &ev.Operation, &ev.State, &ev.PayloadVersion, &ev.Attempts, &ev.CreatedAt)
	if err != nil {
		return nil, err
	}
	ev.MaxAttempts = q.maxAttempts
	ev.Payload = payload
	return &ev, nil
}

// LeaseBatch acquires up to batchSize pending (or lease-expired) events.
func (q *PGOutbox) LeaseBatch(ctx context.Context, batchSize int) ([]*IndexEvent, error) {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // read-only on error paths; commit below on success
	rows, err := tx.QueryContext(ctx,
		`SELECT id, document_id, operation, payload_version, attempts, payload
		 FROM catalog.index_events
		 WHERE state IN ('pending','leased')
		   AND attempts < $1
		   AND (state = 'pending' OR leased_at IS NULL OR leased_at < now() - ($2 * interval '1 second'))
		 ORDER BY id
		 LIMIT $3
		 FOR UPDATE SKIP LOCKED`,
		q.maxAttempts, int(q.lease.Seconds()), batchSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []*IndexEvent
	var ids []int64
	for rows.Next() {
		var ev IndexEvent
		var raw string
		if err := rows.Scan(&ev.ID, &ev.DocumentID, &ev.Operation, &ev.PayloadVersion, &ev.Attempts, &raw); err != nil {
			return nil, err
		}
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &ev.Payload)
		}
		ev.MaxAttempts = q.maxAttempts
		events = append(events, &ev)
		ids = append(ids, ev.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE catalog.index_events SET state='leased', leased_at=now(), attempts=attempts+1 WHERE id=$1`, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return events, nil
}

// Ack marks an event done.
func (q *PGOutbox) Ack(ctx context.Context, eventID int64) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE catalog.index_events SET state='done', processed_at=now() WHERE id=$1`, eventID)
	return err
}

// Nack releases an event back to pending, or dead once attempts are exhausted.
func (q *PGOutbox) Nack(ctx context.Context, eventID int64) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE catalog.index_events
		 SET state = CASE WHEN attempts >= $2 THEN 'dead' ELSE 'pending' END
		 WHERE id=$1`, eventID, q.maxAttempts)
	return err
}

// PendingCount returns the number of pending/leased events.
func (q *PGOutbox) PendingCount(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx,
		`SELECT count(*) FROM catalog.index_events WHERE state IN ('pending','leased')`).Scan(&n)
	return n, err
}
