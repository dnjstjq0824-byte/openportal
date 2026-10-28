package identity

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Principal is returned only after the provider's cryptographic proof is checked.
type Principal struct {
	Issuer, Subject, Email string
}

type ChallengeVerifier interface {
	Verify(context.Context, Challenge, string) (Principal, error)
}

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
	clientID string
}

// NewOIDCVerifier discovers only an operator-configured HTTPS issuer.
// No issuer, audience or signing key is accepted from a login request.
func NewOIDCVerifier(ctx context.Context, issuer, clientID string) (*OIDCVerifier, error) {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return newOIDCVerifier(ctx, issuer, clientID, client)
}

func newOIDCVerifier(ctx context.Context, issuer, clientID string, client *http.Client) (*OIDCVerifier, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(clientID) == "" {
		return nil, ErrInvalidInput
	}
	ctx = oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if provider.Claims(&metadata) != nil {
		return nil, ErrInvalidInput
	}
	keysURL, err := url.Parse(metadata.JWKSURL)
	if err != nil || keysURL.Scheme != "https" || keysURL.Host == "" || keysURL.User != nil || keysURL.Fragment != "" {
		return nil, ErrInvalidInput
	}
	return &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: clientID, SupportedSigningAlgs: []string{oidc.RS256, oidc.ES256}}), clientID: clientID}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, ch Challenge, raw string) (Principal, error) {
	if v == nil || v.verifier == nil || ch.ChallengeType != "oidc_nonce" || ch.ChallengeText == "" || len(raw) > 64*1024 {
		return Principal{}, ErrInvalidChallenge
	}
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil || token.Subject == "" || subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(ch.ChallengeText)) != 1 {
		return Principal{}, ErrInvalidChallenge
	}
	var claims struct {
		Email           string `json:"email"`
		EmailVerified   bool   `json:"email_verified"`
		AuthorizedParty string `json:"azp"`
	}
	if token.Claims(&claims) != nil || !claims.EmailVerified || claims.Email == "" || !strings.EqualFold(claims.Email, ch.Email) ||
		(len(token.Audience) > 1 && claims.AuthorizedParty == "") || (claims.AuthorizedParty != "" && claims.AuthorizedParty != v.clientID) {
		return Principal{}, ErrInvalidChallenge
	}
	return Principal{Issuer: token.Issuer, Subject: token.Subject, Email: strings.ToLower(claims.Email)}, nil
}

// NewStoreWithVerifier injects a trusted server-side verifier. NewStore alone
// cannot authenticate; persistent storage and browser login are separate gates.
func NewStoreWithVerifier(verifier ChallengeVerifier) *Store {
	s := NewStore()
	s.verifier = verifier
	return s
}
