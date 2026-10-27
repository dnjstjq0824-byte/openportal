package vertical

import (
	"testing"
	"time"
)

func TestIngestOfferValidation(t *testing.T) {
	mgr := NewShoppingFeedManager(24 * time.Hour)

	// Valid offer with sponsored tag
	offer := ProductOffer{
		ID:            "off-1",
		Title:         "Wireless Mechanical Keyboard",
		MerchantName:  "TechMall",
		Price:         89000,
		Currency:      "KRW",
		Availability:  InStock,
		IsSponsored:   true,
		FeedUpdatedAt: time.Now(),
	}
	ingested, err := mgr.IngestOffer(offer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ingested.SponsoredTag != "광고 / Sponsored" {
		t.Errorf("expected sponsored disclosure, got %q", ingested.SponsoredTag)
	}
	if ingested.IsPriceStale {
		t.Errorf("fresh offer must not be stale")
	}

	// Negative price rejected
	badPrice := offer
	badPrice.ID = "off-bad-price"
	badPrice.Price = -100
	if _, err := mgr.IngestOffer(badPrice); err == nil {
		t.Errorf("expected error for negative price, got nil")
	}

	// Unsupported currency rejected
	badCurr := offer
	badCurr.ID = "off-bad-curr"
	badCurr.Currency = "XYZ"
	if _, err := mgr.IngestOffer(badCurr); err == nil {
		t.Errorf("expected error for unsupported currency, got nil")
	}

	// Stale feed offer (>24h)
	staleOffer := offer
	staleOffer.ID = "off-stale"
	staleOffer.FeedUpdatedAt = time.Now().Add(-30 * time.Hour)
	ingestedStale, _ := mgr.IngestOffer(staleOffer)
	if !ingestedStale.IsPriceStale {
		t.Errorf("expected 30h old feed offer to be marked stale")
	}
}

func TestSearchOffersAndSorting(t *testing.T) {
	mgr := NewShoppingFeedManager(24 * time.Hour)
	now := time.Now()

	_, _ = mgr.IngestOffer(ProductOffer{ID: "o1", Title: "Gaming Mouse A", Price: 50000, Currency: "KRW", Availability: InStock, FeedUpdatedAt: now})
	_, _ = mgr.IngestOffer(ProductOffer{ID: "o2", Title: "Gaming Mouse B", Price: 30000, Currency: "KRW", Availability: InStock, FeedUpdatedAt: now})
	_, _ = mgr.IngestOffer(ProductOffer{ID: "o3", Title: "Gaming Mouse Out of Stock", Price: 20000, Currency: "KRW", Availability: OutOfStock, FeedUpdatedAt: now})

	// Search in-stock with price_asc
	results := mgr.SearchOffers("Gaming Mouse", 0, true, "price_asc")
	if len(results) != 2 {
		t.Fatalf("expected 2 in-stock items, got %d", len(results))
	}
	if results[0].Price != 30000 || results[1].Price != 50000 {
		t.Errorf("expected ascending price order: got %f, %f", results[0].Price, results[1].Price)
	}
}

func TestSubmitDiscrepancyReport(t *testing.T) {
	mgr := NewShoppingFeedManager(24 * time.Hour)
	_, _ = mgr.IngestOffer(ProductOffer{ID: "o1", Title: "Test Item", Price: 10000, Currency: "KRW", FeedUpdatedAt: time.Now()})

	rep, err := mgr.SubmitDiscrepancyReport("o1", 10000, 15000, "Price increased on checkout", "acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "pending" {
		t.Errorf("expected pending status, got %s", rep.Status)
	}
}
