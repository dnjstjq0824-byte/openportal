//go:build live

package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
)

type goldenQuery struct {
	Query     string         `json:"query"`
	Language  string         `json:"language"`
	Note      string         `json:"note,omitempty"`
	Judgments map[string]int `json:"judgments"`
}

// TestLiveGoldenEval runs the golden query set (testdata/golden_queries.json)
// against a scratch Nori (documents_v2) index built from
// testdata/golden_corpus.json, then scores the product retrieval path
// (SearchHybrid) with NDCG@10 and MAP@10. Requires TEST_OS_URL.
//
// Run: go test -tags live -run TestLiveGoldenEval ./internal/search/
func TestLiveGoldenEval(t *testing.T) {
	base := strings.TrimRight(os.Getenv("TEST_OS_URL"), "/")
	if base == "" {
		t.Skip("TEST_OS_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	// Generous timeout: the first Nori analysis loads the dictionary.
	httpClient := &http.Client{Timeout: 120 * time.Second}
	call := func(method, path string, payload []byte, ctype string) []byte {
		t.Helper()
		if ctype == "" {
			ctype = "application/json"
		}
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, body)
		if err != nil {
			t.Fatalf("request build failed: %v", err)
		}
		req.Header.Set("Content-Type", ctype)
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s failed: %v", method, path, err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode >= 300 {
			t.Fatalf("%s %s -> %d: %s", method, path, resp.StatusCode, data)
		}
		return data
	}

	corpusRaw, err := os.ReadFile("testdata/golden_corpus.json")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var corpus []map[string]interface{}
	if err := json.Unmarshal(corpusRaw, &corpus); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}
	queriesRaw, err := os.ReadFile("testdata/golden_queries.json")
	if err != nil {
		t.Fatalf("read queries: %v", err)
	}
	var queries []goldenQuery
	if err := json.Unmarshal(queriesRaw, &queries); err != nil {
		t.Fatalf("parse queries: %v", err)
	}
	for _, q := range queries {
		if len(q.Judgments) == 0 {
			t.Fatalf("query %q has no judgments", q.Query)
		}
	}

	mapping, err := os.ReadFile(findMapping(t, "documents_v2.json"))
	if err != nil {
		t.Fatalf("read v2 mapping: %v", err)
	}
	idx := fmt.Sprintf("documents_golden_%d", time.Now().UnixNano()%100000)
	call("PUT", "/"+idx, mapping, "")
	defer call("DELETE", "/"+idx, nil, "")

	var bulk strings.Builder
	for _, d := range corpus {
		meta, _ := json.Marshal(map[string]interface{}{
			"index": map[string]interface{}{"_index": idx, "_id": d["document_id"]},
		})
		raw, _ := json.Marshal(d)
		bulk.Write(meta)
		bulk.WriteByte('\n')
		bulk.Write(raw)
		bulk.WriteByte('\n')
	}
	bulkOut := call("POST", "/_bulk?refresh=true", []byte(bulk.String()), "application/x-ndjson")
	if strings.Contains(string(bulkOut), `"errors":true`) {
		t.Fatalf("bulk failed: %s", bulkOut)
	}

	c := New(base, idx, "", "")
	var ndcgs, aps []float64
	for _, q := range queries {
		res, err := c.SearchHybrid(ctx, q.Query, q.Language, 1, 10, "")
		if err != nil {
			t.Fatalf("query %q failed: %v", q.Query, err)
		}
		grades := make([]int, 0, len(res.Hits))
		for _, h := range res.Hits {
			grades = append(grades, q.Judgments[h.DocumentID])
		}
		totalRel := 0
		for _, g := range q.Judgments {
			if g >= 2 {
				totalRel++
			}
		}
		ndcg := ranking.ComputeNDCG(grades, 10)
		ap := ranking.ComputeAP(grades, totalRel, 10)
		ndcgs = append(ndcgs, ndcg)
		aps = append(aps, ap)
		top := ""
		if len(res.Hits) > 0 {
			top = res.Hits[0].DocumentID
		}
		t.Logf("q=%-28q lang=%s total=%2d top=%-4s grades=%v ndcg=%.4f ap=%.4f",
			q.Query, q.Language, res.Total, top, grades, ndcg, ap)
		if ndcg < 0.50 {
			t.Errorf("query %q NDCG@10 too low: %.4f (grades %v)", q.Query, ndcg, grades)
		}
	}

	mean := func(xs []float64) float64 {
		var s float64
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}
	meanNDCG, meanAP := mean(ndcgs), ranking.ComputeMAP(aps)
	t.Logf("GOLDEN: %d queries, mean NDCG@10=%.4f MAP@10=%.4f", len(queries), meanNDCG, meanAP)
	if meanNDCG < 0.90 {
		t.Errorf("mean NDCG@10 too low: %.4f, want >= 0.90", meanNDCG)
	}
	if meanAP < 0.90 {
		t.Errorf("MAP@10 too low: %.4f, want >= 0.90", meanAP)
	}
}
