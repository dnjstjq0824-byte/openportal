package main

import (
	"context"
	"sync"
	"testing"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
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
		"experiments": func() any { return a.ensureExperimentPlatform() },
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

// Without a database handle the experiment platform stays memory-only:
// registration and deterministic assignment work, persistence is a no-op.
func TestExperimentPlatformMemoryModeWithoutDB(t *testing.T) {
	a := &app{}
	platform := a.ensureExperimentPlatform()
	if platform == nil {
		t.Fatal("expected non-nil platform")
	}
	def := ranking.ExperimentDef{
		ID: "mem-smoke", Name: "Memory smoke", Layer: "ranking", Salt: "s",
		SplitPercent: 50, Status: ranking.ExpStatusRunning,
		ControlProfile: "a", TreatmentProfile: "b",
	}
	if err := platform.RegisterExperiment(def); err != nil {
		t.Fatal(err)
	}
	if err := platform.PersistExperiment(context.Background(), def.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := platform.AssignDeterministic(def.ID, "user-1"); err != nil {
		t.Fatal(err)
	}
}
