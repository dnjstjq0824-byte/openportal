package indexer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPipelineRetryAfterIndexFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	svc := NewIndexingService(server.URL, "documents", server.Client())
	html := []byte("<html><head><title>Retry</title></head><body><p>Index this document after a temporary service failure.</p></body></html>")
	if _, _, err := svc.IngestCrawlResult(context.Background(), "retry", "https://example.org/retry", html, time.Now()); err == nil {
		t.Fatal("expected initial index failure")
	}
	_, indexed, err := svc.IngestCrawlResult(context.Background(), "retry", "https://example.org/retry", html, time.Now())
	if err != nil || !indexed || attempts != 2 {
		t.Fatalf("retry: indexed=%v attempts=%d err=%v", indexed, attempts, err)
	}
}

func TestPipelineVersionsAndSelfDeduplication(t *testing.T) {
	svc := NewIndexingService("", "", nil)
	for version := 1; version <= 3; version++ {
		html := fmt.Appendf(nil, "<html><head><title>Revision %d</title></head><body><p>This document describes a stable search architecture and its indexing pipeline.</p></body></html>", version)
		if _, indexed, err := svc.IngestCrawlResult(context.Background(), "same", "https://example.org/same", html, time.Now()); err != nil || !indexed {
			t.Fatalf("revision %d: indexed=%v err=%v", version, indexed, err)
		}
		events, err := svc.Outbox().LeaseBatch(context.Background(), 1)
		if err != nil || len(events) != 1 {
			t.Fatalf("lease: events=%v err=%v", events, err)
		}
		if events[0].PayloadVersion != version {
			t.Errorf("version=%d want=%d", events[0].PayloadVersion, version)
		}
		if events[0].Payload["is_representative"] != true {
			t.Error("document incorrectly treated as its own duplicate")
		}
		if err := svc.Outbox().Ack(events[0].ID); err != nil {
			t.Fatal(err)
		}
	}
}

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
