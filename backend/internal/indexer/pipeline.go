package indexer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
)

// IndexingService integrates HTML boilerplate extraction, SimHash deduplication,
// transactional outbox events, and OpenSearch ingestion with deletion tombstone propagation.
type IndexingService struct {
	mu          sync.Mutex
	remover     *BoilerplateRemover
	outbox      *OutboxQueue
	pgOutbox    *PGOutbox
	tombstones  *TombstoneManager
	safety      *ranking.SafeSearchClassifier
	metrics     *MetricsCollector
	baseURL     string
	indexName   string
	httpClient  *http.Client
	docVersions map[string]*DocumentVersion
	knownHashes map[uint64]string // simhash -> docID
}

// EnableDurableOutbox switches the service to the transactional outbox pattern
// (C-3): IngestCrawlResult and ApplyTombstone record intent in PostgreSQL and
// return; the returned Relay delivers events to OpenSearch in bulk. Without it
// the service keeps the direct single-document dispatch path.
func (s *IndexingService) EnableDurableOutbox(db *sql.DB) *Relay {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pgOutbox = NewPGOutbox(db, 30*time.Second, 5)
	s.tombstones.WithDB(db)
	return &Relay{
		Store: PGRelayStore(s.pgOutbox),
		Bulk:  s.BulkIndex,
		Delete: func(ctx context.Context, docID string, payload map[string]interface{}) error {
			reason := TombstoneReasonUserDeleted
			if raw, ok := payload["reason"].(string); ok && raw != "" {
				reason = TombstoneReason(raw)
			}
			requester, _ := payload["requester"].(string)
			_, err := s.tombstones.ApplyTombstone(ctx, docID, reason, requester)
			return err
		},
	}
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
		safety:      ranking.NewSafeSearchClassifier(),
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
	s.mu.Lock()
	defer s.mu.Unlock()
	extracted, err := s.remover.Extract(rawHTML, rawURL)
	if err != nil {
		return nil, false, fmt.Errorf("boilerplate extraction error: %w", err)
	}

	// 1. Calculate SimHash
	extracted.SimHash = SimHash(extracted.Title + " " + extracted.Body)

	// 2. Check for near-duplicates among known documents
	isNearDup := false
	for knownHash, knownID := range s.knownHashes {
		if knownID != docID && IsNearDuplicate(extracted.SimHash, knownHash, 4) {
			isNearDup = true
			s.metrics.RecordNearDuplicate()
			break
		}
	}

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
		versionNum = prev.Version + 1
	}
	nextVersion := &DocumentVersion{
		Version:     versionNum,
		DocumentID:  docID,
		ContentHash: contentHash,
		SimHash:     extracted.SimHash,
		WordCount:   extracted.WordCount,
	}

	canonicalURL := extracted.CanonicalURL
	if canonicalURL == "" {
		canonicalURL = rawURL
	}
	host := ""
	if u, err := url.Parse(canonicalURL); err == nil {
		host = strings.ToLower(u.Hostname())
	}
	if host == "" {
		if u, err := url.Parse(rawURL); err == nil {
			host = strings.ToLower(u.Hostname())
		}
	}

	spamScore := computeSpamScore(extracted)

	// C-5: index-time safety labels so SafeSearch filters inside OpenSearch
	// (consistent total + pagination) instead of post-filtering the page.
	safetyCat, adultScore, malwareScore := s.safety.IndexLabel(extracted.Title, extracted.Description, canonicalURL)

	// 5. Enqueue outbox upsert event
	docPayload := map[string]interface{}{
		"safety":                 safetyCat,
		"adult_score":            adultScore,
		"malware_score":          malwareScore,
		"document_id":            docID,
		"url":                    rawURL,
		"canonical_url":          canonicalURL,
		"host":                   host,
		"title":                  extracted.Title,
		"description":            extracted.Description,
		"body":                   extracted.Body,
		"headings":               extracted.Headings,
		"anchor_texts":           extracted.AnchorTexts,
		"language":               extracted.Language,
		"simhash":                FormatSimHash(extracted.SimHash),
		"indexable":              true,
		"is_representative":      !isNearDup,
		"is_tombstone":           false,
		"last_crawled_at":        crawledAt.Format(time.RFC3339),
		"domain_authority":       domainAuthorityScore(canonicalURL),
		"spam_score":             spamScore,
		"source":                 "crawler",
		"published_at":           crawledAt.Format(time.RFC3339),
		"term_frequency_density": computeTermFrequencyDensity(extracted.Body),
		"suggest": map[string]interface{}{
			"input": SuggestInputs(extracted.Title, extracted.Headings),
		},
	}

	// Durable path (C-3): record intent in PostgreSQL; the relay worker owns
	// delivery, so no direct dispatch happens here.
	if s.pgOutbox != nil {
		if _, err := s.pgOutbox.Enqueue(ctx, docID, OpUpsert, versionNum, docPayload); err != nil {
			return nil, false, fmt.Errorf("durable outbox enqueue failed: %w", err)
		}
		s.metrics.RecordIndexed(crawledAt, crawledAt)
	} else {
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
	}

	// Commit change-detection state only after the configured destination accepts
	// the document, so a failed HTTP dispatch cannot suppress a subsequent retry.
	s.docVersions[docID] = nextVersion
	for hash, knownID := range s.knownHashes {
		if knownID == docID {
			delete(s.knownHashes, hash)
		}
	}
	s.knownHashes[extracted.SimHash] = docID
	return extracted, true, nil
}

// ApplyTombstone marks a document as legally/user deleted, purges from active search,
// updates OpenSearch, and records the tombstone.
func (s *IndexingService) ApplyTombstone(ctx context.Context, docID string, reason TombstoneReason, requester string) error {
	// Durable path (C-3): record the delete intent; the relay worker applies
	// the OpenSearch tombstone and the audit row.
	if s.pgOutbox != nil {
		_, err := s.pgOutbox.Enqueue(ctx, docID, OpDelete, 1, map[string]interface{}{
			"reason":    string(reason),
			"requester": requester,
		})
		if err != nil {
			return err
		}
		s.metrics.RecordTombstone(false)
		return nil
	}
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

// domainAuthorityScore is a lightweight heuristic for ranking signals.
// It prefers HTTPS and shorter, stable domains. Real authority should come from
// a link-graph/quality pipeline in production.
func domainAuthorityScore(rawURL string) float64 {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return 0.3
	}
	score := 0.4
	if u.Scheme == "https" {
		score += 0.2
	}
	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	if len(host) <= 20 {
		score += 0.1
	}
	if strings.Contains(host, ".go.kr") || strings.Contains(host, ".ac.kr") || strings.Contains(host, ".or.kr") {
		score += 0.1
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func computeTermFrequencyDensity(text string) float64 {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 5 {
		return 0.05
	}
	freq := make(map[string]int, len(words))
	maxCount := 0
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()[]{}")
		if len(w) < 2 {
			continue
		}
		freq[w]++
		if freq[w] > maxCount {
			maxCount = freq[w]
		}
	}
	if maxCount == 0 {
		return 0.05
	}
	return float64(maxCount) / float64(len(words))
}

func computeSpamScore(doc *ExtractedDoc) float64 {
	if doc == nil {
		return 1.0
	}
	var score float64
	if doc.WordCount < 10 || len(strings.TrimSpace(doc.Body)) < 30 {
		score += 0.4
	}
	if len(strings.TrimSpace(doc.Title)) == 0 {
		score += 0.3
	} else if len(doc.Title) > 300 {
		score += 0.2
	}
	density := computeTermFrequencyDensity(doc.Body)
	if density > 0.4 {
		score += 0.3
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}
