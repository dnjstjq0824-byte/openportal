package identity

import (
	"errors"
	"testing"
	"time"
)

func TestMFAReplayAndReplacementDenied(t *testing.T) {
	s := NewStoreWithVerifier(fixtureVerifier{})
	now := time.Unix(1800000000, 0).UTC()
	s.nowFunc = func() time.Time { return now }
	login := func() (string, *Account) {
		t.Helper()
		ch, err := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
		if err != nil {
			t.Fatal(err)
		}
		token, _, account, _, err := s.CompleteChallenge(ch.ID, "signed:"+ch.ChallengeText, "Alice")
		if err != nil {
			t.Fatal(err)
		}
		return token, account
	}
	token, account := login()
	secret, _, err := s.EnrollMFA(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := GenerateTOTP(secret, now)
	if err := s.VerifyAndActivateMFA(token, code); err != nil {
		t.Fatal(err)
	}
	secondToken, _ := login()
	if err := s.VerifyAndActivateMFA(secondToken, code); !errors.Is(err, ErrInvalidMFA) {
		t.Fatalf("TOTP replay accepted: %v", err)
	}
	if _, _, err := s.EnrollMFA(account.ID); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("existing MFA secret replacement allowed: %v", err)
	}
	now = now.Add(30 * time.Second)
	code, _ = GenerateTOTP(secret, now)
	if err := s.VerifyAndActivateMFA(secondToken, code); err != nil {
		t.Fatalf("fresh TOTP rejected: %v", err)
	}
}

func TestSessionExpiryBoundary(t *testing.T) {
	s := NewStoreWithVerifier(fixtureVerifier{})
	now := time.Unix(1800000000, 0).UTC()
	s.nowFunc = func() time.Time { return now }
	ch, _ := s.CreateChallenge("oidc_nonce", "alice@example.test", "localhost")
	token, session, _, _, err := s.CompleteChallenge(ch.ID, "signed:"+ch.ChallengeText, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	now = session.ExpiresAt
	if _, _, err := s.VerifySession(token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("session accepted at expiry boundary")
	}
}
