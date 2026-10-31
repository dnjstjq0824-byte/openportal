package identity

import (
	"testing"
)

func TestPKCES256Verification(t *testing.T) {
	verifier := "openportal-test-code-verifier-string-length-at-least-43-chars-long"
	challenge, err := GeneratePKCES256Challenge(verifier)
	if err != nil {
		t.Fatalf("unexpected error generating challenge: %v", err)
	}
	if err := VerifyPKCES256(verifier, challenge); err != nil {
		t.Fatalf("expected valid PKCE S256 verification, got %v", err)
	}
	if err := VerifyPKCES256("short-verifier", challenge); err != ErrInvalidPKCEVerifier {
		t.Fatalf("expected ErrInvalidPKCEVerifier for short verifier, got %v", err)
	}
	if err := VerifyPKCES256(verifier+"tampered", challenge); err != ErrPKCEChallengeMismatch {
		t.Fatalf("expected ErrPKCEChallengeMismatch for tampered verifier, got %v", err)
	}
}

func TestValidateFIDO2RPAssertion(t *testing.T) {
	// Valid ES256 assertion with monotonic counter and user verification
	if err := ValidateFIDO2RPAssertion("openportal.org", "openportal.org", COSEAlgES256, 10, 11, true, true); err != nil {
		t.Fatalf("expected valid assertion, got %v", err)
	}
	// Invalid RP ID (phishing origin)
	if err := ValidateFIDO2RPAssertion("openportal.org", "evil-phishing.org", COSEAlgES256, 10, 11, true, false); err != ErrInvalidFIDO2RPID {
		t.Fatalf("expected ErrInvalidFIDO2RPID, got %v", err)
	}
	// Unsupported COSE algorithm (-65535 RS1)
	if err := ValidateFIDO2RPAssertion("openportal.org", "openportal.org", -65535, 10, 11, true, false); err != ErrUnsupportedCOSEAlg {
		t.Fatalf("expected ErrUnsupportedCOSEAlg, got %v", err)
	}
	// Cloned authenticator (counter regression)
	if err := ValidateFIDO2RPAssertion("openportal.org", "openportal.org", COSEAlgEdDSA, 15, 15, true, false); err != ErrClonedAuthenticator {
		t.Fatalf("expected ErrClonedAuthenticator, got %v", err)
	}
	// Missing User Verification when required for /operator PAM
	if err := ValidateFIDO2RPAssertion("openportal.org", "openportal.org", COSEAlgRS256, 1, 2, false, true); err != ErrUserVerificationReq {
		t.Fatalf("expected ErrUserVerificationReq, got %v", err)
	}
}

func TestMapEnterpriseIdPRoles(t *testing.T) {
	roles := MapEnterpriseIdPRoles([]string{"OpenPortal-Search-Operators", "secops-audit", "unrelated-group"})
	if len(roles) != 2 || roles[0] != "SearchOperator" || roles[1] != "SecurityAuditor" {
		t.Fatalf("unexpected mapped roles: %+v", roles)
	}
}

