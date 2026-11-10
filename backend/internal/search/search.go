package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL, Index, Username, Password string
	HTTP                               *http.Client
}
type Hit struct {
	DocumentID           string  `json:"document_id,omitempty"`
	Title                string  `json:"title"`
	URL                  string  `json:"url"`
	Description          string  `json:"description"`
	Language             string  `json:"language"`
	Score                float64 `json:"score"`
	PublishedAt          string  `json:"-"`
	LastCrawledAt        string  `json:"-"`
	DomainAuthority      float64 `json:"-"`
	TermFrequencyDensity float64 `json:"-"`
}
type AIOverviewSummary struct {
	Summary     string       `json:"summary"`
	Citations   []AICitation `json:"citations"`
	Confidence  float64      `json:"confidence"`
	IsFaithful  bool         `json:"is_faithful"`
	Disclaimer  string       `json:"disclaimer,omitempty"`
	ExecutionMs int64        `json:"execution_ms"`
}

type AICitation struct {
	Index    int    `json:"index"`
	SourceID string `json:"source_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Snippet  string `json:"snippet"`
}

type SponsoredAdItem struct {
	AdID           string  `json:"ad_id"`
	Headline       string  `json:"headline"`
	Description    string  `json:"description"`
	TargetURL      string  `json:"target_url"`
	DisplayURL     string  `json:"display_url"`
	SponsoredLabel string  `json:"sponsored_label"`
	ClearingCPCKRW float64 `json:"clearing_cpc_krw,omitempty"`
}

type Response struct {
	Query          string             `json:"query"`
	Total          int64              `json:"total"`
	Hits           []Hit              `json:"hits"`
	ElapsedMS      int64              `json:"elapsed_ms"`
	RequestID      string             `json:"request_id,omitempty"`
	CorrectedQuery string             `json:"corrected_query,omitempty"`
	SafeSearch     string             `json:"safe_search,omitempty"`
	Vertical       string             `json:"vertical,omitempty"`
	Intent         string             `json:"intent,omitempty"`
	NextPageToken  string             `json:"next_page_token,omitempty"`
	AIOverview     *AIOverviewSummary `json:"ai_overview,omitempty"`
	SponsoredAds   []SponsoredAdItem  `json:"sponsored_ads"`
}
type osHitSource struct {
	DocumentID           string  `json:"document_id"`
	Title                string  `json:"title"`
	URL                  string  `json:"url"`
	Description          string  `json:"description"`
	Language             string  `json:"language"`
	PublishedAt          string  `json:"published_at"`
	LastCrawledAt        string  `json:"last_crawled_at"`
	DomainAuthority      float64 `json:"domain_authority"`
	TermFrequencyDensity float64 `json:"term_frequency_density"`
}

type osHit struct {
	ID     string      `json:"_id"`
	Score  float64     `json:"_score"`
	Source osHitSource `json:"_source"`
}

type osHits struct {
	Total struct {
		Value int64 `json:"value"`
	} `json:"total"`
	Hits []osHit `json:"hits"`
}

type osResponse struct {
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Failed int `json:"failed"`
	} `json:"_shards"`
	Hits *osHits `json:"hits"`
}

// toHits maps raw OpenSearch hits to API hits, dropping URL-less records.
func toHits(hits []osHit) []Hit {
	out := make([]Hit, 0, len(hits))
	for _, h := range hits {
		if !strings.HasPrefix(h.Source.URL, "https://") && !strings.HasPrefix(h.Source.URL, "http://") {
			continue
		}
		docID := h.Source.DocumentID
		if docID == "" {
			docID = h.ID
		}
		out = append(out, Hit{
			DocumentID:           docID,
			Title:                h.Source.Title,
			URL:                  h.Source.URL,
			Description:          h.Source.Description,
			Language:             h.Source.Language,
			Score:                h.Score,
			PublishedAt:          h.Source.PublishedAt,
			LastCrawledAt:        h.Source.LastCrawledAt,
			DomainAuthority:      h.Source.DomainAuthority,
			TermFrequencyDensity: h.Source.TermFrequencyDensity,
		})
	}
	return out
}

func New(baseURL, index, username, password string) *Client {
	return &Client{BaseURL: baseURL, Index: index, Username: username, Password: password, HTTP: &http.Client{Timeout: 4 * time.Second}}
}
func (c *Client) do(ctx context.Context, target string, body interface{}, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("search backend unavailable: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		return fmt.Errorf("search backend status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 2<<20 {
		return errors.New("search response exceeds limit")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	if result, ok := out.(*osResponse); ok {
		if result.TimedOut || result.Shards.Failed > 0 || result.Hits == nil {
			return errors.New("incomplete search response")
		}
	}
	return nil
}
func (c *Client) Search(ctx context.Context, phrase, lang string, page, limit int) (Response, error) {
	return c.SearchWithSafety(ctx, phrase, lang, page, limit, "")
}

// SearchWithSafety applies the SafeSearch level inside the OpenSearch query
// (C-5) so the returned total and pagination already reflect filtering.
func (c *Client) SearchWithSafety(ctx context.Context, phrase, lang string, page, limit int, safeSearch string) (Response, error) {
	if page < 1 || page > 100 || limit < 1 || limit > 20 {
		return Response{}, errors.New("invalid pagination")
	}
	start := time.Now()

	parsed := ParseQuery(phrase)
	parsed.SafeSearch = safeSearch
	opensearchDSL := parsed.BuildOpenSearchQuery(lang)

	query := map[string]interface{}{
		"from":             (page - 1) * limit,
		"size":             limit,
		"track_total_hits": true,
		"_source":          []string{"document_id", "title", "url", "description", "language", "published_at", "last_crawled_at", "domain_authority", "term_frequency_density"},
		"query":            opensearchDSL,
	}

	target := c.BaseURL + "/" + url.PathEscape(c.Index) + "/_search"
	var result osResponse
	err := c.do(ctx, target, query, &result)
	if err != nil {
		return Response{}, err
	}
	r := Response{
		Query:     phrase,
		Total:     result.Hits.Total.Value,
		Hits:      toHits(result.Hits.Hits),
		ElapsedMS: time.Since(start).Milliseconds(),
		// No ad engine is connected (A-1): always emit an explicit empty slot.
		SponsoredAds: []SponsoredAdItem{},
	}
	if int64(page*limit) < result.Hits.Total.Value {
		r.NextPageToken = EncodePageToken(page+1, limit)
	}
	return r, nil
}

// Suggest and SuggestCorrection live in suggest.go (C-4): completion
// suggester autocomplete plus index-grounded typo correction.
