package main

import (
	"context"
	"example.com/openportal-search/backend/internal/identity"
)

// HTTP policy fixtures do not stand in for a verified identity provider.
type fixtureVerifier struct{}

func (fixtureVerifier) Verify(_ context.Context, ch identity.Challenge, response string) (identity.Principal, error) {
	if response != "signed:"+ch.ChallengeText {
		return identity.Principal{}, identity.ErrInvalidChallenge
	}
	return identity.Principal{Issuer: "test-fixture", Subject: ch.Email, Email: ch.Email}, nil
}
