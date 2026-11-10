package search

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

var errNoSuggesterEnvelope = errors.New("completion suggester envelope missing")

// completionResponse decodes the OpenSearch completion suggester envelope.
type completionResponse struct {
	Suggest map[string][]struct {
		Options []struct {
			Text string `json:"text"`
		} `json:"options"`
	} `json:"suggest"`
}

// Suggest returns up to 8 autocomplete candidates for phrase (C-4). Primary
// path is the completion suggester over the index-time suggest inputs
// (Nori-analyzed on documents_v2); when the backend rejects it (an index
// created before the suggest field existed), it falls back to a
// match_phrase_prefix query over titles. Both paths query the real index.
func (c *Client) Suggest(ctx context.Context, phrase string) ([]string, error) {
	if items, err := c.suggestCompletion(ctx, phrase); err == nil {
		return items, nil
	}
	return c.suggestPrefix(ctx, phrase)
}

func (c *Client) suggestCompletion(ctx context.Context, phrase string) ([]string, error) {
	body := map[string]interface{}{
		"size": 0,
		"suggest": map[string]interface{}{
			"text": phrase,
			"title-complete": map[string]interface{}{
				"completion": map[string]interface{}{
					"field":           "suggest",
					"size":            8,
					"skip_duplicates": true,
					"fuzzy":           map[string]interface{}{"fuzziness": "AUTO"},
				},
			},
		},
	}
	var out completionResponse
	if err := c.do(ctx, c.BaseURL+"/"+url.PathEscape(c.Index)+"/_search", body, &out); err != nil {
		return nil, err
	}
	// A real backend always echoes the suggester envelope; a missing key
	// means the response is not a suggester answer (fail closed → fallback).
	entries, ok := out.Suggest["title-complete"]
	if !ok {
		return nil, errNoSuggesterEnvelope
	}
	items := []string{}
	for _, entry := range entries {
		for _, opt := range entry.Options {
			if opt.Text == "" {
				continue
			}
			dup := false
			for _, seen := range items {
				if seen == opt.Text {
					dup = true
					break
				}
			}
			if !dup {
				items = append(items, opt.Text)
			}
		}
	}
	return items, nil
}

// suggestPrefix is the pre-C-4 title prefix query, kept as the fallback for
// indexes that predate the suggest mapping field.
func (c *Client) suggestPrefix(ctx context.Context, phrase string) ([]string, error) {
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

// phraseResponse decodes the OpenSearch phrase suggester envelope.
type phraseResponse struct {
	Suggest map[string][]struct {
		Options []struct {
			Text  string  `json:"text"`
			Score float64 `json:"score"`
		} `json:"options"`
	} `json:"suggest"`
}

// SuggestCorrection returns an index-grounded spelling correction for phrase,
// or "" when the query stands as-is (C-4). It replaces the hardcoded rewrite
// map with two real mechanisms:
//
//  1. Keyboard-layout rescue: QWERTY-typed Korean (2-bulshik) is converted and
//     accepted only when the converted query matches indexed documents while
//     the original matches none.
//  2. Phrase suggester: candidates come from actual index terms
//     (suggest_mode "missing" emits options only for out-of-vocabulary terms,
//     so valid queries are never "corrected").
func (c *Client) SuggestCorrection(ctx context.Context, phrase string) (string, error) {
	trimmed := strings.TrimSpace(phrase)
	if trimmed == "" {
		return "", nil
	}
	if converted := ConvertQwertyToHangul(trimmed); converted != "" && !strings.EqualFold(converted, trimmed) {
		origTotal, err := c.countMatches(ctx, trimmed)
		if err != nil {
			return "", err
		}
		if origTotal == 0 {
			convTotal, err := c.countMatches(ctx, converted)
			if err != nil {
				return "", err
			}
			if convTotal > 0 {
				return converted, nil
			}
		}
	}
	return c.phraseCorrection(ctx, trimmed)
}

// countMatches returns the total indexed matches for phrase (size 0).
func (c *Client) countMatches(ctx context.Context, phrase string) (int64, error) {
	parsed := ParseQuery(phrase)
	body := map[string]interface{}{
		"size":             0,
		"track_total_hits": true,
		"query":            parsed.BuildOpenSearchQuery(""),
	}
	var out osResponse
	if err := c.do(ctx, c.BaseURL+"/"+url.PathEscape(c.Index)+"/_search", body, &out); err != nil {
		return 0, err
	}
	if out.Hits == nil {
		return 0, nil
	}
	return out.Hits.Total.Value, nil
}

// phraseCorrection asks the index for a spelling correction built from its
// own terms, consulting both title and body (body-only phrases such as
// "search engine" have no title evidence). suggest_mode "missing" restricts
// candidates to terms absent from the index, so in-vocabulary queries yield
// no options. The highest-scoring candidate across both fields wins.
func (c *Client) phraseCorrection(ctx context.Context, phrase string) (string, error) {
	phraseFor := func(field string) map[string]interface{} {
		return map[string]interface{}{
			"phrase": map[string]interface{}{
				"field":      field,
				"size":       1,
				"gram_size":  1,
				"confidence": 1.0,
				"max_errors": 2,
				"direct_generator": []interface{}{
					map[string]interface{}{
						"field":        field,
						"suggest_mode": "missing",
					},
				},
			},
		}
	}
	body := map[string]interface{}{
		"size": 0,
		"suggest": map[string]interface{}{
			"text":             phrase,
			"correction-title": phraseFor("title"),
			"correction-body":  phraseFor("body"),
		},
	}
	var out phraseResponse
	if err := c.do(ctx, c.BaseURL+"/"+url.PathEscape(c.Index)+"/_search", body, &out); err != nil {
		return "", err
	}
	if _, ok := out.Suggest["correction-title"]; !ok {
		return "", errNoSuggesterEnvelope
	}
	best := ""
	bestScore := -1.0
	for _, key := range []string{"correction-title", "correction-body"} {
		for _, entry := range out.Suggest[key] {
			for _, opt := range entry.Options {
				if opt.Text == "" || strings.EqualFold(opt.Text, phrase) {
					continue
				}
				if opt.Score > bestScore {
					best, bestScore = opt.Text, opt.Score
				}
			}
		}
	}
	return best, nil
}
