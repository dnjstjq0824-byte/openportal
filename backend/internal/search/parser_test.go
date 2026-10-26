package search

import (
	"testing"
)

func TestParseQuery(t *testing.T) {
	raw := `검색엔진 "분산 아키텍처" -스팸 site:example.org lang:ko filetype:pdf`
	parsed := ParseQuery(raw)

	if len(parsed.PositiveTerms) != 1 || parsed.PositiveTerms[0] != "검색엔진" {
		t.Errorf("unexpected positive terms: %v", parsed.PositiveTerms)
	}

	if len(parsed.Phrases) != 1 || parsed.Phrases[0] != "분산 아키텍처" {
		t.Errorf("unexpected phrases: %v", parsed.Phrases)
	}

	if len(parsed.Negations) != 1 || parsed.Negations[0] != "스팸" {
		t.Errorf("unexpected negations: %v", parsed.Negations)
	}

	if parsed.SiteFilter != "example.org" {
		t.Errorf("unexpected site filter: %q", parsed.SiteFilter)
	}

	if parsed.LangFilter != "ko" {
		t.Errorf("unexpected lang filter: %q", parsed.LangFilter)
	}

	if parsed.FiletypeFilter != "pdf" {
		t.Errorf("unexpected filetype filter: %q", parsed.FiletypeFilter)
	}

	// Verify OpenSearch DSL construction
	dsl := parsed.BuildOpenSearchQuery("ko")
	boolQ, ok := dsl["bool"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected bool query in DSL: %v", dsl)
	}

	if _, ok := boolQ["must"]; !ok {
		t.Errorf("missing must in bool query")
	}
	if _, ok := boolQ["must_not"]; !ok {
		t.Errorf("missing must_not in bool query")
	}
	if _, ok := boolQ["filter"]; !ok {
		t.Errorf("missing filter in bool query")
	}
}

func TestPageTokenEncoding(t *testing.T) {
	token := EncodePageToken(3, 10)
	page, limit, err := DecodePageToken(token)
	if err != nil {
		t.Fatalf("DecodePageToken failed: %v", err)
	}
	if page != 3 || limit != 10 {
		t.Errorf("expected page 3, limit 10; got page %d, limit %d", page, limit)
	}

	_, _, err = DecodePageToken("invalid-token")
	if err == nil {
		t.Errorf("expected error decoding invalid token")
	}
}

