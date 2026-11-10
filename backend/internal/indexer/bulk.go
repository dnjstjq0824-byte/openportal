package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BulkDoc is one document for _bulk ingestion.
type BulkDoc struct {
	ID      string
	Payload map[string]interface{}
}

type bulkItemResult struct {
	Index struct {
		ID     string `json:"_id"`
		Status int    `json:"status"`
		Error  *struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	} `json:"index"`
}

type bulkResponse struct {
	Errors bool             `json:"errors"`
	Items  []bulkItemResult `json:"items"`
}

// BulkIndex ingests documents with a single OpenSearch _bulk request (C-3).
// Unlike the single-document PUT path it never sets refresh=true: freshly
// indexed documents become visible on the index refresh interval instead of
// forcing a refresh per document.
func (s *IndexingService) BulkIndex(ctx context.Context, docs []BulkDoc) error {
	if s.baseURL == "" || s.indexName == "" {
		return fmt.Errorf("opensearch target not configured")
	}
	if len(docs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, doc := range docs {
		meta, err := json.Marshal(map[string]interface{}{
			"index": map[string]interface{}{"_index": s.indexName, "_id": doc.ID},
		})
		if err != nil {
			return err
		}
		buf.Write(meta)
		buf.WriteByte('\n')
		body, err := json.Marshal(doc.Payload)
		if err != nil {
			return err
		}
		buf.Write(body)
		buf.WriteByte('\n')
	}
	endpoint := fmt.Sprintf("%s/_bulk", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("bulk request failed (%d): %s", resp.StatusCode, string(data))
	}
	var parsed bulkResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("bulk response decode failed: %w", err)
	}
	if !parsed.Errors {
		return nil
	}
	var failed []string
	for _, item := range parsed.Items {
		if item.Index.Status >= 400 {
			reason := ""
			if item.Index.Error != nil {
				reason = item.Index.Error.Type + ": " + item.Index.Error.Reason
			}
			failed = append(failed, fmt.Sprintf("%s(%d %s)", item.Index.ID, item.Index.Status, reason))
		}
	}
	return fmt.Errorf("bulk partially failed: %s", strings.Join(failed, ", "))
}
