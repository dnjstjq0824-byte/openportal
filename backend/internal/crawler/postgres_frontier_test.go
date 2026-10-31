package crawler

import (
	"testing"
	"time"
)

func TestPostgresFrontierHostCacheAndAllow(t *testing.T) {
	pf := NewPostgresFrontier(nil) // nil db operates with host cache

	host := &VerifiedHost{
		ID:           "test-host-1",
		Scheme:       "https",
		Hostname:     "example.com",
		Port:         443,
		HostKey:      "https://example.com:443",
		FetchEnabled: true,
		CrawlDelay:   250 * time.Millisecond,
		CreatedAt:    time.Now().UTC(),
	}

	pf.RegisterVerifiedHost(host)

	found, allowed := pf.IsHostAllowed("https://example.com:443")
	if !allowed || found == nil {
		t.Fatalf("expected host to be allowed, got allowed=%v, found=%v", allowed, found)
	}
	if found.Hostname != "example.com" {
		t.Errorf("expected hostname example.com, got %s", found.Hostname)
	}

	_, unauth := pf.IsHostAllowed("https://unregistered.test:443")
	if unauth {
		t.Errorf("expected unregistered host to be disallowed")
	}

	// Submit URL for registered host
	item, err := pf.SubmitURL("https://example.com/test-page", 50)
	if err != nil {
		t.Fatalf("expected submit to succeed, got %v", err)
	}
	if item.Priority != 50 {
		t.Errorf("expected priority 50, got %d", item.Priority)
	}
	if item.State != StateQueued {
		t.Errorf("expected StateQueued, got %s", item.State)
	}

	// Submit URL for unregistered host
	_, err = pf.SubmitURL("https://unregistered.test/page", 10)
	if err != ErrHostNotRegistered {
		t.Errorf("expected ErrHostNotRegistered, got %v", err)
	}
}

