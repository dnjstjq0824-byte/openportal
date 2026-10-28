package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

func signedIDToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	hash := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestOIDCProofAndSession(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const issuer = "https://identity.example.test"
	const client = "openportal"
	verifier := &OIDCVerifier{clientID: client, verifier: oidc.NewVerifier(issuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: client, SupportedSigningAlgs: []string{oidc.RS256}})}
	claimsFor := func(ch *Challenge) map[string]any {
		return map[string]any{"iss": issuer, "sub": "subject-1", "aud": client, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": ch.ChallengeText, "email": ch.Email, "email_verified": true}
	}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"issuer", func(c map[string]any) { c["iss"] = "https://attacker.example.test" }},
		{"audience", func(c map[string]any) { c["aud"] = "other-client" }},
		{"expired", func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{"nonce", func(c map[string]any) { c["nonce"] = "wrong" }},
		{"subject", func(c map[string]any) { delete(c, "sub") }},
		{"unverified-email", func(c map[string]any) { c["email_verified"] = false }},
		{"email-mismatch", func(c map[string]any) { c["email"] = "victim@example.test" }},
		{"azp", func(c map[string]any) { c["azp"] = "other-client" }},
		{"multi-audience", func(c map[string]any) { c["aud"] = []string{client, "other-client"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStoreWithVerifier(verifier)
			ch, _ := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
			claims := claimsFor(ch)
			tc.mutate(claims)
			if _, _, _, _, err := s.CompleteChallenge(ch.ID, signedIDToken(t, key, claims), "Alice"); !errors.Is(err, ErrInvalidChallenge) {
				t.Fatalf("invalid proof accepted: %v", err)
			}
			if len(s.accounts) != 0 || len(s.sessions) != 0 {
				t.Fatal("invalid proof changed identity state")
			}
		})
	}
	s := NewStoreWithVerifier(verifier)
	ch, _ := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
	for _, raw := range []string{"signed:" + ch.ChallengeText, signedIDToken(t, otherKey, claimsFor(ch)), strings.Repeat("x", 65537)} {
		if _, _, _, _, err := s.CompleteChallenge(ch.ID, raw, "Alice"); !errors.Is(err, ErrInvalidChallenge) {
			t.Fatalf("forged proof accepted: %v", err)
		}
	}
	raw := signedIDToken(t, key, claimsFor(ch))
	token, _, acc, _, err := s.CompleteChallenge(ch.ID, raw, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, got, err := s.VerifySession(token); err != nil || got.ID != acc.ID {
		t.Fatal("verified proof did not produce a usable session")
	}
	if _, _, _, _, err := s.CompleteChallenge(ch.ID, raw, "Alice"); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatal("replay accepted")
	}

	// A provider subject cannot take over an account by sharing its email.
	collision, _ := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
	claims := claimsFor(collision)
	claims["sub"] = "other-subject"
	if _, _, _, _, err := s.CompleteChallenge(collision.ID, signedIDToken(t, key, claims), "Attacker"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("email linked without authorization: %v", err)
	}

	// Only one concurrent completion may consume the same valid proof.
	concurrent, _ := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
	raw = signedIDToken(t, key, claimsFor(concurrent))
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, _, _, err := s.CompleteChallenge(concurrent.ID, raw, "Alice"); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("proof consumed %d times", successes.Load())
	}
	if err := s.RevokeSession(token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.VerifySession(token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("revoked session accepted")
	}
}

func TestUnconfiguredAuthenticationFailsClosed(t *testing.T) {
	s := NewStore()
	ch, _ := s.CreateChallenge("passkey_register", "alice@example.test", "localhost")
	if _, _, _, _, err := s.CompleteChallenge(ch.ID, "signed:"+ch.ChallengeText, "Alice"); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatal("unconfigured store authenticated")
	}
	for _, issuer := range []string{"http://localhost", "https://user:pass@example.test", "https://example.test?issuer=other", "https://example.test#fragment"} {
		if _, err := NewOIDCVerifier(context.Background(), issuer, "client"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid issuer accepted: %s", issuer)
		}
	}
}

func TestOIDCDiscoveryAndRemoteKeys(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer string
	var keyReads atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			keyReads.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuer = server.URL
	verifier, err := newOIDCVerifier(context.Background(), issuer, "openportal", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	s := NewStoreWithVerifier(verifier)
	ch, _ := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
	raw := signedIDToken(t, key, map[string]any{"iss": issuer, "sub": "alice-id", "aud": "openportal", "exp": time.Now().Add(time.Hour).Unix(), "nonce": ch.ChallengeText, "email": ch.Email, "email_verified": true})
	if _, _, _, _, err := s.CompleteChallenge(ch.ID, raw, "Alice"); err != nil {
		t.Fatal(err)
	}
	if keyReads.Load() == 0 {
		t.Fatal("provider signing keys were not retrieved")
	}
}
