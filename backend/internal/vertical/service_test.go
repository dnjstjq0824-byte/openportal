package vertical

import (
	"context"
	"testing"
)

func TestVerticalServiceFederatedSearch(t *testing.T) {
	svc := NewVerticalService()
	ctx := context.Background()

	// 1. Image Search
	imgResp, err := svc.ExecuteVerticalQuery(ctx, VerticalQuery{
		Vertical: VerticalImage,
		Query:    "검색엔진",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("image search failed: %v", err)
	}
	if imgResp.TotalHits == 0 {
		t.Fatalf("expected at least 1 image hit for '검색엔진'")
	}
	images, ok := imgResp.Data.([]SearchableImage)
	if !ok || len(images) == 0 {
		t.Fatalf("expected []SearchableImage data type")
	}
	if images[0].License != LicenseCCBY && images[0].License != LicenseCCBYSA && images[0].License != LicensePublicDomain {
		t.Fatalf("invalid image license: %s", images[0].License)
	}

	// 2. Video Search
	vidResp, err := svc.ExecuteVerticalQuery(ctx, VerticalQuery{
		Vertical: VerticalVideo,
		Query:    "트래픽",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("video search failed: %v", err)
	}
	if vidResp.TotalHits == 0 {
		t.Fatalf("expected at least 1 video hit for '트래픽'")
	}
	videos, ok := vidResp.Data.([]VideoItem)
	if !ok || len(videos) == 0 {
		t.Fatalf("expected []VideoItem data type")
	}
	if videos[0].EmbedURL == "" || videos[0].Category != DurationMedium {
		t.Fatalf("video item metadata missing or incorrect")
	}

	// 3. News Search
	newsResp, err := svc.ExecuteVerticalQuery(ctx, VerticalQuery{
		Vertical: VerticalNews,
		Query:    "Phase 5",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("news search failed: %v", err)
	}
	if newsResp.TotalHits == 0 {
		t.Fatalf("expected news hits for 'Phase 5'")
	}
	articles, ok := newsResp.Data.([]NewsArticle)
	if !ok || len(articles) == 0 {
		t.Fatalf("expected []NewsArticle data type")
	}
	if articles[0].Publisher == "" || articles[0].CanonicalURL == "" {
		t.Fatalf("news attribution missing")
	}

	// 4. Map / POI Search
	mapResp, err := svc.ExecuteVerticalQuery(ctx, VerticalQuery{
		Vertical: VerticalMap,
		Query:    "시청",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("map search failed: %v", err)
	}
	if mapResp.TotalHits == 0 {
		t.Fatalf("expected map hits for '시청'")
	}
	pois, ok := mapResp.Data.([]POISearchResult)
	if !ok || len(pois) == 0 {
		t.Fatalf("expected []POISearchResult data type")
	}
	if pois[0].Place.Location.Latitude == 0 || pois[0].Place.Location.Longitude == 0 {
		t.Fatalf("missing lat/lng for POI")
	}

	// 5. Shopping Search
	shopResp, err := svc.ExecuteVerticalQuery(ctx, VerticalQuery{
		Vertical: VerticalShopping,
		Query:    "노트북 랩탑",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("shopping search failed: %v", err)
	}
	if shopResp.TotalHits == 0 {
		t.Fatalf("expected shopping hits for '랩탑'")
	}
	offers, ok := shopResp.Data.([]ProductOffer)
	if !ok || len(offers) == 0 {
		t.Fatalf("expected []ProductOffer data type")
	}
	if !offers[0].IsSponsored || offers[0].SponsoredTag != "광고 / Sponsored" {
		t.Fatalf("shopping sponsored tag disclosure missing")
	}
}

