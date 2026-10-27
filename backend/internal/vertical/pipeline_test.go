package vertical

import (
	"context"
	"testing"
	"time"
)

func TestPhase5EndToEndVerticalPipeline(t *testing.T) {
	// 1. Image vertical
	validLicense, _ := ValidateImageLicense(LicenseCCBY, "https://openportal.org/test.png")
	if !validLicense {
		t.Fatalf("image license check failed")
	}
	thumbSpec, err := GenerateThumbnailSpec(1920, 1080, ThumbnailSmall)
	if err != nil || thumbSpec.TargetWidth != 150 {
		t.Fatalf("thumbnail generation failed: %v", err)
	}
	safety := ClassifyImageSafety([]string{"landscape"}, 0.05, 0.05)
	if safety != SafetySafe {
		t.Fatalf("image safety classification failed")
	}

	// 2. Video vertical
	vItem, err := NewVideoItem(
		"v-test-1",
		"OpenPortal Engine Overview",
		"Technical architecture walkthrough",
		360,
		"https://media.openportal.org/thumb.jpg",
		"https://www.youtube.com/embed/dQw4w9WgXcQ",
		"OpenPortal Team",
		time.Now(),
		true,
	)
	if err != nil || vItem.Category != DurationMedium {
		t.Fatalf("video creation or duration categorization failed: %v", err)
	}

	// 3. News vertical
	pubRegistry := map[string]PublisherPolicy{
		"pub-portal": {
			PublisherID:      "pub-portal",
			Name:             "Portal News",
			Website:          "https://news.openportal.org",
			Licensed:         true,
			MaxSummaryChars:  200,
			AllowSyndication: true,
		},
	}
	newsArt, err := IngestNewsArticle(NewsArticle{
		ID:           "n-1",
		PublisherID:  "pub-portal",
		Title:        "OpenPortal Launches Phase 5 Vertical Search",
		Summary:      "OpenPortal announces verified vertical search for Images, Videos, News, Maps, and Shopping.",
		CanonicalURL: "https://news.openportal.org/article/1",
		PublishedAt:  time.Now(),
	}, pubRegistry)
	if err != nil || newsArt.SimHash == 0 {
		t.Fatalf("news ingestion failed: %v", err)
	}

	newsMgr := NewNewsLifecycleManager(365)
	activeNews := newsMgr.FilterActiveArticles([]NewsArticle{*newsArt})
	if len(activeNews) != 1 {
		t.Fatalf("news active filter failed")
	}

	// 4. Map & POI vertical
	poiMgr := NewPOIManager()
	err = poiMgr.RegisterPlace(POIPlace{
		ID:            "poi-seoul",
		Name:          "Seoul City Hall",
		Address:       "110 Sejong-daero, Jung-gu, Seoul",
		Location:      GeoPoint{Latitude: 37.566535, Longitude: 126.977969},
		IsVerified:    true,
		SourceLicense: "ODbL-1.0",
	})
	if err != nil {
		t.Fatalf("poi registration failed: %v", err)
	}
	poiResults := poiMgr.SearchPOI("seoul", GeoPoint{Latitude: 37.5665, Longitude: 126.9780}, 10.0)
	if len(poiResults) == 0 {
		t.Fatalf("poi search returned 0 results")
	}

	// 5. Shopping vertical
	shopMgr := NewShoppingFeedManager(24 * time.Hour)
	_, err = shopMgr.IngestOffer(ProductOffer{
		ID:            "shop-1",
		Title:         "High Performance Laptop",
		MerchantName:  "OpenPortal Store",
		Price:         1500000,
		Currency:      "KRW",
		Availability:  InStock,
		IsSponsored:   true,
		FeedUpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("shopping offer ingestion failed: %v", err)
	}
	shopResults := shopMgr.SearchOffers("laptop", 2000000, true, "price_asc")
	if len(shopResults) == 0 || shopResults[0].SponsoredTag != "광고 / Sponsored" {
		t.Fatalf("shopping search or sponsored tag failed")
	}

	// 6. Federation coordinator
	coord := NewVerticalFederationCoordinator(1*time.Minute, 200*time.Millisecond)
	resp, err := coord.Execute(context.Background(), VerticalQuery{
		Vertical: VerticalNews,
		Query:    "openportal",
		Page:     1,
		PageSize: 10,
	}, func(ctx context.Context, vq VerticalQuery) (any, int, string, error) {
		return activeNews, 1, "Licensed News Feed", nil
	})
	if err != nil || resp.TotalHits != 1 {
		t.Fatalf("federation execution failed: %v", err)
	}

	// 7. Phase 5 Gate Evaluation (Technical Pass, Legal Contracts Pending -> NO-GO)
	checklist := VerticalAuditChecklist{
		ImageLicensingAndSafetyPassed: true,
		VideoEmbedWhitelistPassed:     true,
		NewsPublisherAndRetractPassed: true,
		MapsODbLAttributionPassed:     true,
		ShoppingSponsoredTagPassed:    true,
		CommercialContractsExecuted:   false, // Strict gate: contracts require external legal signature
		LegalSignoffRecorded:          false,
	}

	report := EvaluatePhase5ReleaseGate(checklist)
	if report.Decision != GateDecisionNoGo {
		t.Fatalf("expected GateDecisionNoGo due to missing commercial contracts, got %s", report.Decision)
	}
	if len(report.UnresolvedRisks) != 2 {
		t.Fatalf("expected 2 unresolved risks (commercial contracts & legal signoff), got %d", len(report.UnresolvedRisks))
	}
}
