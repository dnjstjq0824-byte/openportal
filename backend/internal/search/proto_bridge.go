package search

import (
	"fmt"
	"strconv"
	"strings"

	searchv1 "example.com/openportal-search/backend/gen/openportal/search/v1"
)

// ToProtoSearchRequest converts the validated REST search query parameters into a canonical Protobuf v1 SearchRequest.
func ToProtoSearchRequest(query, lang string, page, pageSize int, vertical, safeSearch string) *searchv1.SearchRequest {
	locale := "ko-KR"
	if lang == "en" {
		locale = "en-US"
	} else if lang == "" {
		locale = ""
	}
	if vertical == "" {
		vertical = "web"
	}
	if safeSearch == "" {
		safeSearch = "moderate"
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	return &searchv1.SearchRequest{
		Query:      strings.TrimSpace(query),
		Locale:     locale,
		Vertical:   vertical,
		PageSize:   int32(pageSize),
		PageToken:  fmt.Sprintf("page:%d", page),
		SafeSearch: safeSearch,
	}
}

// FromProtoSearchRequest extracts REST query parameters from a Protobuf v1 SearchRequest.
func FromProtoSearchRequest(req *searchv1.SearchRequest) (query, lang string, page, pageSize int, vertical, safeSearch string, err error) {
	if req == nil {
		return "", "", 0, 0, "", "", fmt.Errorf("nil search request")
	}
	query = strings.TrimSpace(req.GetQuery())
	switch req.GetLocale() {
	case "ko", "ko-KR":
		lang = "ko"
	case "en", "en-US":
		lang = "en"
	case "":
		lang = ""
	default:
		return "", "", 0, 0, "", "", fmt.Errorf("unsupported locale: %s", req.GetLocale())
	}
	page = 1
	if tok := req.GetPageToken(); tok != "" {
		if !strings.HasPrefix(tok, "page:") {
			return "", "", 0, 0, "", "", fmt.Errorf("invalid page_token: %s", tok)
		}
		p, pErr := strconv.Atoi(strings.TrimPrefix(tok, "page:"))
		if pErr != nil || p < 1 || p > 100 {
			return "", "", 0, 0, "", "", fmt.Errorf("invalid page in page_token: %s", tok)
		}
		page = p
	}
	pageSize = int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 10
	}
	vertical = req.GetVertical()
	if vertical == "" {
		vertical = "web"
	}
	safeSearch = req.GetSafeSearch()
	if safeSearch == "" {
		safeSearch = "moderate"
	}
	return query, lang, page, pageSize, vertical, safeSearch, nil
}

// ToProtoSearchResponse converts a REST Response into a Protobuf v1 SearchResponse.
// Organic hits are strictly marked Sponsored=false per OpenPortal ad/organic separation policy.
func ToProtoSearchResponse(res Response, page, pageSize int, requestID string) *searchv1.SearchResponse {
	hits := make([]*searchv1.SearchHit, 0, len(res.Hits))
	for _, h := range res.Hits {
		hits = append(hits, &searchv1.SearchHit{
			DocumentId: h.DocumentID,
			Title:      h.Title,
			Url:        h.URL,
			Snippet:    h.Description,
			Language:   h.Language,
			Score:      h.Score,
			Sponsored:  false,
		})
	}
	nextToken := ""
	if pageSize > 0 && int64(page*pageSize) < res.Total && page < 100 {
		nextToken = fmt.Sprintf("page:%d", page+1)
	}
	return &searchv1.SearchResponse{
		OrganicHits:    hits,
		TotalEstimated: res.Total,
		NextPageToken:  nextToken,
		ElapsedMs:      res.ElapsedMS,
		RequestId:      requestID,
	}
}

// FromProtoSearchResponse converts a Protobuf v1 SearchResponse back to the REST Response structure.
func FromProtoSearchResponse(pb *searchv1.SearchResponse, query string) Response {
	if pb == nil {
		return Response{Query: query, Hits: []Hit{}}
	}
	hits := make([]Hit, 0, len(pb.GetOrganicHits()))
	for _, h := range pb.GetOrganicHits() {
		if h.GetSponsored() {
			// Organic response must never include sponsored hits
			continue
		}
		hits = append(hits, Hit{
			DocumentID:  h.GetDocumentId(),
			Title:       h.GetTitle(),
			URL:         h.GetUrl(),
			Description: h.GetSnippet(),
			Language:    h.GetLanguage(),
			Score:       h.GetScore(),
		})
	}
	return Response{
		Query:     query,
		Total:     pb.GetTotalEstimated(),
		Hits:      hits,
		ElapsedMS: pb.GetElapsedMs(),
	}
}

// ToProtoSuggestResponse converts a list of suggestion strings to Protobuf v1 SuggestResponse.
func ToProtoSuggestResponse(items []string) *searchv1.SuggestResponse {
	out := make([]string, len(items))
	copy(out, items)
	return &searchv1.SuggestResponse{Suggestions: out}
}

