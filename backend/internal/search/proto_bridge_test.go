package search

import (
	"testing"

	searchv1 "example.com/openportal-search/backend/gen/openportal/search/v1"
)

func TestProtoBridgeRoundTripAndOrganicIsolation(t *testing.T) {
	req := ToProtoSearchRequest("  검색엔진  ", "ko", 2, 10, "web", "strict")
	if req.GetQuery() != "검색엔진" || req.GetLocale() != "ko-KR" || req.GetPageToken() != "page:2" || req.GetSafeSearch() != "strict" {
		t.Fatalf("unexpected proto request: %+v", req)
	}
	q, lang, page, size, vertical, safe, err := FromProtoSearchRequest(req)
	if err != nil {
		t.Fatalf("FromProtoSearchRequest failed: %v", err)
	}
	if q != "검색엔진" || lang != "ko" || page != 2 || size != 10 || vertical != "web" || safe != "strict" {
		t.Fatalf("mismatch extracted params: q=%s lang=%s page=%d size=%d vertical=%s safe=%s", q, lang, page, size, vertical, safe)
	}

	if _, _, _, _, _, _, err := FromProtoSearchRequest(&searchv1.SearchRequest{Query: "x", PageToken: "invalid"}); err == nil {
		t.Fatal("expected error for invalid page_token")
	}

	restRes := Response{
		Query:     "검색엔진",
		Total:     25,
		ElapsedMS: 12,
		Hits: []Hit{
			{DocumentID: "fixture-001", Title: "오픈포털 검색엔진 프로젝트", URL: "https://example.org/intro", Description: "예시 설명", Language: "ko", Score: 3.14},
		},
	}
	pbRes := ToProtoSearchResponse(restRes, 2, 10, "req-12345")
	if pbRes.GetNextPageToken() != "page:3" || pbRes.GetRequestId() != "req-12345" || len(pbRes.GetOrganicHits()) != 1 {
		t.Fatalf("unexpected proto response: %+v", pbRes)
	}
	if pbRes.GetOrganicHits()[0].GetSponsored() {
		t.Fatal("organic hit must never be marked sponsored")
	}

	// Inject a sponsored hit into proto response and verify FromProtoSearchResponse strips it from organic results
	pbRes.OrganicHits = append(pbRes.OrganicHits, &searchv1.SearchHit{
		DocumentId: "ad-001",
		Title:      "Sponsored Ad",
		Url:        "https://example.org/ad",
		Sponsored:  true,
	})
	roundTrip := FromProtoSearchResponse(pbRes, "검색엔진")
	if roundTrip.Total != 25 || len(roundTrip.Hits) != 1 || roundTrip.Hits[0].DocumentID != "fixture-001" {
		t.Fatalf("roundtrip failed to strip sponsored hit: %+v", roundTrip)
	}

	sugPb := ToProtoSuggestResponse([]string{"오픈포털", "검색엔진"})
	if len(sugPb.GetSuggestions()) != 2 || sugPb.GetSuggestions()[0] != "오픈포털" {
		t.Fatalf("unexpected suggest proto: %+v", sugPb)
	}
}

