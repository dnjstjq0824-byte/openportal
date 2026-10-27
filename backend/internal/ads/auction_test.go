package ads

import (
	"testing"
)

func TestRunGSPAuction(t *testing.T) {
	bids := []AdBid{
		{AdID: "ad-1", MaxCPCKRW: 500.0, QualityScore: 8.0, Headline: "Ad 1"}, // Score = 4000
		{AdID: "ad-2", MaxCPCKRW: 600.0, QualityScore: 5.0, Headline: "Ad 2"}, // Score = 3000
		{AdID: "ad-3", MaxCPCKRW: 300.0, QualityScore: 6.0, Headline: "Ad 3"}, // Score = 1800
	}

	results, err := RunGSPAuction(bids, 2)
	if err != nil {
		t.Fatalf("unexpected auction error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 winning slots, got %d", len(results))
	}

	// Winner #1 must be ad-1 (Score 4000)
	if results[0].AdID != "ad-1" {
		t.Errorf("expected ad-1 at rank 1, got %s", results[0].AdID)
	}
	// Winner #1 clearing price under GSP: nextScore / qualityScore + 1 = 3000 / 8.0 + 1 = 376.0 KRW
	expectedClearing := (3000.0 / 8.0) + 1.0
	if results[0].ClearingCPCKRW != expectedClearing {
		t.Errorf("expected clearing price %f, got %f", expectedClearing, results[0].ClearingCPCKRW)
	}

	// Must have mandatory Sponsored tag
	for _, res := range results {
		if !res.IsSponsored || res.SponsoredLabel != "광고 / Sponsored" {
			t.Errorf("expected mandatory sponsored tag disclosure on ad result %+v", res)
		}
	}
}

func TestValidateOrganicSeparation(t *testing.T) {
	organic := []float64{3.5, 2.1, 1.8}
	ads := []AdBid{{AdID: "ad-1", MaxCPCKRW: 500.0, QualityScore: 8.0}}

	if err := ValidateOrganicSeparation(organic, ads); err != nil {
		t.Errorf("unexpected validation failure: %v", err)
	}
}

