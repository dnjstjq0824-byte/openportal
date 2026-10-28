package identity

import "context"

// Policy tests use an explicit fixture verifier; this is not authentication evidence.
type fixtureVerifier struct{}

func (fixtureVerifier) Verify(_ context.Context, ch Challenge, response string) (Principal, error) {
	if response != "signed:"+ch.ChallengeText {
		return Principal{}, ErrInvalidChallenge
	}
	return Principal{Issuer: "test-fixture", Subject: ch.Email, Email: ch.Email}, nil
}
