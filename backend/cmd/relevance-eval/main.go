// Command relevance-eval runs a graded judgment set against a live search
// endpoint and reports NDCG@k, MRR, MAP and zero-result rate (Phase 3, §6).
//
// Judgment file schema:
//
//	{"version":1,"k":10,"queries":[
//	  {"query":"...","judgments":[{"url":"https://...","grade":3}]}
//	]}
//
// Grades follow the 0-3 relevance rubric (QUALITY_EVALUATION_GUIDE_KO §2).
// Retrieved URLs are canonicalized before matching; unjudged URLs score 0
// and are counted separately (unjudged_rate) so coverage gaps stay visible.
//
// Exit codes: 0 = evaluation complete (scores printed as JSON), 1 = any query
// failed (fail-closed, never a partial PASS), 2 = bad arguments or bad file.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
)

type judgment struct {
	URL   string `json:"url"`
	Grade int    `json:"grade"`
}

type gradedQuery struct {
	Query     string     `json:"query"`
	Judgments []judgment `json:"judgments"`
}

type judgmentSet struct {
	Version int           `json:"version"`
	K       int           `json:"k"`
	Queries []gradedQuery `json:"queries"`
}

type searchHit struct {
	URL string `json:"url"`
}

type searchResponse struct {
	Hits []searchHit `json:"hits"`
}

type queryScore struct {
	Query         string  `json:"query"`
	NDCG          float64 `json:"ndcg"`
	AP            float64 `json:"ap"`
	FirstRelevant int     `json:"first_relevant_rank"`
	Hits          int     `json:"hits"`
	Unjudged      int     `json:"unjudged"`
}

type summary struct {
	Queries      int          `json:"queries"`
	K            int          `json:"k"`
	MeanNDCG     float64      `json:"mean_ndcg"`
	MRR          float64      `json:"mrr"`
	MAP          float64      `json:"map"`
	ZeroRate     float64      `json:"zero_rate"`
	UnjudgedRate float64      `json:"unjudged_rate"`
	PerQuery     []queryScore `json:"per_query"`
}

// canonicalURL mirrors the gateway normalizer: scheme + lowercase host +
// optional port + path without trailing slash + raw query.
func canonicalURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	host := strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" {
		host += ":" + port
	}
	path := strings.TrimSuffix(parsed.EscapedPath(), "/")
	out := parsed.Scheme + "://" + host + path
	if parsed.RawQuery != "" {
		out += "?" + parsed.RawQuery
	}
	return out
}

func loadSet(path string) (judgmentSet, error) {
	var set judgmentSet
	data, err := os.ReadFile(path)
	if err != nil {
		return set, err
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return set, fmt.Errorf("invalid judgment file: %w", err)
	}
	if set.Version != 1 {
		return set, fmt.Errorf("unsupported judgment version %d (want 1)", set.Version)
	}
	if len(set.Queries) == 0 {
		return set, fmt.Errorf("judgment set has no queries")
	}
	for i, q := range set.Queries {
		if strings.TrimSpace(q.Query) == "" {
			return set, fmt.Errorf("query %d is empty", i)
		}
		for j, judgment := range q.Judgments {
			if judgment.Grade < 0 || judgment.Grade > 3 {
				return set, fmt.Errorf("query %d judgment %d has invalid grade %d (want 0..3)", i, j, judgment.Grade)
			}
		}
	}
	return set, nil
}

func fetchHits(client *http.Client, baseURL, endpoint, query string) ([]searchHit, error) {
	target, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return nil, err
	}
	params := target.Query()
	params.Set("q", query)
	params.Set("page", "1")
	target.RawQuery = params.Encode()
	response, err := client.Get(target.String())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned HTTP %d for query %q", response.StatusCode, query)
	}
	var decoded searchResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("invalid search response for query %q: %w", query, err)
	}
	return decoded.Hits, nil
}

func evaluate(client *http.Client, baseURL, endpoint string, set judgmentSet, k int) (summary, error) {
	result := summary{Queries: len(set.Queries), K: k, PerQuery: make([]queryScore, 0, len(set.Queries))}
	firstRanks := make([]int, 0, len(set.Queries))
	averagePrecisions := make([]float64, 0, len(set.Queries))
	hitCounts := make([]int64, 0, len(set.Queries))
	var ndcgSum float64
	var judgedSlots, unjudgedSlots int
	for _, graded := range set.Queries {
		hits, err := fetchHits(client, baseURL, endpoint, graded.Query)
		if err != nil {
			return result, err
		}
		grades := make(map[string]int, len(graded.Judgments))
		relevant := 0
		for _, judgment := range graded.Judgments {
			grades[canonicalURL(judgment.URL)] = judgment.Grade
			if judgment.Grade >= 2 {
				relevant++
			}
		}
		limit := k
		if len(hits) < limit {
			limit = len(hits)
		}
		relevance := make([]int, 0, limit)
		first := 0
		unjudged := 0
		for i := 0; i < limit; i++ {
			grade, judged := grades[canonicalURL(hits[i].URL)]
			if !judged {
				unjudged++
				unjudgedSlots++
			}
			judgedSlots++
			relevance = append(relevance, grade)
			if first == 0 && grade >= 2 {
				first = i + 1
			}
		}
		ndcg := ranking.ComputeNDCG(relevance, k)
		ap := ranking.ComputeAP(relevance, relevant, k)
		ndcgSum += ndcg
		firstRanks = append(firstRanks, first)
		averagePrecisions = append(averagePrecisions, ap)
		hitCounts = append(hitCounts, int64(len(hits)))
		result.PerQuery = append(result.PerQuery, queryScore{
			Query: graded.Query, NDCG: ndcg, AP: ap,
			FirstRelevant: first, Hits: len(hits), Unjudged: unjudged,
		})
	}
	result.MeanNDCG = ndcgSum / float64(len(set.Queries))
	result.MRR = ranking.ComputeMRR(firstRanks)
	result.MAP = ranking.ComputeMAP(averagePrecisions)
	result.ZeroRate = ranking.ComputeZeroResultsRate(hitCounts)
	if judgedSlots > 0 {
		result.UnjudgedRate = float64(unjudgedSlots) / float64(judgedSlots)
	}
	return result, nil
}

func run(judgmentPath, baseURL, endpoint string, k int, timeout time.Duration) (int, string) {
	set, err := loadSet(judgmentPath)
	if err != nil {
		return 2, fmt.Sprintf("judgment file error: %v", err)
	}
	if k <= 0 {
		k = set.K
	}
	if k <= 0 {
		k = 10
	}
	client := &http.Client{Timeout: timeout}
	result, err := evaluate(client, baseURL, endpoint, set, k)
	if err != nil {
		return 1, fmt.Sprintf("evaluation failed: %v", err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return 1, fmt.Sprintf("summary encoding failed: %v", err)
	}
	return 0, string(encoded)
}

func main() {
	judgmentPath := flag.String("judgments", "", "path to judgment JSON file (required)")
	baseURL := flag.String("base-url", "http://127.0.0.1:8080", "search service base URL")
	endpoint := flag.String("endpoint", "/api/v1/search", "search endpoint path")
	k := flag.Int("k", 0, "evaluation depth (defaults to the judgment file k, else 10)")
	timeout := flag.Duration("timeout", 10*time.Second, "per-query HTTP timeout")
	flag.Parse()
	if *judgmentPath == "" {
		fmt.Fprintln(os.Stderr, "missing required --judgments path")
		flag.Usage()
		os.Exit(2)
	}
	code, message := run(*judgmentPath, *baseURL, *endpoint, *k, *timeout)
	if code == 0 {
		fmt.Println(message)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
	os.Exit(code)
}
