//go:build integration

// C-3 integration proof: durable outbox + tombstone persistence against a live
// PostgreSQL (the compose stack). Run explicitly, never in the default suite:
//
//	TEST_PG_URL='postgres://openportal:...@127.0.0.1:5435/openportal?sslmode=disable' \
//	  go test -count=1 -tags integration -run TestC3 ./internal/indexer/
package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func c3db(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_URL")
	if dsn == "" {
		t.Skip("TEST_PG_URL not set; run against the compose postgres")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	// Migration 016 must be applied (columns exist, UUID FK gone).
	var coltype string
	err = db.QueryRowContext(ctx,
		`SELECT format_type(a.atttypid, a.atttypmod) FROM pg_attribute a
		 JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname='catalog' AND c.relname='index_events' AND a.attname='document_id'`).Scan(&coltype)
	if err != nil || coltype != "text" {
		t.Fatalf("migration 016 not applied (document_id type=%q, err=%v)", coltype, err)
	}
	return db
}

func c3cleanup(t *testing.T, db *sql.DB) {
	t.Helper()
	_, _ = db.Exec(`DELETE FROM catalog.index_events WHERE document_id LIKE 'c3test-%'`)
	sum := sha256.Sum256([]byte("c3test-doomed"))
	_, _ = db.Exec(`DELETE FROM catalog.removal_tombstones WHERE canonical_hash=$1`, hex.EncodeToString(sum[:]))
}

func TestC3PGOutboxCycle(t *testing.T) {
	db := c3db(t)
	defer db.Close()
	defer c3cleanup(t, db)
	ctx := context.Background()
	outbox := NewPGOutbox(db, 30*time.Second, 5)

	ev1, err := outbox.Enqueue(ctx, "c3test-doc-1", OpUpsert, 1, map[string]interface{}{"title": "hello"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := outbox.Enqueue(ctx, "c3test-doc-2", OpDelete, 1, map[string]interface{}{"reason": "user_requested_deletion"}); err != nil {
		t.Fatalf("enqueue delete: %v", err)
	}
	leased, err := outbox.LeaseBatch(ctx, 10)
	if err != nil {
		t.Fatalf("lease: %v", err)
	}
	if len(leased) != 2 {
		t.Fatalf("expected 2 leased, got %d", len(leased))
	}
	// Second lease must not return the same rows (they are leased, not expired).
	again, err := outbox.LeaseBatch(ctx, 10)
	if err != nil {
		t.Fatalf("re-lease: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("leased rows must be invisible, got %d", len(again))
	}
	if err := outbox.Ack(ctx, ev1.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	var state string
	if err := db.QueryRowContext(ctx, `SELECT state FROM catalog.index_events WHERE id=$1`, ev1.ID).Scan(&state); err != nil || state != "done" {
		t.Fatalf("ack state=%q err=%v", state, err)
	}
	n, err := outbox.PendingCount(ctx)
	if err != nil || n < 1 {
		t.Fatalf("pending=%d err=%v", n, err)
	}
}

func TestC3TombstonePersist(t *testing.T) {
	db := c3db(t)
	defer db.Close()
	os := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"updated"}`))
	}))
	defer os.Close()
	mgr := NewTombstoneManager(os.URL, "idx", os.Client(), nil).WithDB(db)
	if _, err := mgr.ApplyTombstone(context.Background(), "c3test-doomed", TombstoneReasonPrivacyRight, "dpo"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	sum := sha256.Sum256([]byte("c3test-doomed"))
	var reason string
	var hold bool
	err := db.QueryRowContext(context.Background(),
		`SELECT reason, legal_hold FROM catalog.removal_tombstones WHERE canonical_hash=$1`, hex.EncodeToString(sum[:])).Scan(&reason, &hold)
	if err != nil {
		t.Fatalf("audit row missing: %v", err)
	}
	if reason != string(TombstoneReasonPrivacyRight) || !hold {
		t.Fatalf("reason=%q hold=%v", reason, hold)
	}
}

func TestC3DurableCycleEndToEnd(t *testing.T) {
	db := c3db(t)
	defer db.Close()
	defer c3cleanup(t, db)
	var bulks int
	os := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_bulk" {
			bulks++
		}
		_, _ = w.Write([]byte(`{"errors":false,"items":[]}`))
	}))
	defer os.Close()
	svc := NewIndexingService(os.URL, "idx", os.Client())
	relay := svc.EnableDurableOutbox(db)
	html := []byte(`<html><head><title>C3 Test Doc</title></head><body><p>Durable outbox end to end verification content body.</p></body></html>`)
	if _, _, err := svc.IngestCrawlResult(context.Background(), "c3test-e2e", "https://example.test/c3", html, time.Now().UTC()); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// Nothing dispatched yet: intent sits in PostgreSQL.
	if bulks != 0 {
		t.Fatalf("durable ingest must not dispatch directly, bulks=%d", bulks)
	}
	upserts, _, err := relay.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if upserts != 1 || bulks != 1 {
		t.Fatalf("upserts=%d bulks=%d", upserts, bulks)
	}
	var state string
	if err := db.QueryRowContext(context.Background(),
		`SELECT state FROM catalog.index_events WHERE document_id='c3test-e2e'`).Scan(&state); err != nil || state != "done" {
		t.Fatalf("event state=%q err=%v", state, err)
	}
}
