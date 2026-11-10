package crawler

import (
	"testing"
	"time"
)

// C-1: completed items older than the cutoff re-enter the queue for recrawls.
func TestRescheduleStaleMemory(t *testing.T) {
	frontier := NewFrontier()
	frontier.RegisterVerifiedHost(&VerifiedHost{
		ID: "h", Scheme: "https", Hostname: "example.test", Port: 443,
		HostKey: "https://example.test:443", FetchEnabled: true, CreatedAt: time.Now().UTC(),
	})
	if _, err := frontier.SubmitURL("https://example.test/old", 10); err != nil {
		t.Fatal(err)
	}
	leased := frontier.LeaseAcquire("w", 1, time.Minute)
	if len(leased) != 1 {
		t.Fatal("expected lease")
	}
	if err := frontier.Ack(leased[0].URLHash, "w"); err != nil {
		t.Fatal(err)
	}
	// Fresh completions are untouched: the cutoff is a staleness horizon in
	// the past (production passes now-7d), never "now" itself — anything
	// completed before the call instant is definitionally older than it.
	if n, _ := frontier.RescheduleStale(time.Now().UTC().Add(-time.Hour), 10); n != 0 {
		t.Fatalf("fresh done must stay done, got %d", n)
	}
	// Backdate the completion, then reschedule.
	frontier.mu.Lock()
	frontier.itemsByHash[leased[0].URLHash].ScheduledAt = time.Now().UTC().Add(-8 * 24 * time.Hour)
	frontier.mu.Unlock()
	n, err := frontier.RescheduleStale(time.Now().UTC().Add(-7*24*time.Hour), 10)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	again := frontier.LeaseAcquire("w", 10, time.Minute)
	if len(again) != 1 || again[0].URLHash != leased[0].URLHash {
		t.Fatalf("re-queued item must be leasable: %v", again)
	}
}
