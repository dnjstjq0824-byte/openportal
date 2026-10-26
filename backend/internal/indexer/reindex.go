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

// ReindexManager coordinates OpenSearch mapping versioning, aliases, Blue-Green migrations, and rollback.
type ReindexManager struct {
	baseURL    string
	httpClient *http.Client
}

// NewReindexManager constructs a reindex manager targeting OpenSearch.
func NewReindexManager(baseURL string, client *http.Client) *ReindexManager {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &ReindexManager{
		baseURL:    baseURL,
		httpClient: client,
	}
}

// CreateIndex creates an OpenSearch physical index with given settings and mappings.
func (m *ReindexManager) CreateIndex(ctx context.Context, indexName string, schemaJSON []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, m.baseURL+"/"+indexName, bytes.NewReader(schemaJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index %s failed (%d): %s", indexName, resp.StatusCode, string(body))
	}
	return nil
}

// CheckIndexExists checks if an index exists.
func (m *ReindexManager) CheckIndexExists(ctx context.Context, indexName string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, m.baseURL+"/"+indexName, nil)
	if err != nil {
		return false, err
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// Reindex copies documents from sourceIndex to destIndex using the OpenSearch _reindex API.
func (m *ReindexManager) Reindex(ctx context.Context, sourceIndex, destIndex string) error {
	payload := map[string]interface{}{
		"source": map[string]interface{}{
			"index": sourceIndex,
		},
		"dest": map[string]interface{}{
			"index": destIndex,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/_reindex?wait_for_completion=true", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reindex from %s to %s failed (%d): %s", sourceIndex, destIndex, resp.StatusCode, string(body))
	}

	var result struct {
		Failures []interface{} `json:"failures"`
		Total    int64         `json:"total"`
		Created  int64         `json:"created"`
		Updated  int64         `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if len(result.Failures) > 0 {
		return fmt.Errorf("reindex completed with %d failures", len(result.Failures))
	}

	return nil
}

// SwapAlias atomically removes aliasName from oldIndex and attaches it to newIndex.
func (m *ReindexManager) SwapAlias(ctx context.Context, aliasName, oldIndex, newIndex string) error {
	actions := []map[string]interface{}{}
	if oldIndex != "" {
		actions = append(actions, map[string]interface{}{
			"remove": map[string]interface{}{
				"index": oldIndex,
				"alias": aliasName,
			},
		})
	}
	actions = append(actions, map[string]interface{}{
		"add": map[string]interface{}{
			"index": newIndex,
			"alias": aliasName,
		},
	})

	payload := map[string]interface{}{
		"actions": actions,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/_aliases", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alias swap for %s (%s -> %s) failed (%d): %s", aliasName, oldIndex, newIndex, resp.StatusCode, string(body))
	}

	return nil
}

// RollbackAlias atomically reverts an alias pointing to failedIndex back to previousIndex.
func (m *ReindexManager) RollbackAlias(ctx context.Context, aliasName, failedIndex, previousIndex string) error {
	return m.SwapAlias(ctx, aliasName, failedIndex, previousIndex)
}

// GetAliasTarget finds the physical index currently targeted by an alias.
func (m *ReindexManager) GetAliasTarget(ctx context.Context, aliasName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/_alias/"+aliasName, nil)
	if err != nil {
		return "", err
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil // Alias does not exist
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get alias %s failed (%d): %s", aliasName, resp.StatusCode, string(body))
	}

	var targets map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", err
	}

	for idxName := range targets {
		return idxName, nil
	}
	return "", nil
}

