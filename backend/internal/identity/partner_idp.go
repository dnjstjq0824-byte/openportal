package identity

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
)

var (
	ErrInvalidPKCEVerifier   = errors.New("pkce: code_verifier must be 43..128 RFC 7636 characters")
	ErrPKCEChallengeMismatch = errors.New("pkce: S256 code_challenge mismatch")
	ErrInvalidFIDO2RPID      = errors.New("webauthn: relying party ID mismatch")
	ErrUnsupportedCOSEAlg    = errors.New("webauthn: unsupported COSE algorithm (allowed: -7 ES256, -8 EdDSA, -257 RS256)")
	ErrClonedAuthenticator   = errors.New("webauthn: signature counter regression detected (possible cloned hardware token)")
	ErrUserVerificationReq   = errors.New("webauthn: user verification (biometric/PIN) required for privileged operation")
)

const (
	COSEAlgES256 = -7
	COSEAlgEdDSA = -8
	COSEAlgRS256 = -257
)

// VerifyPKCES256 validates an OAuth 2.0 / OIDC PKCE (RFC 7636) S256 code_verifier
// against a previously issued code_challenge in constant time.
func VerifyPKCES256(codeVerifier, codeChallenge string) error {
	v := strings.TrimSpace(codeVerifier)
	if len(v) < 43 || len(v) > 128 {
		return ErrInvalidPKCEVerifier
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		isAlphaNum := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		isReserved := c == '-' || c == '.' || c == '_' || c == '~'
		if !isAlphaNum && !isReserved {
			return ErrInvalidPKCEVerifier
		}
	}
	sum := sha256.Sum256([]byte(v))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(computed), []byte(strings.TrimSpace(codeChallenge))) != 1 {
		return ErrPKCEChallengeMismatch
	}
	return nil
}

// GeneratePKCES256Challenge computes the S256 code_challenge for a valid code_verifier.
func GeneratePKCES256Challenge(codeVerifier string) (string, error) {
	v := strings.TrimSpace(codeVerifier)
	if len(v) < 43 || len(v) > 128 {
		return "", ErrInvalidPKCEVerifier
	}
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// ValidateFIDO2RPAssertion validates WebAuthn FIDO2 Relying Party parameters,
// COSE algorithm whitelist, signature counter monotonicity, and User Verification.
func ValidateFIDO2RPAssertion(expectedRPID, actualRPID string, coseAlg int, prevSignCount, newSignCount uint32, userVerified, requireUV bool) error {
	if strings.TrimSpace(expectedRPID) == "" || !strings.EqualFold(strings.TrimSpace(expectedRPID), strings.TrimSpace(actualRPID)) {
		return ErrInvalidFIDO2RPID
	}
	if coseAlg != COSEAlgES256 && coseAlg != COSEAlgEdDSA && coseAlg != COSEAlgRS256 {
		return ErrUnsupportedCOSEAlg
	}
	if prevSignCount > 0 && newSignCount <= prevSignCount {
		return ErrClonedAuthenticator
	}
	if requireUV && !userVerified {
		return ErrUserVerificationReq
	}
	return nil
}

// MapEnterpriseIdPRoles maps external OIDC / Entra ID enterprise directory groups
// to least-privilege internal OpenPortal operator roles.
func MapEnterpriseIdPRoles(idpGroups []string) []string {
	roleSet := make(map[string]bool)
	for _, g := range idpGroups {
		norm := strings.ToLower(strings.TrimSpace(g))
		switch norm {
		case "openportal-search-operators", "sre-search-ops", "searchoperator":
			roleSet["SearchOperator"] = true
		case "openportal-security-auditors", "secops-audit", "securityauditor":
			roleSet["SecurityAuditor"] = true
		case "openportal-billing-managers", "finops-ads-billing", "billingmanager":
			roleSet["BillingManager"] = true
		}
	}
	roles := make([]string, 0, len(roleSet))
	for _, canonical := range []string{"SearchOperator", "SecurityAuditor", "BillingManager"} {
		if roleSet[canonical] {
			roles = append(roles, canonical)
		}
	}
	return roles
}

