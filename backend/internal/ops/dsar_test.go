package ops

import (
	"testing"
	"time"
)

func TestDSARManagerExportAndErasure(t *testing.T) {
	mgr := NewDSARManager()
	acc := "acc-user-privacy-1"

	// 1. Submit export request
	expReq, err := mgr.SubmitRequest(acc, DSARExport)
	if err != nil {
		t.Fatalf("failed to submit export request: %v", err)
	}
	if expReq.Status != "pending" {
		t.Errorf("expected pending status, got %s", expReq.Status)
	}

	// 2. Process export archive
	archive := UserDataArchive{
		AccountID:      acc,
		Email:          "user@example.com",
		RecentSearches: []string{"search engine architecture", "golang"},
		ExportedAt:     time.Now(),
	}
	completedExp, err := mgr.ProcessExport(expReq.ID, archive)
	if err != nil || completedExp.Status != "completed" || completedExp.ExportData == "" {
		t.Fatalf("failed processing export archive: %v", err)
	}

	// 3. Submit and execute erasure request
	eraseReq, err := mgr.SubmitRequest(acc, DSARErasure)
	if err != nil {
		t.Fatalf("failed submitting erasure request: %v", err)
	}

	purged := false
	purgeHook := func(accountID string) error {
		if accountID == acc {
			purged = true
		}
		return nil
	}

	if err := mgr.ExecuteErasure(eraseReq.ID, purgeHook); err != nil {
		t.Fatalf("failed executing erasure: %v", err)
	}
	if !purged {
		t.Errorf("expected purgeHook to be called for %s", acc)
	}
}
