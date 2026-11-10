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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLiveC4C6OpenSearch proves C-4 (completion/phrase suggesters) and C-6
// (hybrid recall path) against a real OpenSearch. It builds a scratch index
// from the committed v1 mapping, bulks 3 fixtures, runs the client, and
// deletes the index. Requires TEST_OS_URL (e.g. http://127.0.0.1:9200).
//
// Run: go test -tags live -run TestLiveC4C6OpenSearch ./internal/search/
func TestLiveC4C6OpenSearch(t *testing.T) {
	base := strings.TrimRight(os.Getenv("TEST_OS_URL"), "/")
	if base == "" {
		t.Skip("TEST_OS_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	httpClient := &http.Client{Timeout: 10 * time.Second}
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

	// The v2 Nori mapping must at least parse on the real cluster.
	v2raw, err := os.ReadFile(findMapping(t, "documents_v2.json"))
	if err != nil {
		t.Fatalf("read v2 mapping: %v", err)
	}
	v2idx := fmt.Sprintf("documents_c4verify_v2_%d", time.Now().UnixNano()%100000)
	call("PUT", "/"+v2idx, v2raw, "")
	defer call("DELETE", "/"+v2idx, nil, "")

	// Scratch v1 index with the committed mapping (completion field included).
	mapping, err := os.ReadFile(findMapping(t, "documents_v1.json"))
	if err != nil {
		t.Fatalf("read v1 mapping: %v", err)
	}
	idx := fmt.Sprintf("documents_c4verify_%d", time.Now().UnixNano()%100000)
	call("PUT", "/"+idx, mapping, "")
	defer call("DELETE", "/"+idx, nil, "")

	docs := []map[string]interface{}{
		{"document_id": "live-001", "url": "https://example.org/intro",
			"title": "오픈포털 검색엔진 프로젝트", "description": "독립 검색엔진 아키텍처 개발을 위한 예시 문서",
			"body": "검색 색인 크롤러 역색인 랭킹 API Go 한국어", "language": "ko",
			"indexable": true, "safety": "clean", "adult_score": 0.0, "malware_score": 0.0,
			"suggest": map[string]interface{}{"input": []string{"오픈포털 검색엔진 프로젝트"}}},
		{"document_id": "live-002", "url": "https://example.org/search",
			"title": "검색엔진 인덱싱과 랭킹", "description": "검색 색인과 품질 평가에 대한 테스트 문서",
			"body": "역색인 랭킹 스코어링 형태소 분석 검색 결과", "language": "ko",
			"indexable": true, "safety": "clean", "adult_score": 0.0, "malware_score": 0.0,
			"suggest": map[string]interface{}{"input": []string{"검색엔진 인덱싱과 랭킹"}}},
		{"document_id": "live-003", "url": "https://example.org/docs",
			"title":       "OpenPortal developer documentation",
			"description": "Example English document in the demo dataset",
			"body":        "OpenPortal full text search engine API integration", "language": "en",
			"indexable": true, "safety": "clean", "adult_score": 0.0, "malware_score": 0.0,
			"suggest": map[string]interface{}{"input": []string{"OpenPortal developer documentation"}}},
	}
	var bulk strings.Builder
	for _, d := range docs {
		meta, _ := json.Marshal(map[string]interface{}{"index": map[string]interface{}{"_index": idx, "_id": d["document_id"]}})
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

	items, err := c.Suggest(ctx, "검색")
	if err != nil {
		t.Fatalf("live Suggest failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("live Suggest(검색) returned no completion candidates")
	}
	t.Logf("Suggest(검색) = %q", items)

	corr, err := c.SuggestCorrection(ctx, "검섹엔진")
	if err != nil {
		t.Fatalf("live SuggestCorrection failed: %v", err)
	}
	if corr != "검색엔진" {
		t.Fatalf("live phrase correction = %q, want 검색엔진", corr)
	}

	corr, err = c.SuggestCorrection(ctx, "serach engine")
	if err != nil {
		t.Fatalf("live SuggestCorrection(en) failed: %v", err)
	}
	if corr != "search engine" {
		t.Fatalf("live body-phrase correction = %q, want 'search engine'", corr)
	}

	corr, err = c.SuggestCorrection(ctx, "rjatordpswls")
	if err != nil {
		t.Fatalf("live layout rescue failed: %v", err)
	}
	if corr != "검색엔진" {
		t.Fatalf("live layout rescue = %q, want 검색엔진", corr)
	}

	res, err := c.SearchHybrid(ctx, "검색엔징", "", 1, 10, "")
	if err != nil {
		t.Fatalf("live SearchHybrid failed: %v", err)
	}
	if res.Total == 0 || len(res.Hits) == 0 {
		t.Fatalf("live hybrid fuzzy query matched nothing: %+v", res)
	}
	t.Logf("SearchHybrid(검색엔징) total=%d hits=%d top=%s score=%v",
		res.Total, len(res.Hits), res.Hits[0].DocumentID, res.Hits[0].Score)
}

func findMapping(t *testing.T, name string) string {
	t.Helper()
	for _, p := range []string{
		filepath.Join("..", "..", "..", "infra", "opensearch", name),
		filepath.Join("..", "..", "infra", "opensearch", name),
		filepath.Join("infra", "opensearch", name),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatalf("mapping %s not found", name)
	return ""
}
