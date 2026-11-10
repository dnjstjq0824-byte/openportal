package search

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func hybridStub(t *testing.T, bodies *[]string, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		*bodies = append(*bodies, string(raw))
		mu.Unlock()
		if strings.Contains(string(raw), `"fuzziness"`) {
			_, _ = w.Write([]byte(`{"hits":{"total":{"value":3},"hits":[
				{"_id":"doc-b","_score":1.1,"_source":{"document_id":"doc-b","title":"B","url":"https://example.org/b","description":"shared","language":"en"}},
				{"_id":"doc-c","_score":1.0,"_source":{"document_id":"doc-c","title":"C","url":"https://example.org/c","description":"recall only","language":"en"}}
			]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":5},"hits":[
			{"_id":"doc-a","_score":2.0,"_source":{"document_id":"doc-a","title":"A","url":"https://example.org/a","description":"keyword only","language":"en"}},
			{"_id":"doc-b","_score":1.9,"_source":{"document_id":"doc-b","title":"B","url":"https://example.org/b","description":"shared","language":"en"}}
		]}}`))
	}))
}

func TestSearchHybridFusesTwoPaths(t *testing.T) {
	var bodies []string
	var mu sync.Mutex
	upstream := hybridStub(t, &bodies, &mu)
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")

	res, err := c.SearchHybrid(context.Background(), "test query", "", 1, 10, "")
	if err != nil {
		t.Fatalf("SearchHybrid failed: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 backend queries, got %d", len(bodies))
	}
	if bodies[0] == bodies[1] {
		t.Fatal("keyword and recall queries must differ")
	}
	fuzzy := 0
	for _, b := range bodies {
		if strings.Contains(b, `"fuzziness":"AUTO"`) {
			fuzzy++
		}
	}
	if fuzzy != 1 {
		t.Fatalf("expected exactly 1 fuzzy recall query, got bodies: %v", bodies)
	}
	// doc-b ranks in both lists → first; doc-c is recall-only but surfaces.
	if len(res.Hits) != 3 {
		t.Fatalf("expected 3 fused hits, got %d", len(res.Hits))
	}
	if res.Hits[0].DocumentID != "doc-b" || res.Hits[1].DocumentID != "doc-a" || res.Hits[2].DocumentID != "doc-c" {
		t.Fatalf("wrong fusion order: %v", res.Hits)
	}
	wantB := 1.0/62 + 1.0/61 // rank 2 keyword + rank 1 recall, k=60
	if math.Abs(res.Hits[0].Score-wantB) > 1e-9 {
		t.Fatalf("doc-b score %v, want RRF %v", res.Hits[0].Score, wantB)
	}
	if res.Total != 5 {
		t.Fatalf("total must be the max over paths (keyword 5, recall 3), got %d", res.Total)
	}
}

func TestSearchHybridSafetyOnBothPaths(t *testing.T) {
	var bodies []string
	var mu sync.Mutex
	upstream := hybridStub(t, &bodies, &mu)
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")

	if _, err := c.SearchHybrid(context.Background(), "test", "", 1, 10, "strict"); err != nil {
		t.Fatalf("SearchHybrid failed: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 backend queries, got %d", len(bodies))
	}
	for i, b := range bodies {
		if !strings.Contains(b, `"safety":["adult","gambling","malware"]`) {
			t.Fatalf("path %d missing strict safety filter: %s", i, b)
		}
		if !strings.Contains(b, `"is_tombstone"`) {
			t.Fatalf("path %d missing tombstone exclusion: %s", i, b)
		}
	}
}

func TestSearchHybridDegradedAndFailed(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), `"fuzziness"`) {
			w.WriteHeader(500)
			return
		}
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[
			{"_id":"doc-a","_score":2.0,"_source":{"document_id":"doc-a","title":"A","url":"https://example.org/a","description":"","language":"en"}}
		]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")

	res, err := c.SearchHybrid(context.Background(), "test", "", 1, 10, "")
	if err != nil {
		t.Fatalf("one failing path must degrade, got error: %v", err)
	}
	if len(res.Hits) != 1 || res.Hits[0].DocumentID != "doc-a" {
		t.Fatalf("degraded result wrong: %+v", res)
	}

	upstream.Close()
	if _, err := c.SearchHybrid(context.Background(), "test", "", 1, 10, ""); err == nil {
		t.Fatal("both paths failing must fail the search")
	}
}

func TestSearchHybridPagination(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":30},"hits":[
			{"_id":"doc-1","_score":3.0,"_source":{"document_id":"doc-1","title":"one","url":"https://example.org/1","description":"","language":"en"}},
			{"_id":"doc-2","_score":2.0,"_source":{"document_id":"doc-2","title":"two","url":"https://example.org/2","description":"","language":"en"}},
			{"_id":"doc-3","_score":1.0,"_source":{"document_id":"doc-3","title":"three","url":"https://example.org/3","description":"","language":"en"}}
		]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")

	res, err := c.SearchHybrid(context.Background(), "test", "", 2, 2, "")
	if err != nil {
		t.Fatalf("SearchHybrid failed: %v", err)
	}
	if len(res.Hits) != 1 || res.Hits[0].DocumentID != "doc-3" {
		t.Fatalf("page 2 wrong: %+v", res.Hits)
	}
	if res.Total != 30 || res.NextPageToken == "" {
		t.Fatalf("total/token wrong: %+v", res)
	}
}

func TestSearchKNN(t *testing.T) {
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[
			{"_id":"doc-v","_score":0.9,"_source":{"document_id":"doc-v","title":"V","url":"https://example.org/v","description":"","language":"en"}}
		]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")

	hits, err := c.SearchKNN(context.Background(), []float32{0.1, 0.2}, 5)
	if err != nil {
		t.Fatalf("SearchKNN failed: %v", err)
	}
	if len(hits) != 1 || hits[0].DocumentID != "doc-v" {
		t.Fatalf("knn hits wrong: %+v", hits)
	}
	if !strings.Contains(gotBody, `"knn"`) || !strings.Contains(gotBody, `"embedding"`) {
		t.Fatalf("not a knn query: %s", gotBody)
	}
	if _, err := c.SearchKNN(context.Background(), nil, 5); err == nil {
		t.Fatal("empty vector must be an explicit not-configured error")
	} else if !strings.Contains(err.Error(), "no embedding model configured") {
		t.Fatalf("wrong knn error: %v", err)
	}
}
