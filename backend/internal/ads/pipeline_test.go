package ads

import (
	"testing"
	"time"
)

func TestPhase7EndToEndAdsPipeline(t *testing.T) {
	// 1. Webmaster domain verification
	wmMgr := NewWebmasterManager()
	rec, err := wmMgr.GenerateVerificationRecord("acc-wm", "example.com", VerifyDNSTXT)
	if err != nil {
		t.Fatalf("failed to generate verification: %v", err)
	}
	verified, err := wmMgr.ConfirmVerification("acc-wm", "example.com", rec.ExpectedToken)
	if err != nil || !verified {
		t.Fatalf("domain verification failed: %v", err)
	}
	sitemap, err := wmMgr.SubmitSitemap("acc-wm", "https://example.com/sitemap.xml")
	if err != nil || sitemap.Status != "processed" {
		t.Fatalf("sitemap submission failed: %v", err)
	}

	// 2. Campaign review and state machine
	creative := AdCreative{
		ID:             "cr-1",
		CampaignID:     "cmp-1",
		Headline:       "최신 고성능 노트북 특가",
		Description:    "공식 온라인 스토어에서 특별 혜택을 확인하세요.",
		DestinationURL: "https://store.example.com/laptops",
		Status:         StatusDraft,
	}
	approved, _ := ReviewCreative(creative, []string{"region_kr"})
	if !approved {
		t.Fatalf("creative review failed unexpectedly")
	}
	if err := ApplyTransition(&creative, StatusPendingReview, "advertiser"); err != nil {
		t.Fatalf("failed transition to pending_review: %v", err)
	}
	if err := ApplyTransition(&creative, StatusApproved, "ad_reviewer"); err != nil {
		t.Fatalf("failed transition to approved: %v", err)
	}

	// 3. GSP Auction execution with mandatory sponsored tag
	bids := []AdBid{
		{AdID: "cr-1", MaxCPCKRW: 600, QualityScore: 8.0, Headline: creative.Headline, Description: creative.Description, TargetURL: creative.DestinationURL},
		{AdID: "cr-2", MaxCPCKRW: 500, QualityScore: 6.0, Headline: "Competitor Ad", Description: "Desc", TargetURL: "https://store2.test"},
	}
	auctionResults, err := RunGSPAuction(bids, 1)
	if err != nil || len(auctionResults) == 0 {
		t.Fatalf("auction failed: %v", err)
	}
	if !auctionResults[0].IsSponsored || auctionResults[0].SponsoredLabel != "광고 / Sponsored" {
		t.Fatalf("mandatory sponsored label missing")
	}

	// 4. Click fraud validation
	fraudDetector := NewClickFraudDetector(5 * time.Second)
	clickVal := fraudDetector.ValidateClick("198.51.100.5", "Mozilla/5.0 Chrome", "cr-1", auctionResults[0].ClearingCPCKRW)
	if !clickVal.IsValid {
		t.Fatalf("genuine click flagged as invalid")
	}

	// 5. Double-entry ledger accounting
	ledger := NewAdBillingLedger()
	_, err = ledger.Deposit("acc-adv-1", 50000, "KRW", "dep-001")
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}
	tx, err := ledger.ChargeClick("acc-adv-1", clickVal.BillableAmount, "KRW", "click-001")
	if err != nil || tx.BalanceAfter >= 50000 {
		t.Fatalf("charge click failed: %v", err)
	}

	// 6. Release Gate Evaluation (Technical PASS, Financial Audit Pending -> NO-GO)
	checklist := AdsAuditChecklist{
		WebmasterDomainVerificationPassed: true,
		CampaignStateMachinePassed:        true,
		ProhibitedIndustryFilterPassed:    true,
		GSPAuctionSponsoredTagPassed:      true,
		ClickFraudAbuseDetectionPassed:    true,
		DoubleEntryLedgerBalancedPassed:   true,
		RealTransactionSandboxEnforced:    true,
		ThirdPartyFinancialAuditPassed:    false, // Must remain fail-closed per rules
		DualAuthorizationApprovalSigned:   false,
	}
	report := EvaluatePhase7ReleaseGate(checklist)
	if report.Decision != GateDecisionNoGo {
		t.Fatalf("expected GateDecisionNoGo due to missing financial audit, got %s", report.Decision)
	}
	if len(report.UnresolvedRisks) != 2 {
		t.Fatalf("expected 2 unresolved risks, got %d", len(report.UnresolvedRisks))
	}
}
