package indexer

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestReindexAndAliasLifecycle(t *testing.T) {
	ctx := context.Background()
	baseURL := "http://127.0.0.1:9200"

	client := &http.Client{Timeout: 10 * time.Second}
	mgr := NewReindexManager(baseURL, client)

	// Check if OpenSearch is available
	exists, err := mgr.CheckIndexExists(ctx, "documents_v2")
	if err != nil || !exists {
		t.Skip("OpenSearch documents_v2 not reachable; skipping live integration test")
		return
	}

	// 1. Reindex from documents_v1 to documents_v2
	v1Exists, _ := mgr.CheckIndexExists(ctx, "documents_v1")
	if v1Exists {
		if err := mgr.Reindex(ctx, "documents_v1", "documents_v2"); err != nil {
			t.Fatalf("Reindex failed: %v", err)
		}
	}

	// 2. Point alias 'documents_search_test' to documents_v1 first
	aliasName := "documents_search_test"
	if v1Exists {
		if err := mgr.SwapAlias(ctx, aliasName, "", "documents_v1"); err != nil {
			t.Fatalf("Initial alias set failed: %v", err)
		}
		target1, err := mgr.GetAliasTarget(ctx, aliasName)
		if err != nil || target1 != "documents_v1" {
			t.Fatalf("expected alias target 'documents_v1', got %q (err: %v)", target1, err)
		}

		// 3. Blue-Green Switch: atomic swap to documents_v2
		if err := mgr.SwapAlias(ctx, aliasName, "documents_v1", "documents_v2"); err != nil {
			t.Fatalf("Blue-green alias swap failed: %v", err)
		}
		target2, err := mgr.GetAliasTarget(ctx, aliasName)
		if err != nil || target2 != "documents_v2" {
			t.Fatalf("expected alias target 'documents_v2', got %q (err: %v)", target2, err)
		}

		// 4. Rollback: atomic swap back to documents_v1
		if err := mgr.RollbackAlias(ctx, aliasName, "documents_v2", "documents_v1"); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}
		targetRollback, err := mgr.GetAliasTarget(ctx, aliasName)
		if err != nil || targetRollback != "documents_v1" {
			t.Fatalf("expected rollback target 'documents_v1', got %q", targetRollback)
		}
	}
}

