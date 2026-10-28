package main

import (
	"sync"
	"testing"
)

// All concurrent first requests must share the same stateful services.
func TestConcurrentServiceInitialization(t *testing.T) {
	a := &app{}
	services := map[string]func() any{
		"identity":   func() any { return a.ensureStore() },
		"abuse":      func() any { return a.ensureAbuseDetector() },
		"ranking":    func() any { return a.ensureReranker() },
		"safety":     func() any { return a.ensureSafeSearch() },
		"moderation": func() any { return a.ensureModeration() },
	}
	for name, get := range services {
		t.Run(name, func(t *testing.T) {
			const workers = 64
			start := make(chan struct{})
			results := make(chan any, workers)
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					results <- get()
				}()
			}
			close(start)
			wg.Wait()
			close(results)
			want := get()
			for got := range results {
				if got != want {
					t.Fatal("concurrent requests received different service instances")
				}
			}
		})
	}
}
