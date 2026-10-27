package ai

import (
	"testing"
	"time"
)

func TestExtractCitationIndices(t *testing.T) {
	text := "OpenPortal employs hybrid BM25 and neural search [1]. The architecture supports distributed sharding [2] and real-time outbox [1]."
	indices := ExtractCitationIndices(text)
	if len(indices) != 2 {
		t.Fatalf("expected 2 unique indices, got %d", len(indices))
	}
	if indices[0] != 1 || indices[1] != 2 {
		t.Errorf("unexpected indices: %+v", indices)
	}
}

func TestVerifyGrounding(t *testing.T) {
	now := time.Now()
	citations := []CitationAnchor{
		{
			Index:       1,
			DocumentID:  "doc-1",
			Title:       "OpenSearch Architecture Guide",
			URL:         "https://openportal.org/docs/arch",
			Publisher:   "OpenPortal Foundation",
			PublishedAt: now,
			Snippet:     "OpenPortal integrates BM25 lexical ranking and distributed sharding for high throughput search.",
		},
		{
			Index:       2,
			DocumentID:  "doc-2",
			Title:       "PostgreSQL Outbox Pattern",
			URL:         "https://openportal.org/docs/outbox",
			Publisher:   "OpenPortal Foundation",
			PublishedAt: now,
			Snippet:     "Transactional outbox pattern ensures reliable change data capture between PostgreSQL and OpenSearch.",
		},
	}

	// 1. Faithful text with citations
	faithfulText := "OpenPortal integrates BM25 lexical ranking for high throughput search [1]. Transactional outbox pattern ensures reliable change capture between PostgreSQL and OpenSearch [2]."
	repFaithful := VerifyGrounding(faithfulText, citations)
	if !repFaithful.IsFaithful {
		t.Errorf("expected faithful text to pass verification: score=%f", repFaithful.OverallFaithScore)
	}
	if repFaithful.OverallFaithScore < 0.80 {
		t.Errorf("expected faith score >= 0.80, got %f", repFaithful.OverallFaithScore)
	}

	// 2. Hallucinatory / uncited text
	hallucinationText := "OpenPortal was founded in 1995 and uses quantum computing processors for all database indexing tasks without any citation."
	repHallucination := VerifyGrounding(hallucinationText, citations)
	if repHallucination.IsFaithful {
		t.Errorf("expected hallucination to fail verification")
	}
	if repHallucination.OverallFaithScore != 0.0 {
		t.Errorf("expected faith score 0.0, got %f", repHallucination.OverallFaithScore)
	}
}
