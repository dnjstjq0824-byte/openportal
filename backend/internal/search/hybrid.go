package search

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
)

// Hybrid fusion depth: each path fetches from 0 so the fused ranking can be
// paged honestly. Capped to bound backend load on deep pages.
const (
	hybridDepthPadding = 10
	hybridDepthMax     = 120
)

// SearchHybrid fuses two independent OpenSearch retrieval paths with RRF
// (C-6): the precision keyword path (title-weighted BM25) and the recall
// path (fuzzy + phrase proximity). Both paths apply identical safety,
// tombstone, and attribute filters, so every fused candidate was eligible
// on both. Total is the maximum over the successful paths, so a typo that
// only the recall path matches still reports a consistent non-zero total;
// the page is sliced from the fused union.
//
// When one path fails the other still serves (degraded single-system
// fusion); when both fail the search fails.
func (c *Client) SearchHybrid(ctx context.Context, phrase, lang string, page, limit int, safeSearch string) (Response, error) {
	if page < 1 || page > 100 || limit < 1 || limit > 20 {
		return Response{}, errors.New("invalid pagination")
	}
	start := time.Now()

	parsed := ParseQuery(phrase)
	parsed.SafeSearch = safeSearch
	depth := page*limit + hybridDepthPadding
	if depth > hybridDepthMax {
		depth = hybridDepthMax
	}
	source := []string{"document_id", "title", "url", "description", "language", "published_at", "last_crawled_at", "domain_authority", "term_frequency_density"}
	target := c.BaseURL + "/" + url.PathEscape(c.Index) + "/_search"

	type pathResult struct {
		total int64
		hits  []Hit
		err   error
	}
	results := make([]pathResult, 2)
	queries := []map[string]interface{}{
		parsed.BuildOpenSearchQuery(lang),
		parsed.BuildRecallQuery(lang),
	}
	var wg sync.WaitGroup
	for i := range queries {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := map[string]interface{}{
				"from":             0,
				"size":             depth,
				"track_total_hits": true,
				"_source":          source,
				"query":            queries[i],
			}
			var out osResponse
			if err := c.do(ctx, target, body, &out); err != nil {
				results[i].err = err
				return
			}
			results[i].total = out.Hits.Total.Value
			results[i].hits = toHits(out.Hits.Hits)
		}(i)
	}
	wg.Wait()

	if results[0].err != nil && results[1].err != nil {
		return Response{}, results[0].err
	}

	systems := make([]ranking.RankingSystem, 0, 2)
	merged := map[string]Hit{}
	names := []string{"keyword", "recall"}
	for i, res := range results {
		if res.err != nil {
			continue
		}
		sys := ranking.RankingSystem{Name: names[i], Weight: 1.0, Hits: make([]ranking.RankedItem, 0, len(res.hits))}
		for _, h := range res.hits {
			sys.Hits = append(sys.Hits, ranking.RankedItem{
				DocumentID: h.DocumentID,
				Title:      h.Title,
				URL:        h.URL,
				Snippet:    h.Description,
				Score:      h.Score,
			})
			if _, seen := merged[h.DocumentID]; !seen {
				merged[h.DocumentID] = h
			}
		}
		systems = append(systems, sys)
	}
	fused := ranking.ReciprocalRankFusion(systems, ranking.DefaultRRFConfig())

	from := (page - 1) * limit
	hits := []Hit{}
	for i := from; i < len(fused) && len(hits) < limit; i++ {
		h := merged[fused[i].DocumentID]
		h.Score = fused[i].RRFScore
		hits = append(hits, h)
	}
	var total int64
	for _, res := range results {
		if res.err == nil && res.total > total {
			total = res.total
		}
	}
	r := Response{
		Query:     phrase,
		Total:     total,
		Hits:      hits,
		ElapsedMS: time.Since(start).Milliseconds(),
		// No ad engine is connected (A-1): always emit an explicit empty slot.
		SponsoredAds: []SponsoredAdItem{},
	}
	if int64(page*limit) < r.Total {
		r.NextPageToken = EncodePageToken(page+1, limit)
	}
	return r, nil
}

// SearchKNN runs a k-NN vector query over the embedding field (C-6 vector
// seam). The DSL and response parsing are real; query embeddings require an
// external embedding model, which is not configured — callers must supply a
// query vector, and an empty vector is an explicit not-configured error,
// never a silent lexical fallback.
func (c *Client) SearchKNN(ctx context.Context, queryVector []float32, size int) ([]Hit, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("vector search requires query embeddings: no embedding model configured")
	}
	if size < 1 || size > 50 {
		return nil, errors.New("invalid knn size")
	}
	body := map[string]interface{}{
		"size":    size,
		"_source": []string{"document_id", "title", "url", "description", "language", "published_at", "last_crawled_at", "domain_authority", "term_frequency_density"},
		"query": map[string]interface{}{
			"knn": map[string]interface{}{
				"embedding": map[string]interface{}{
					"vector": queryVector,
					"k":      size,
					"filter": map[string]interface{}{
						"term": map[string]interface{}{"indexable": true},
					},
				},
			},
		},
	}
	var out osResponse
	if err := c.do(ctx, c.BaseURL+"/"+url.PathEscape(c.Index)+"/_search", body, &out); err != nil {
		return nil, err
	}
	return toHits(out.Hits.Hits), nil
}
