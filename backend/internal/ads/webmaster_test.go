package ads

import (
	"testing"
)

func TestWebmasterDomainVerificationAndSitemap(t *testing.T) {
	mgr := NewWebmasterManager()

	accountID := "acc-webmaster-1"
	domain := "example.org"

	// 1. Generate verification token
	rec, err := mgr.GenerateVerificationRecord(accountID, domain, VerifyDNSTXT)
	if err != nil {
		t.Fatalf("failed to generate verification: %v", err)
	}
	if rec.IsVerified {
		t.Errorf("new verification record must not be verified initially")
	}

	// 2. Unverified sitemap submission must fail
	_, err = mgr.SubmitSitemap(accountID, "https://example.org/sitemap.xml")
	if err == nil {
		t.Errorf("expected error submitting sitemap for unverified domain, got nil")
	}

	// 3. Confirm with wrong token must fail
	_, err = mgr.ConfirmVerification(accountID, domain, "wrong-token")
	if err == nil {
		t.Errorf("expected mismatch error, got nil")
	}

	// 4. Confirm with correct token must succeed
	success, err := mgr.ConfirmVerification(accountID, domain, rec.ExpectedToken)
	if err != nil || !success {
		t.Fatalf("expected successful verification: %v", err)
	}

	// 5. Submit sitemap after verification must succeed
	sitemap, err := mgr.SubmitSitemap(accountID, "https://example.org/sitemap.xml")
	if err != nil {
		t.Fatalf("failed to submit sitemap after verification: %v", err)
	}
	if sitemap.Status != "processed" {
		t.Errorf("expected status 'processed', got %s", sitemap.Status)
	}
}

