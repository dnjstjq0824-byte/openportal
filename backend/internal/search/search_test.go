package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchRequestAndMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/documents_v1/_search" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_score":2.5,"_source":{"title":"한글 테스트","url":"https://example.org","description":"예시","language":"ko"}}]}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "documents_v1", "", "")
	result, err := c.Search(context.Background(), "한글", "ko", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Hits) != 1 || result.Hits[0].Title != "한글 테스트" {
		t.Fatalf("bad mapping: %+v", result)
	}
}
