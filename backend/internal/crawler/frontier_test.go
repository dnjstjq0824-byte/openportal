package crawler

import (
	"testing"
	"time"
)

func TestFrontierLifecycle(t *testing.T) {
	frontier := NewFrontier()

	// 1. Unregistered host must be rejected
	_, err := frontier.SubmitURL("https://unregistered.com/page", 50)
	if err != ErrHostNotRegistered {
		t.Errorf("expected ErrHostNotRegistered, got %v", err)
	}

	// 2. Register verified host
	host := &VerifiedHost{
		ID:           "host-001",
		Scheme:       "https",
		Hostname:     "example.com",
		Port:         443,
		HostKey:      "https://example.com:443",
		FetchEnabled: true,
		CrawlDelay:   1 * time.Second,
	}
	frontier.RegisterVerifiedHost(host)

	// 3. Submit seed URL with priority 90
	item1, err := frontier.SubmitURL("https://example.com/seed", 90)
	if err != nil {
		t.Fatalf("SubmitURL seed failed: %v", err)
	}
	if item1.Priority != 90 || item1.State != StateQueued {
		t.Errorf("item1 mismatch: %+v", item1)
	}

	// 4. Duplicate submission returns ErrFrontierURLAlreadyExists
	_, err = frontier.SubmitURL("https://example.com/seed", 80)
	if err != ErrFrontierURLAlreadyExists {
		t.Errorf("expected ErrFrontierURLAlreadyExists on duplicate, got %v", err)
	}

	// 5. Submit second URL with lower priority (30)
	item2, err := frontier.SubmitURL("https://example.com/page2", 30)
	if err != nil {
		t.Fatalf("SubmitURL page2 failed: %v", err)
	}

	// 6. Lease acquisition: higher priority item1 should be leased first
	leased := frontier.LeaseAcquire("worker-1", 1, 10*time.Second)
	if len(leased) != 1 {
		t.Fatalf("expected 1 leased item, got %d", len(leased))
	}
	if leased[0].URLHash != item1.URLHash {
		t.Errorf("expected item1 (priority 90) to be leased first, got %s", leased[0].URL)
	}
	if leased[0].State != StateLeased || leased[0].LeaseOwner != "worker-1" {
		t.Errorf("leased state mismatch: %+v", leased[0])
	}

	// 7. Ack successful crawl
	err = frontier.Ack(item1.URLHash, "worker-1")
	if err != nil {
		t.Fatalf("Ack failed: %v", err)
	}
	if item1.State != StateDone {
		t.Errorf("expected StateDone, got %s", item1.State)
	}

	// 8. Lease item2 and test Nack retry logic
	leased2 := frontier.LeaseAcquire("worker-2", 1, 10*time.Second)
	if len(leased2) != 1 || leased2[0].URLHash != item2.URLHash {
		t.Fatalf("expected item2 to be leased, got %v", leased2)
	}

	// First Nack (Retry 1)
	err = frontier.Nack(item2.URLHash, "worker-2", "network timeout")
	if err != nil {
		t.Fatalf("Nack failed: %v", err)
	}
	if item2.State != StateQueued || item2.RetryCount != 1 {
		t.Errorf("after first Nack: state=%s, retry=%d", item2.State, item2.RetryCount)
	}

	// Exhaust retries up to MaxRetries (3)
	item2.RetryCount = 3
	item2.State = StateLeased
	item2.LeaseOwner = "worker-2"
	err = frontier.Nack(item2.URLHash, "worker-2", "fatal 500 error")
	if err != nil {
		t.Fatalf("final Nack failed: %v", err)
	}
	if item2.State != StateFailed {
		t.Errorf("expected StateFailed on retry exhaustion, got %s", item2.State)
	}

	// 9. Verify Queue depths
	if frontier.QueueDepth(StateDone) != 1 {
		t.Errorf("expected 1 done item, got %d", frontier.QueueDepth(StateDone))
	}
	if frontier.QueueDepth(StateFailed) != 1 {
		t.Errorf("expected 1 failed item, got %d", frontier.QueueDepth(StateFailed))
	}
}

