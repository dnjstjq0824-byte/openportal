package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IndexingService integrates HTML boilerplate extraction, SimHash deduplication,
// transactional outbox events, and OpenSearch ingestion with deletion tombstone propagation.
type IndexingService struct {
	remover     *BoilerplateRemover
	outbox      *OutboxQueue
	tombstones  *TombstoneManager
	metrics     *MetricsCollector
	baseURL     string
	indexName   string
	httpClient  *http.Client
	docVersions map[string]*DocumentVersion
	knownHashes map[uint64]string // simhash -> docID
}

// NewIndexingService constructs a complete indexing service.
func NewIndexingService(baseURL, indexName string, client *http.Client) *IndexingService {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	outbox := NewOutboxQueue(15*time.Second, 5)
	tombstones := NewTombstoneManager(baseURL, indexName, client, outbox)
	metrics := NewMetricsCollector(outbox)

	return &IndexingService{
		remover:     NewBoilerplateRemover(),
		outbox:      outbox,
		tombstones:  tombstones,
		metrics:     metrics,
		baseURL:     baseURL,
		indexName:   indexName,
		httpClient:  client,
		docVersions: make(map[string]*DocumentVersion),
		knownHashes: make(map[uint64]string),
	}
}

// IngestCrawlResult processes freshly fetched HTML: strips boilerplate, checks SimHash near-duplicate,
// detects content modification, enqueues an outbox upsert event, and dispatches to OpenSearch.
func (s *IndexingService) IngestCrawlResult(ctx context.Context, docID, rawURL string, rawHTML []byte, crawledAt time.Time) (*ExtractedDoc, bool, error) {
	extracted, err := s.remover.Extract(rawHTML, rawURL)
	if err != nil {
		return nil, false, fmt.Errorf("boilerplate extraction error: %w", err)
	}

	// 1. Calculate SimHash
	extracted.SimHash = SimHash(extracted.Title + " " + extracted.Body)

	// 2. Check for near-duplicates among known documents
	isNearDup := false
	for knownHash := range s.knownHashes {
		if IsNearDuplicate(extracted.SimHash, knownHash, 4) {
			isNearDup = true
			s.metrics.RecordNearDuplicate()
			break
		}
	}
	s.knownHashes[extracted.SimHash] = docID

	// 3. Detect change vs previous stored version
	prev := s.docVersions[docID]
	decision, contentHash := DetectChange(prev, extracted.Body, extracted.SimHash)
	extracted.ContentHash = contentHash

	if decision == DecisionUnchanged {
		// Content unchanged, skip re-indexing
		return extracted, false, nil
	}

	// 4. Update stored version
	versionNum := 1
	if prev != nil {
		versionNum++
	}
	s.docVersions[docID] = &DocumentVersion{
		DocumentID:  docID,
		ContentHash: contentHash,
		SimHash:     extracted.SimHash,
		WordCount:   extracted.WordCount,
	}

	// 5. Enqueue outbox upsert event
	docPayload := map[string]interface{}{
		"document_id":       docID,
		"url":               rawURL,
		"title":             extracted.Title,
		"description":       extracted.Description,
		"body":              extracted.Body,
		"headings":          extracted.Headings,
		"anchor_texts":      extracted.AnchorTexts,
		"language":          extracted.Language,
		"simhash":           FormatSimHash(extracted.SimHash),
		"indexable":         true,
		"is_representative": !isNearDup,
		"is_tombstone":      false,
		"last_crawled_at":   crawledAt.Format(time.RFC3339),
	}

	ev, err := s.outbox.Enqueue(docID, OpUpsert, versionNum, docPayload)
	if err != nil {
		return nil, false, fmt.Errorf("outbox enqueue failed: %w", err)
	}

	// 6. If OpenSearch target is configured, dispatch directly (or via consumer worker)
	if s.baseURL != "" && s.indexName != "" {
		if err := s.dispatchToOpenSearch(ctx, docID, docPayload); err != nil {
			s.outbox.Nack(ev.ID, err)
			return nil, false, err
		}
		_ = s.outbox.Ack(ev.ID)
		s.metrics.RecordIndexed(crawledAt, crawledAt)
	}

	return extracted, true, nil
}

// ApplyTombstone marks a document as legally/user deleted, purges from active search,
// updates OpenSearch, and records the tombstone.
func (s *IndexingService) ApplyTombstone(ctx context.Context, docID string, reason TombstoneReason, requester string) error {
	_, err := s.tombstones.ApplyTombstone(ctx, docID, reason, requester)
	if err != nil {
		return err
	}
	s.metrics.RecordTombstone(false)
	return nil
}

// Metrics returns the telemetry snapshot.
func (s *IndexingService) Metrics() IndexerSnapshot {
	return s.metrics.Snapshot()
}

// Outbox returns the underlying outbox queue for verification.
func (s *IndexingService) Outbox() *OutboxQueue {
	return s.outbox
}

func (s *IndexingService) dispatchToOpenSearch(ctx context.Context, docID string, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/%s/_doc/%s?refresh=true", s.baseURL, s.indexName, docID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch doc index failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

