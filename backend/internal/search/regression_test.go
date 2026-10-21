package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIncompleteSearchFailsClosed(t *testing.T) {
	for _, body := range []string{
		`{"timed_out":true,"hits":{"total":{"value":0},"hits":[]}}`,
		`{"_shards":{"failed":1},"hits":{"total":{"value":0},"hits":[]}}`,
		`{}`,
		`{"hits":{"total":{"value":0},"hits":[]}} trailing`,
	} {
		t.Run(body, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer s.Close()
			c := New(s.URL, "documents_v1", "", "")
			if _, err := c.Search(context.Background(), "test", "", 1, 10); err == nil {
				t.Fatal("incomplete backend response must not be a successful empty search")
			}
			if _, err := c.Suggest(context.Background(), "test"); err == nil {
				t.Fatal("incomplete backend response must not be successful suggestions")
			}
		})
	}
}
