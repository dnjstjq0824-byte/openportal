package ranking

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// TestMemoryBurstCounterAllow verifies record-then-check sliding-window
// semantics: max events allowed, the (max+1)-th rejected, keys isolated.
func TestMemoryBurstCounterAllow(t *testing.T) {
	c := NewMemoryBurstCounter()
	window := 2 * time.Second
	for i := 0; i < 5; i++ {
		if !c.Allow("k", window, 5) {
			t.Fatalf("event %d should be allowed", i+1)
		}
	}
	if c.Allow("k", window, 5) {
		t.Fatal("6th event within window should be rejected")
	}
	if !c.Allow("other", window, 5) {
		t.Fatal("different key should be isolated")
	}
}

// TestMemoryBurstCounterExpiry verifies old events fall out of the window.
func TestMemoryBurstCounterExpiry(t *testing.T) {
	c := NewMemoryBurstCounter()
	window := 50 * time.Millisecond
	for i := 0; i < 3; i++ {
		if !c.Allow("k", window, 3) {
			t.Fatalf("event %d should be allowed", i+1)
		}
	}
	if c.Allow("k", window, 3) {
		t.Fatal("4th event should be rejected before expiry")
	}
	time.Sleep(70 * time.Millisecond)
	if !c.Allow("k", window, 3) {
		t.Fatal("events should expire from the window")
	}
}

// TestBurstCounterConcurrency exercises the memory counter under -race.
func TestBurstCounterConcurrency(t *testing.T) {
	c := NewMemoryBurstCounter()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				c.Allow(fmt.Sprintf("key-%d", g%4), time.Minute, 100000)
			}
		}(g)
	}
	wg.Wait()
}

// TestRedisBurstCounterParity runs the same verdict script against Redis and
// fails when it diverges from memory semantics. Requires TEST_REDIS_ADDR.
func TestRedisBurstCounterParity(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	r := NewRedisBurstCounter(addr, 2*time.Second)
	window := 5 * time.Second
	key := fmt.Sprintf("parity-%d", time.Now().UnixNano())
	for i := 0; i < 5; i++ {
		if !r.Allow(key, window, 5) {
			t.Fatalf("redis: event %d should be allowed", i+1)
		}
	}
	if r.Allow(key, window, 5) {
		t.Fatal("redis: 6th event within window should be rejected")
	}
	if !r.Allow(key+"-other", window, 5) {
		t.Fatal("redis: different key should be isolated")
	}
}

// TestRedisBurstCounterSharedState proves two counter instances (two replicas)
// share one budget through Redis. Requires TEST_REDIS_ADDR.
func TestRedisBurstCounterSharedState(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	a := NewRedisBurstCounter(addr, 2*time.Second)
	b := NewRedisBurstCounter(addr, 2*time.Second)
	window := 5 * time.Second
	key := fmt.Sprintf("shared-%d", time.Now().UnixNano())
	for i := 0; i < 3; i++ {
		if !a.Allow(key, window, 5) {
			t.Fatalf("replica A event %d should be allowed", i+1)
		}
	}
	for i := 0; i < 2; i++ {
		if !b.Allow(key, window, 5) {
			t.Fatalf("replica B event %d should be allowed (shared count %d)", i+1, 4+i)
		}
	}
	if a.Allow(key, window, 5) || b.Allow(key, window, 5) {
		t.Fatal("6th shared event should be rejected by both replicas")
	}
}

// TestRedisBurstCounterFailOpen verifies the availability-first policy: an
// unreachable Redis allows traffic instead of erroring (C-6).
func TestRedisBurstCounterFailOpen(t *testing.T) {
	r := NewRedisBurstCounter("127.0.0.1:1", 100*time.Millisecond)
	if !r.Allow("k", time.Second, 1) {
		t.Fatal("unreachable redis should fail open (allow)")
	}
}

// TestAbuseDetectorRedisBackend wires InspectQuery through Redis end to end.
// Requires TEST_REDIS_ADDR.
func TestAbuseDetectorRedisBackend(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	ad := NewAbuseDetectorWithCounter(5*time.Second, 3, 100,
		NewRedisBurstCounter(addr, 2*time.Second))
	ip := fmt.Sprintf("10.99.99.%d", time.Now().UnixNano()%250+1)
	for i := 0; i < 3; i++ {
		if rep := ad.InspectQuery(ip, fmt.Sprintf("benign query %d", i)); rep.IsAbusive {
			t.Fatalf("query %d should be benign, got %+v", i, rep)
		}
	}
	rep := ad.InspectQuery(ip, "one more benign query")
	if !rep.IsAbusive || rep.Type != AbuseBurstRate {
		t.Fatalf("4th query should trip burst limit, got %+v", rep)
	}
}
