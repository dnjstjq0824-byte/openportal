package search

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuggestCompletionPath(t *testing.T) {
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(`{"suggest":{"title-complete":[{"text":"검","options":[
			{"text":"검색엔진 인덱싱과 랭킹"},{"text":"검색엔진 인덱싱과 랭킹"},{"text":"검색엔진"}
		]}]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")
	items, err := c.Suggest(context.Background(), "검")
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if !strings.Contains(gotBody, `"completion"`) || !strings.Contains(gotBody, `"field":"suggest"`) {
		t.Fatalf("expected completion suggester query, got: %s", gotBody)
	}
	if len(items) != 2 || items[0] != "검색엔진 인덱싱과 랭킹" || items[1] != "검색엔진" {
		t.Fatalf("dedup/parse failed: %q", items)
	}
}

func TestSuggestPrefixFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Old index: completion field missing → error; prefix query works.
		if strings.Contains(string(body), `"completion"`) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"no mapping found for field [suggest]"}`))
			return
		}
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[
			{"_id":"d1","_score":1.0,"_source":{"document_id":"d1","title":"OpenPortal docs","url":"https://example.org/d","description":"","language":"en"}}
		]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")
	items, err := c.Suggest(context.Background(), "Open")
	if err != nil {
		t.Fatalf("fallback Suggest failed: %v", err)
	}
	if len(items) != 1 || items[0] != "OpenPortal docs" {
		t.Fatalf("fallback items wrong: %q", items)
	}
}

func TestSuggestCorrectionLayoutRescue(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"suggest"`) {
			_, _ = w.Write([]byte(`{"suggest":{"correction-title":[{"text":"x","options":[]}],"correction-body":[{"text":"x","options":[]}]}}`))
			return
		}
		// Count path: only the converted query matches.
		total := 0
		if strings.Contains(string(body), "검색엔진") {
			total = 2
		}
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":` + string(rune('0'+total)) + `},"hits":[]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")
	corr, err := c.SuggestCorrection(context.Background(), "rjatordpswls")
	if err != nil {
		t.Fatalf("SuggestCorrection failed: %v", err)
	}
	if corr != "검색엔진" {
		t.Fatalf("layout rescue failed: %q", corr)
	}
}

func TestSuggestCorrectionOriginalHitsNoRescue(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"suggest"`) {
			_, _ = w.Write([]byte(`{"suggest":{"correction-title":[{"text":"x","options":[]}],"correction-body":[{"text":"x","options":[]}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":3},"hits":[]}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")
	// "dkssud" converts to 안녕, but the original already matches → no rescue,
	// and the phrase suggester has no options → "".
	corr, err := c.SuggestCorrection(context.Background(), "dkssud")
	if err != nil {
		t.Fatalf("SuggestCorrection failed: %v", err)
	}
	if corr != "" {
		t.Fatalf("expected no correction, got %q", corr)
	}
	if calls != 2 { // 1 count + 1 phrase, no second count for converted
		t.Fatalf("expected 2 backend calls, got %d", calls)
	}
}

func TestSuggestCorrectionPhrasePath(t *testing.T) {
	var gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(`{"suggest":{
			"correction-title":[{"text":"검섹엔진","options":[{"text":"검색엔진","score":0.9}]}],
			"correction-body":[{"text":"검섹엔진","options":[]}]
		}}`))
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")
	// Pure-Hangul typo: layout conversion is a no-op, phrase path decides.
	corr, err := c.SuggestCorrection(context.Background(), "검섹엔진")
	if err != nil {
		t.Fatalf("SuggestCorrection failed: %v", err)
	}
	if corr != "검색엔진" {
		t.Fatalf("phrase correction failed: %q", corr)
	}
	if !strings.Contains(gotBody, `"phrase"`) || !strings.Contains(gotBody, `"suggest_mode":"missing"`) {
		t.Fatalf("expected phrase suggester with missing mode, got: %s", gotBody)
	}
}

func TestSuggestCorrectionErrorPropagates(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer upstream.Close()
	c := New(upstream.URL, "documents_v1", "", "")
	if _, err := c.Suggest(context.Background(), "x"); err == nil {
		t.Fatal("expected Suggest error on 500 backend")
	}
	if _, err := c.SuggestCorrection(context.Background(), "검섹엔진"); err == nil {
		t.Fatal("expected SuggestCorrection error on 500 backend")
	}
}
