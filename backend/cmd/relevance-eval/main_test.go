package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeJudgments(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "judgments.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureServer() *httptest.Server {
	rankings := map[string][]string{
		"alpha": {
			"https://example.org/u2",
			"https://example.org/u9",
			"https://example.org/u1",
			"https://example.org/u-extra",
		},
		"beta": {
			"https://example.org/v9",
			"https://example.org/v1",
		},
		"gamma": {},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls := rankings[r.URL.Query().Get("q")]
		hits := make([]map[string]string, 0, len(urls))
		for _, u := range urls {
			hits = append(hits, map[string]string{"url": u})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": hits})
	}))
}

const fixtureJudgments = `{"version":1,"k":3,"queries":[
  {"query":"alpha","judgments":[
    {"url":"https://example.org/u1","grade":3},
    {"url":"https://example.org/u2","grade":2},
    {"url":"https://example.org/u3","grade":0}]},
  {"query":"beta","judgments":[
    {"url":"https://example.org/v1","grade":3}]},
  {"query":"gamma","judgments":[]}
]}`

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) < 1e-4
}

func TestEvaluateScoresKnownRanking(t *testing.T) {
	server := fixtureServer()
	defer server.Close()
	path := writeJudgments(t, fixtureJudgments)
	set, err := loadSet(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := evaluate(server.Client(), server.URL, "/api/v1/search", set, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Queries != 3 || result.K != 3 {
		t.Fatalf("unexpected summary header: %+v", result)
	}
	// alpha: relevance [2,0,3] -> DCG 6.5, IDCG 7+3/log2(3); first=1; AP (1+2/3)/2.
	alpha := result.PerQuery[0]
	if !closeEnough(alpha.NDCG, 6.5/(7+3/math.Log2(3))) {
		t.Fatalf("alpha NDCG = %v", alpha.NDCG)
	}
	if !closeEnough(alpha.AP, (1+2.0/3.0)/2) {
		t.Fatalf("alpha AP = %v", alpha.AP)
	}
	if alpha.FirstRelevant != 1 || alpha.Unjudged != 1 {
		t.Fatalf("alpha ranks = %+v", alpha)
	}
	// beta: relevance [0,3] -> DCG 7/log2(3), IDCG 7; first=2; AP 0.5.
	beta := result.PerQuery[1]
	if !closeEnough(beta.NDCG, (7/math.Log2(3))/7) {
		t.Fatalf("beta NDCG = %v", beta.NDCG)
	}
	if !closeEnough(beta.AP, 0.5) {
		t.Fatalf("beta AP = %v", beta.AP)
	}
	if beta.FirstRelevant != 2 {
		t.Fatalf("beta ranks = %+v", beta)
	}
	// gamma: zero hits.
	if result.PerQuery[2].Hits != 0 {
		t.Fatalf("gamma hits = %+v", result.PerQuery[2])
	}
	if !closeEnough(result.ZeroRate, 1.0/3.0) {
		t.Fatalf("zero rate = %v", result.ZeroRate)
	}
	if !closeEnough(result.MRR, (1+0.5+0)/3) {
		t.Fatalf("MRR = %v", result.MRR)
	}
	if !closeEnough(result.MAP, (((1+2.0/3.0)/2)+0.5+0)/3) {
		t.Fatalf("MAP = %v", result.MAP)
	}
	// 2 unjudged slots (u9, v9) out of 5 scored slots.
	if !closeEnough(result.UnjudgedRate, 2.0/5.0) {
		t.Fatalf("unjudged rate = %v", result.UnjudgedRate)
	}
}

func TestEvaluateFailsClosedOnUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	path := writeJudgments(t, fixtureJudgments)
	set, err := loadSet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evaluate(server.Client(), server.URL, "/api/v1/search", set, 3); err == nil {
		t.Fatal("expected evaluation error on HTTP 503")
	}
}

func TestLoadSetRejectsBadFiles(t *testing.T) {
	for _, content := range []string{
		`not json`,
		`{"version":2,"k":3,"queries":[{"query":"q","judgments":[]}]}`,
		`{"version":1,"k":3,"queries":[]}`,
		`{"version":1,"k":3,"queries":[{"query":"","judgments":[]}]}`,
		`{"version":1,"k":3,"queries":[{"query":"q","judgments":[{"url":"https://example.org/","grade":4}]}]}`,
	} {
		if _, err := loadSet(writeJudgments(t, content)); err == nil {
			t.Fatalf("expected load error for %q", content)
		}
	}
}

func TestRunExitCodes(t *testing.T) {
	server := fixtureServer()
	defer server.Close()
	code, _ := run(writeJudgments(t, fixtureJudgments), server.URL, "/api/v1/search", 3, 5*time.Second)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	code, _ = run(filepath.Join(t.TempDir(), "missing.json"), server.URL, "/api/v1/search", 3, 5*time.Second)
	if code != 2 {
		t.Fatalf("expected exit 2 for missing file, got %d", code)
	}
}

func TestCanonicalURL(t *testing.T) {
	if got := canonicalURL("https://EXAMPLE.org/Path/?b=2&a=1"); got != "https://example.org/Path?b=2&a=1" {
		t.Fatalf("canonical = %q", got)
	}
	if got := canonicalURL("not a url"); got != "not a url" {
		t.Fatalf("fallback = %q", got)
	}
}
