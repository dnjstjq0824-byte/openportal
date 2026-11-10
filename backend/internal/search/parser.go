package search

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// ParsedQuery contains structured elements from a search input string.
type ParsedQuery struct {
	RawQuery       string
	CleanQuery     string
	Phrases        []string
	Negations      []string
	SiteFilter     string
	LangFilter     string
	FiletypeFilter string
	PositiveTerms  []string
	// SafeSearch is one of "", "strict", "moderate", "off". Empty and "moderate"
	// behave identically (moderate default), matching the runtime classifier.
	SafeSearch string
}

// safetyFilterClauses renders the SafeSearch level as must_not clauses over the
// index-time safety labels (C-5). Semantics mirror the runtime classifier's
// FilterResults exactly; documents without labels pass (fail-open for legacy
// docs — all newly indexed documents carry labels).
func safetyFilterClauses(level string) []interface{} {
	switch level {
	case "off":
		return []interface{}{
			map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"safety": "malware"}},
				map[string]interface{}{"range": map[string]interface{}{"malware_score": map[string]interface{}{"gte": 0.9}}},
			}}},
		}
	case "strict":
		return []interface{}{
			map[string]interface{}{"terms": map[string]interface{}{"safety": []string{"adult", "gambling", "malware"}}},
		}
	default: // "" and "moderate"
		return []interface{}{
			map[string]interface{}{"terms": map[string]interface{}{"safety": []string{"gambling", "malware"}}},
			map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{
				map[string]interface{}{"term": map[string]interface{}{"safety": "adult"}},
				map[string]interface{}{"range": map[string]interface{}{"adult_score": map[string]interface{}{"gte": 0.7}}},
			}}},
		}
	}
}

// ParseQuery extracts exact phrases ("..."), negations (-term), site filters (site:...),
// and language filters (lang:...) from the input query.
func ParseQuery(raw string) ParsedQuery {
	trimmed := strings.TrimSpace(raw)
	res := ParsedQuery{RawQuery: trimmed}
	if trimmed == "" {
		return res
	}

	var terms []string
	inQuote := false
	var cur strings.Builder

	for _, r := range trimmed {
		if r == '"' {
			if inQuote {
				phrase := strings.TrimSpace(cur.String())
				if phrase != "" {
					res.Phrases = append(res.Phrases, phrase)
				}
				cur.Reset()
				inQuote = false
			} else {
				if cur.Len() > 0 {
					terms = append(terms, cur.String())
					cur.Reset()
				}
				inQuote = true
			}
		} else if r == ' ' && !inQuote {
			if cur.Len() > 0 {
				terms = append(terms, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		if inQuote {
			// Unclosed quote: treat as regular phrase
			phrase := strings.TrimSpace(cur.String())
			if phrase != "" {
				res.Phrases = append(res.Phrases, phrase)
			}
		} else {
			terms = append(terms, cur.String())
		}
	}

	var positive []string
	for _, t := range terms {
		lower := strings.ToLower(t)
		if strings.HasPrefix(lower, "site:") {
			val := strings.TrimPrefix(t, "site:")
			val = strings.TrimPrefix(val, "SITE:")
			res.SiteFilter = strings.TrimSpace(val)
		} else if strings.HasPrefix(lower, "lang:") {
			val := strings.TrimPrefix(t, "lang:")
			val = strings.TrimPrefix(val, "LANG:")
			res.LangFilter = strings.TrimSpace(val)
		} else if strings.HasPrefix(lower, "filetype:") {
			val := strings.TrimPrefix(lower, "filetype:")
			res.FiletypeFilter = strings.TrimSpace(val)
		} else if strings.HasPrefix(lower, "ext:") {
			val := strings.TrimPrefix(lower, "ext:")
			res.FiletypeFilter = strings.TrimSpace(val)
		} else if strings.HasPrefix(t, "-") && len(t) > 1 {
			neg := strings.TrimPrefix(t, "-")
			res.Negations = append(res.Negations, strings.TrimSpace(neg))
		} else {
			positive = append(positive, t)
		}
	}

	res.PositiveTerms = positive
	var cleanParts []string
	if len(positive) > 0 {
		cleanParts = append(cleanParts, strings.Join(positive, " "))
	}
	for _, p := range res.Phrases {
		cleanParts = append(cleanParts, `"`+p+`"`)
	}
	res.CleanQuery = strings.TrimSpace(strings.Join(cleanParts, " "))

	return res
}

// sharedMustNot returns the exclusions every retrieval path must apply:
// tombstones plus the C-5 SafeSearch level filters.
func (p *ParsedQuery) sharedMustNot() []interface{} {
	mustNot := []interface{}{
		map[string]interface{}{
			"term": map[string]interface{}{"is_tombstone": true},
		},
	}
	return append(mustNot, safetyFilterClauses(p.SafeSearch)...)
}

// sharedFilters returns the filters every retrieval path must apply:
// indexable plus site/filetype/language restrictions.
func (p *ParsedQuery) sharedFilters(defaultLang string) []interface{} {
	filters := []interface{}{
		map[string]interface{}{"term": map[string]interface{}{"indexable": true}},
	}
	if p.SiteFilter != "" {
		filters = append(filters, map[string]interface{}{
			"wildcard": map[string]interface{}{
				"url": "*" + strings.ToLower(p.SiteFilter) + "*",
			},
		})
	}
	if p.FiletypeFilter != "" {
		filters = append(filters, map[string]interface{}{
			"wildcard": map[string]interface{}{
				"url": "*." + strings.ToLower(p.FiletypeFilter) + "*",
			},
		})
	}
	lang := p.LangFilter
	if lang == "" && defaultLang != "" {
		lang = defaultLang
	}
	if lang != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"language": lang},
		})
	}
	return filters
}

// BuildOpenSearchQuery constructs the OpenSearch DSL bool query from ParsedQuery.
func (p *ParsedQuery) BuildOpenSearchQuery(defaultLang string) map[string]interface{} {
	mustClauses := []interface{}{}
	mustNotClauses := []interface{}{}
	filterClauses := []interface{}{}

	// 1. Positive terms (BM25 multi-match)
	if len(p.PositiveTerms) > 0 {
		termQuery := strings.Join(p.PositiveTerms, " ")
		mustClauses = append(mustClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": termQuery,
				"fields": []string{
					"title^3.5",
					"headings^2.0",
					"anchor_texts^1.8",
					"description^1.5",
					"body^1.0",
				},
				"type": "best_fields",
			},
		})
	}

	// 2. Exact phrases
	for _, phrase := range p.Phrases {
		mustClauses = append(mustClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  phrase,
				"fields": []string{"title", "body", "headings"},
				"type":   "phrase",
			},
		})
	}

	// 3. Negations
	for _, neg := range p.Negations {
		mustNotClauses = append(mustNotClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  neg,
				"fields": []string{"title", "body"},
				"type":   "best_fields",
			},
		})
	}

	// Tombstones + C-5 SafeSearch + attribute filters: shared by every
	// retrieval path so keyword and recall candidates obey identical rules.
	mustNotClauses = append(mustNotClauses, p.sharedMustNot()...)
	filterClauses = append(filterClauses, p.sharedFilters(defaultLang)...)

	boolQuery := map[string]interface{}{
		"filter": filterClauses,
	}
	if len(mustClauses) > 0 {
		boolQuery["must"] = mustClauses
	} else {
		boolQuery["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	}
	if len(mustNotClauses) > 0 {
		boolQuery["must_not"] = mustNotClauses
	}

	return map[string]interface{}{
		"bool": boolQuery,
	}
}

// BuildRecallQuery constructs the recall-oriented second retrieval path for
// hybrid fusion (C-6): typo-tolerant fuzzy matching over broader fields plus
// phrase proximity, OR-ed so either signal retrieves. Safety, tombstone, and
// attribute filters are identical to the keyword path by construction.
func (p *ParsedQuery) BuildRecallQuery(defaultLang string) map[string]interface{} {
	shouldClauses := []interface{}{}
	if len(p.PositiveTerms) > 0 {
		termQuery := strings.Join(p.PositiveTerms, " ")
		shouldClauses = append(shouldClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": termQuery,
				"fields": []string{
					"title^2.0",
					"description",
					"body",
					"headings",
				},
				"type":          "best_fields",
				"fuzziness":     "AUTO",
				"prefix_length": 2,
			},
		})
		shouldClauses = append(shouldClauses, map[string]interface{}{
			"match_phrase": map[string]interface{}{
				"body": map[string]interface{}{"query": termQuery, "slop": 3},
			},
		})
	}
	for _, phrase := range p.Phrases {
		shouldClauses = append(shouldClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  phrase,
				"fields": []string{"title", "body", "headings"},
				"type":   "phrase",
			},
		})
	}

	mustNotClauses := []interface{}{}
	for _, neg := range p.Negations {
		mustNotClauses = append(mustNotClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  neg,
				"fields": []string{"title", "body"},
				"type":   "best_fields",
			},
		})
	}
	mustNotClauses = append(mustNotClauses, p.sharedMustNot()...)

	boolQuery := map[string]interface{}{
		"filter":               p.sharedFilters(defaultLang),
		"must_not":             mustNotClauses,
		"minimum_should_match": 1,
	}
	if len(shouldClauses) > 0 {
		boolQuery["should"] = shouldClauses
	} else {
		boolQuery["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
		delete(boolQuery, "minimum_should_match")
	}

	return map[string]interface{}{
		"bool": boolQuery,
	}
}

// EncodePageToken encodes page and limit into an opaque URL-safe base64 token.
func EncodePageToken(page, limit int) string {
	raw := fmt.Sprintf("p:%d:l:%d", page, limit)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// DecodePageToken decodes an opaque page token.
func DecodePageToken(token string) (page, limit int, err error) {
	bytes, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(string(bytes), ":")
	if len(parts) != 4 || parts[0] != "p" || parts[2] != "l" {
		return 0, 0, fmt.Errorf("invalid page token format")
	}
	p, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	l, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, 0, err
	}
	return p, l, nil
}

