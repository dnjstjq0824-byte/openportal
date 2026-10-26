package indexer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestIndexingPipelineEndToEnd(t *testing.T) {
	ctx := context.Background()
	svc := NewIndexingService("", "", nil) // in-memory outbox mode

	rawHTML := `<!DOCTYPE html>
<html>
<head>
  <title>오픈포털 검색엔진 아키텍처</title>
  <meta name="description" content="자체 크롤러와 역색인 기술 아키텍처">
  <link rel="canonical" href="https://example.org/architecture">
  <script>var x = 1;</script>
</head>
<body>
  <nav><a href="/">메인</a></nav>
  <h1>오픈포털 고성능 색인 시스템</h1>
  <p>오픈포털은 대한민국 사용자들을 위한 독립 웹 검색엔진 시스템입니다. 분산 아키텍처를 기반으로 크롤러, 형태소 분석기, 역색인 및 랭킹 엔진이 긴밀하게 결합되어 동작합니다. 대규모 웹 문서에서 유용한 정보를 빠르고 정확하게 검색할 수 있도록 지원하며 사용자의 프라이버시를 철저히 보장합니다.</p>
  <footer>Copyright 2026</footer>
</body>
</html>`

	// 1. Ingest initial document
	crawledAt := time.Now().UTC()
	doc, indexed, err := svc.IngestCrawlResult(ctx, "doc-arch-001", "https://example.org/architecture", []byte(rawHTML), crawledAt)
	if err != nil {
		t.Fatalf("IngestCrawlResult failed: %v", err)
	}
	if !indexed {
		t.Errorf("expected initial document to be indexed")
	}
	if doc.Language != "ko" {
		t.Errorf("expected language 'ko', got %s", doc.Language)
	}
	if doc.Title != "오픈포털 검색엔진 아키텍처" {
		t.Errorf("unexpected title: %q", doc.Title)
	}
	if strings.Contains(doc.Body, "var x = 1") || strings.Contains(doc.Body, "Copyright 2026") {
		t.Errorf("body contains boilerplate")
	}

	// 2. Ingest identical document (no change detection)
	_, indexedAgain, err := svc.IngestCrawlResult(ctx, "doc-arch-001", "https://example.org/architecture", []byte(rawHTML), crawledAt)
	if err != nil {
		t.Fatalf("second IngestCrawlResult failed: %v", err)
	}
	if indexedAgain {
		t.Errorf("expected identical document to NOT be indexed again")
	}

	// 3. Ingest near-duplicate document with different ID
	nearDupHTML := strings.Replace(rawHTML, "검색할 수 있도록", "찾아볼 수 있도록", 1)
	_, nearDupIndexed, err := svc.IngestCrawlResult(ctx, "doc-arch-002", "https://example.org/architecture-mirror", []byte(nearDupHTML), crawledAt)
	if err != nil {
		t.Fatalf("near duplicate IngestCrawlResult failed: %v", err)
	}
	if !nearDupIndexed {
		t.Errorf("expected near-duplicate document to be processed")
	}

	// Verify near duplicate recorded in metrics
	snap := svc.Metrics()
	if snap.NearDuplicatesDetected < 1 {
		t.Errorf("expected near duplicates detected >= 1, got %d", snap.NearDuplicatesDetected)
	}

	// 4. Test tombstone propagation
	if err := svc.ApplyTombstone(ctx, "doc-arch-001", TombstoneReasonUserDeleted, "user-owner"); err != nil {
		t.Fatalf("ApplyTombstone failed: %v", err)
	}

	snapAfterTombstone := svc.Metrics()
	if snapAfterTombstone.TombstonesApplied != 1 {
		t.Errorf("expected 1 tombstone applied, got %d", snapAfterTombstone.TombstonesApplied)
	}
}

