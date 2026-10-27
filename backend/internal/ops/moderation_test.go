package ops

import (
	"testing"
)

func TestModerationTakedownAndAppeal(t *testing.T) {
	mgr := NewModerationTakedownManager()
	url := "https://example.org/infringing-article"

	// 1. Initial state not tombstoned
	if mgr.IsTombstoned(url) {
		t.Errorf("url should not be tombstoned initially")
	}

	// 2. Execute DMCA takedown
	rec, err := mgr.ExecuteTakedown(url, ReasonCopyrightDMCA, "counsel@rights-holder.org", "DMCA Section 512 Notice")
	if err != nil {
		t.Fatalf("failed executing takedown: %v", err)
	}
	if !rec.Tombstoned {
		t.Errorf("expected tombstoned record")
	}

	// URL must now be tombstoned
	if !mgr.IsTombstoned(url) {
		t.Errorf("url must be tombstoned after takedown")
	}

	// 3. Submit appeal
	if err := mgr.SubmitAppeal(url, "Authorized fair-use educational quotation"); err != nil {
		t.Fatalf("failed submitting appeal: %v", err)
	}
}
