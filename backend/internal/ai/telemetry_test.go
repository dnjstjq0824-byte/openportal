package ai

import (
	"testing"
)

func TestAITelemetryTracker(t *testing.T) {
	tracker := NewAITelemetryTracker(0.01) // $0.01 daily cap for testing

	// 1st query: 1000 tokens ($0.002) -> below cap
	_, exceeded := tracker.RecordQuery("q1", 500, 500, 120)
	if exceeded {
		t.Errorf("expected cap not reached after 1st query")
	}

	// 2nd query: 5000 tokens ($0.010) -> total $0.012 -> exceeds cap
	_, exceeded = tracker.RecordQuery("q2", 2500, 2500, 300)
	if !exceeded {
		t.Errorf("expected cap exceeded after 2nd query")
	}
	if !tracker.IsDailyCapReached() {
		t.Errorf("expected IsDailyCapReached to return true")
	}

	count, totalCost := tracker.GetSummary()
	if count != 2 || totalCost < 0.01 {
		t.Errorf("unexpected summary: count=%d, cost=%f", count, totalCost)
	}
}
