package indexer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// C-3: BulkIndex sends one _bulk request without per-document refresh.
func TestBulkIndexSingleRequest(t *testing.T) {
	var gotPath, gotCT string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"errors":false,"items":[{"index":{"_id":"d1","status":201}},{"index":{"_id":"d2","status":201}}]}`))
	}))
	defer server.Close()
	svc := NewIndexingService(server.URL, "documents_v2", server.Client())
	err := svc.BulkIndex(context.Background(), []BulkDoc{
		{ID: "d1", Payload: map[string]interface{}{"title": "one"}},
		{ID: "d2", Payload: map[string]interface{}{"title": "two"}},
	})
	if err != nil {
		t.Fatalf("BulkIndex: %v", err)
	}
	if gotPath != "/_bulk" {
		t.Fatalf("expected single _bulk call, got path %q", gotPath)
	}
	if gotCT != "application/x-ndjson" {
		t.Fatalf("content type: %q", gotCT)
	}
	lines := strings.Split(strings.TrimSuffix(string(gotBody), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 ndjson lines for 2 docs, got %d", len(lines))
	}
	var meta map[string]map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
		t.Fatalf("meta line: %v", err)
	}
	if meta["index"]["_id"] != "d1" || meta["index"]["_index"] != "documents_v2" {
		t.Fatalf("meta line: %v", meta)
	}
}

func TestBulkIndexPartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":true,"items":[{"index":{"_id":"d1","status":201}},{"index":{"_id":"d2","status":400,"error":{"type":"mapper_exception","reason":"bad"}}}]}`))
	}))
	defer server.Close()
	svc := NewIndexingService(server.URL, "documents_v2", server.Client())
	err := svc.BulkIndex(context.Background(), []BulkDoc{
		{ID: "d1", Payload: map[string]interface{}{"title": "one"}},
		{ID: "d2", Payload: map[string]interface{}{"title": "two"}},
	})
	if err == nil || !strings.Contains(err.Error(), "d2") {
		t.Fatalf("expected partial failure naming d2, got %v", err)
	}
}

// C-3: the relay bulk-delivers upserts and tombstones deletes, acking each.
func TestRelayRunOnce(t *testing.T) {
	var bulks int
	os := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_bulk" {
			bulks++
			_, _ = w.Write([]byte(`{"errors":false,"items":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"updated"}`))
	}))
	defer os.Close()
	svc := NewIndexingService(os.URL, "idx", os.Client())
	queue := NewOutboxQueue(15*time.Second, 5)
	if _, err := queue.Enqueue("doc-a", OpUpsert, 1, map[string]interface{}{"title": "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Enqueue("doc-b", OpUpsert, 2, map[string]interface{}{"title": "b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Enqueue("doc-gone", OpDelete, 1, map[string]interface{}{"reason": "user_requested_deletion", "requester": "tester"}); err != nil {
		t.Fatal(err)
	}
	relay := &Relay{
		Store: MemoryRelayStore(queue),
		Bulk:  svc.BulkIndex,
		Delete: func(ctx context.Context, docID string, payload map[string]interface{}) error {
			_, err := svc.tombstones.ApplyTombstone(ctx, docID, TombstoneReasonUserDeleted, "tester")
			return err
		},
	}
	upserts, deletes, err := relay.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if upserts != 2 || deletes != 1 {
		t.Fatalf("got upserts=%d deletes=%d", upserts, deletes)
	}
	if bulks != 1 {
		t.Fatalf("expected one _bulk call for both upserts, got %d", bulks)
	}
	if queue.PendingCount() != 0 {
		t.Fatalf("expected all events acked, pending=%d", queue.PendingCount())
	}
}

// C-3 red drill: a failed bulk nacks every upsert for retry instead of dropping them.
func TestRelayBulkFailureNacks(t *testing.T) {
	queue := NewOutboxQueue(15*time.Second, 5)
	if _, err := queue.Enqueue("doc-a", OpUpsert, 1, map[string]interface{}{"title": "a"}); err != nil {
		t.Fatal(err)
	}
	relay := &Relay{
		Store: MemoryRelayStore(queue),
		Bulk: func(ctx context.Context, docs []BulkDoc) error {
			return context.DeadlineExceeded
		},
		Delete: func(ctx context.Context, docID string, payload map[string]interface{}) error { return nil },
	}
	if _, _, err := relay.RunOnce(context.Background()); err == nil {
		t.Fatal("expected relay error")
	}
	if queue.PendingCount() != 1 {
		t.Fatalf("failed upsert must stay pending, got %d", queue.PendingCount())
	}
}
