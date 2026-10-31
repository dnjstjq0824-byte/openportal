package indexer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type openSearchSchema struct {
	Settings struct {
		Analysis struct {
			Tokenizer map[string]interface{} `json:"tokenizer"`
			Filter    map[string]interface{} `json:"filter"`
			Analyzer  map[string]interface{} `json:"analyzer"`
		} `json:"analysis"`
	} `json:"settings"`
	Mappings struct {
		Dynamic    string                 `json:"dynamic"`
		Properties map[string]interface{} `json:"properties"`
	} `json:"mappings"`
}

func findSchemaPath() string {
	candidates := []string{
		filepath.Join("..", "..", "..", "infra", "opensearch", "documents_v2.json"),
		filepath.Join("..", "..", "infra", "opensearch", "documents_v2.json"),
		filepath.Join("infra", "opensearch", "documents_v2.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("..", "..", "..", "infra", "opensearch", "documents_v2.json")
}

func TestDocumentsV2SchemaAndAnalyzerConformance(t *testing.T) {
	schemaPath := findSchemaPath()
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read documents_v2.json at %s: %v", schemaPath, err)
	}

	var schema openSearchSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("documents_v2.json is not valid JSON: %v", err)
	}

	// SE-07: Verify Nori tokenizer & analyzers
	if _, ok := schema.Settings.Analysis.Tokenizer["nori_user_dict"]; !ok {
		t.Errorf("expected tokenizer 'nori_user_dict' in documents_v2.json")
	}
	if _, ok := schema.Settings.Analysis.Filter["nori_posfilter"]; !ok {
		t.Errorf("expected filter 'nori_posfilter' in documents_v2.json")
	}
	if _, ok := schema.Settings.Analysis.Analyzer["korean_analyzer"]; !ok {
		t.Errorf("expected analyzer 'korean_analyzer' in documents_v2.json")
	}

	// SE-08: Required properties validation
	requiredFields := []string{
		"document_id",
		"url",
		"canonical_url",
		"host",
		"title",
		"description",
		"body",
		"headings",
		"anchor_texts",
		"language",
		"simhash",
		"indexable",
		"is_representative",
		"is_tombstone",
		"tombstone_reason",
		"domain_authority",
		"spam_score",
		"source",
		"term_frequency_density",
		"published_at",
		"last_crawled_at",
	}

	for _, field := range requiredFields {
		if _, ok := schema.Mappings.Properties[field]; !ok {
			t.Errorf("missing required field %q in documents_v2.json mappings", field)
		}
	}

	// Verify pipeline docPayload fields against strict schema
	svc := NewIndexingService("", "", nil)
	html := []byte(`<!DOCTYPE html><html><head><title>Test Doc</title><link rel="canonical" href="https://test.openportal.org/doc-1"><meta name="description" content="Test description"></head><body><h1>Heading</h1><p>Main body content with sufficient length for testing schema mapping.</p></body></html>`)
	_, indexed, err := svc.IngestCrawlResult(context.Background(), "doc-test-v2", "https://test.openportal.org/doc-1", html, time.Now().UTC())
	if err != nil || !indexed {
		t.Fatalf("IngestCrawlResult failed: %v", err)
	}

	events, err := svc.Outbox().LeaseBatch(context.Background(), 1)
	if err != nil || len(events) == 0 {
		t.Fatalf("no outbox event enqueued: %v", err)
	}
	payload := events[0].Payload

	// Verify all payload fields are declared in schema so dynamic:strict will not reject
	for key := range payload {
		if _, ok := schema.Mappings.Properties[key]; !ok {
			t.Errorf("payload contains field %q which is NOT in documents_v2.json properties (would fail strict mapping)", key)
		}
	}

	// Verify specific values
	if payload["canonical_url"] != "https://test.openportal.org/doc-1" {
		t.Errorf("canonical_url: want 'https://test.openportal.org/doc-1', got %v", payload["canonical_url"])
	}
	if payload["host"] != "test.openportal.org" {
		t.Errorf("host: want 'test.openportal.org', got %v", payload["host"])
	}
	if payload["source"] != "crawler" {
		t.Errorf("source: want 'crawler', got %v", payload["source"])
	}
	if payload["spam_score"] == nil {
		t.Errorf("spam_score is nil")
	}
}
