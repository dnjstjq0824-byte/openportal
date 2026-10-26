package indexer

import (
	"testing"
	"time"
)

func TestMetricsCollector(t *testing.T) {
	q := NewOutboxQueue(10*time.Second, 3)
	collector := NewMetricsCollector(q)

	// Record crawl outcomes
	collector.RecordCrawl(true, 120)
	collector.RecordCrawl(true, 180)
	collector.RecordCrawl(false, 350)

	// Record indexed items
	now := time.Now().UTC()
	collector.RecordIndexed(now.Add(-5*time.Second), now.Add(-2*time.Hour))
	collector.RecordIndexed(now.Add(-3*time.Second), now.Add(-1*time.Hour))

	collector.RecordTombstone(false)
	collector.RecordTombstone(true)
	collector.RecordNearDuplicate()

	snap := collector.Snapshot()
	if snap.TotalCrawled != 3 {
		t.Errorf("expected 3 crawled, got %d", snap.TotalCrawled)
	}
	if snap.SuccessfulCrawls != 2 {
		t.Errorf("expected 2 successful, got %d", snap.SuccessfulCrawls)
	}
	if snap.FailedCrawls != 1 {
		t.Errorf("expected 1 failed, got %d", snap.FailedCrawls)
	}
	if snap.SuccessRate < 0.65 || snap.SuccessRate > 0.67 {
		t.Errorf("unexpected success rate: %f", snap.SuccessRate)
	}
	if snap.TotalIndexed != 2 {
		t.Errorf("expected 2 indexed, got %d", snap.TotalIndexed)
	}
	if snap.TombstonesApplied != 1 || snap.TombstonesPurged != 1 {
		t.Errorf("unexpected tombstone counts")
	}
	if snap.NearDuplicatesDetected != 1 {
		t.Errorf("unexpected near duplicate count: %d", snap.NearDuplicatesDetected)
	}
}

