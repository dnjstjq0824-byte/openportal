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
	DocumentID  string  `json:"document_id,omitempty"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Language    string  `json:"language"`
	Score       float64 `json:"score"`
}
type Response struct {
	Query          string `json:"query"`
	Total          int64  `json:"total"`
	Hits           []Hit  `json:"hits"`
	ElapsedMS      int64  `json:"elapsed_ms"`
	RequestID      string `json:"request_id,omitempty"`
	CorrectedQuery string `json:"corrected_query,omitempty"`
	SafeSearch     string `json:"safe_search,omitempty"`
	Vertical       string `json:"vertical,omitempty"`
	NextPageToken  string `json:"next_page_token,omitempty"`
}
type osResponse struct {
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Failed int `json:"failed"`
	} `json:"_shards"`
	Hits *struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string  `json:"_id"`
			Score  float64 `json:"_score"`
			Source struct {
				DocumentID  string `json:"document_id"`
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Language    string `json:"language"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
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
	if page < 1 || page > 100 || limit < 1 || limit > 20 {
		return Response{}, errors.New("invalid pagination")
	}
	start := time.Now()

	parsed := ParseQuery(phrase)
	opensearchDSL := parsed.BuildOpenSearchQuery(lang)

	query := map[string]interface{}{
		"from":             (page - 1) * limit,
		"size":             limit,
		"track_total_hits": true,
		"_source":          []string{"document_id", "title", "url", "description", "language"},
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
		Hits:      make([]Hit, 0, len(result.Hits.Hits)),
		ElapsedMS: time.Since(start).Milliseconds(),
	}
	if int64(page*limit) < result.Hits.Total.Value {
		r.NextPageToken = EncodePageToken(page+1, limit)
	}
	for _, h := range result.Hits.Hits {
		if !strings.HasPrefix(h.Source.URL, "https://") && !strings.HasPrefix(h.Source.URL, "http://") {
			continue
		}
		docID := h.Source.DocumentID
		if docID == "" {
			docID = h.ID
		}
		r.Hits = append(r.Hits, Hit{
			DocumentID:  docID,
			Title:       h.Source.Title,
			URL:         h.Source.URL,
			Description: h.Source.Description,
			Language:    h.Source.Language,
			Score:       h.Score,
		})
	}
	return r, nil
}
func (c *Client) Suggest(ctx context.Context, phrase string) ([]string, error) {
	q := map[string]interface{}{"size": 8, "_source": []string{"title"}, "query": map[string]interface{}{"bool": map[string]interface{}{"must": map[string]interface{}{"match_phrase_prefix": map[string]interface{}{"title": phrase}}, "filter": map[string]interface{}{"term": map[string]interface{}{"indexable": true}}}}}
	var out osResponse
	if err := c.do(ctx, c.BaseURL+"/"+url.PathEscape(c.Index)+"/_search", q, &out); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	suggestions := []string{}
	for _, hit := range out.Hits.Hits {
		title := hit.Source.Title
		if title != "" && !seen[title] {
			suggestions = append(suggestions, title)
			seen[title] = true
		}
	}
	return suggestions, nil
}
