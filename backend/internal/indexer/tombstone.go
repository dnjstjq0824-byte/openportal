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

// TombstoneReason constants describe the legal or operational reason for document removal.
type TombstoneReason string

const (
	TombstoneReasonUserDeleted     TombstoneReason = "user_requested_deletion"
	TombstoneReasonRobotsNoIndex   TombstoneReason = "robots_noindex_directive"
	TombstoneReasonLegalTakedown   TombstoneReason = "legal_takedown_dmca"
	TombstoneReasonPrivacyRight    TombstoneReason = "privacy_right_to_be_forgotten"
	TombstoneReasonMalwareSpam     TombstoneReason = "malware_or_spam_deindexed"
	TombstoneReasonDeadHostRemoved TombstoneReason = "dead_host_removed"
)

// TombstoneRecord contains tombstone metadata.
type TombstoneRecord struct {
	DocumentID  string          `json:"document_id"`
	Reason      TombstoneReason `json:"reason"`
	Requester   string          `json:"requester"`
	TombstoneAt time.Time       `json:"tombstone_at"`
	PurgeAfter  time.Time       `json:"purge_after"`
}

// TombstoneManager handles marking documents as deleted and propagating deletion
// tombstones to OpenSearch inverted indices, cache, and outbox.
type TombstoneManager struct {
	baseURL    string
	indexName  string
	httpClient *http.Client
	outbox     *OutboxQueue
}

// NewTombstoneManager constructs a TombstoneManager.
func NewTombstoneManager(baseURL, indexName string, client *http.Client, outbox *OutboxQueue) *TombstoneManager {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &TombstoneManager{
		baseURL:    baseURL,
		indexName:  indexName,
		httpClient: client,
		outbox:     outbox,
	}
}

// ApplyTombstone marks the document as deleted, updates OpenSearch with is_tombstone=true and indexable=false,
// enqueues an outbox delete event, and ensures immediate search exclusion.
func (m *TombstoneManager) ApplyTombstone(ctx context.Context, docID string, reason TombstoneReason, requester string) (*TombstoneRecord, error) {
	now := time.Now().UTC()
	record := &TombstoneRecord{
		DocumentID:  docID,
		Reason:      reason,
		Requester:   requester,
		TombstoneAt: now,
		PurgeAfter:  now.Add(24 * time.Hour), // 24hr grace before physical vacuum
	}

	// 1. Enqueue outbox delete event
	if m.outbox != nil {
		_, _ = m.outbox.Enqueue(docID, OpDelete, 1, map[string]interface{}{
			"reason":       string(reason),
			"requester":    requester,
			"tombstone_at": now.Format(time.RFC3339),
		})
	}

	// 2. Propagate to OpenSearch: update indexable=false, is_tombstone=true, tombstone_reason
	if m.baseURL != "" && m.indexName != "" {
		updatePayload := map[string]interface{}{
			"doc": map[string]interface{}{
				"indexable":        false,
				"is_tombstone":     true,
				"tombstone_reason": string(reason),
			},
		}
		data, err := json.Marshal(updatePayload)
		if err != nil {
			return nil, err
		}

		endpoint := fmt.Sprintf("%s/%s/_update/%s?refresh=true", m.baseURL, m.indexName, docID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := m.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("opensearch tombstone update failed (%d): %s", resp.StatusCode, string(body))
		}
	}

	return record, nil
}

// PurgePhysically performs permanent document removal from OpenSearch after retention period.
func (m *TombstoneManager) PurgePhysically(ctx context.Context, docID string) error {
	if m.baseURL == "" || m.indexName == "" {
		return nil
	}
	endpoint := fmt.Sprintf("%s/%s/_doc/%s?refresh=true", m.baseURL, m.indexName, docID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("physical purge failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

