package identity

import (
	"errors"
	"testing"
	"time"
)

func TestPasskeyMFARecoveryAndBOLADefense(t *testing.T) {
	store := NewStore()

	// 1. Passkey / OIDC Challenge Registration for Alice
	chAlice, err := store.CreateChallenge("passkey_register", "alice@example.org", "localhost")
	if err != nil {
		t.Fatalf("CreateChallenge failed: %v", err)
	}
	if _, _, _, _, err := store.CompleteChallenge(chAlice.ID, "wrong-signature", "Alice"); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("expected ErrInvalidChallenge on bad signature, got %v", err)
	}
	tokenAlice, sessAlice, accAlice, recoveryAlice, err := store.CompleteChallenge(chAlice.ID, "signed:"+chAlice.ChallengeText, "Alice")
	if err != nil {
		t.Fatalf("CompleteChallenge failed: %v", err)
	}
	if len(recoveryAlice) != 4 || accAlice.ID == "" || sessAlice.ID == "" {
		t.Fatalf("unexpected registration result: acc=%+v recovery=%v", accAlice, recoveryAlice)
	}
	// Replay of consumed challenge must fail
	if _, _, _, _, err := store.CompleteChallenge(chAlice.ID, "signed:"+chAlice.ChallengeText, "Alice"); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatal("consumed challenge replay must fail")
	}

	// 2. Register Bob for BOLA cross-account / cross-tenant testing
	chBob, err := store.CreateChallenge("passkey_register", "bob@example.org", "localhost")
	if err != nil {
		t.Fatal(err)
	}
	tokenBob, sessBob, accBob, _, err := store.CompleteChallenge(chBob.ID, "signed:"+chBob.ChallengeText, "Bob")
	if err != nil {
		t.Fatal(err)
	}

	// 3. Verify BOLA on preferences: Alice cannot read or update Bob's preferences
	if _, err := store.GetPreferences(accAlice.ID, accBob.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("BOLA: Alice read Bob's preferences: %v", err)
	}
	if _, err := store.UpdatePreferences(accAlice.ID, accBob.ID, Preferences{SafeSearch: "off", Theme: "dark", Language: "en"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("BOLA: Alice updated Bob's preferences: %v", err)
	}

	// 4. Verify opt-in recent searches: default is false, so recording does nothing
	store.RecordRecentSearch(accAlice.ID, "검색엔진")
	recent, err := store.GetRecentSearches(accAlice.ID, accAlice.ID)
	if err != nil || len(recent) != 0 {
		t.Fatalf("recent searches must not be stored when opt-in is false: %v", recent)
	}
	// Opt in and record
	_, err = store.UpdatePreferences(accAlice.ID, accAlice.ID, Preferences{
		SafeSearch:            "strict",
		Theme:                 "dark",
		Language:              "ko",
		RecentSearchesOptIn:   true,
		SearchCorrectionOptIn: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	store.RecordRecentSearch(accAlice.ID, "검색엔진")
	store.RecordRecentSearch(accAlice.ID, "오픈포털")
	recent, err = store.GetRecentSearches(accAlice.ID, accAlice.ID)
	if err != nil || len(recent) != 2 || recent[0] != "오픈포털" {
		t.Fatalf("unexpected recent searches after opt-in: %v", recent)
	}
	// Bob cannot read or clear Alice's recent searches (BOLA)
	if _, err := store.GetRecentSearches(accBob.ID, accAlice.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("BOLA: Bob read Alice's recent searches")
	}
	if err := store.ClearRecentSearches(accBob.ID, accAlice.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("BOLA: Bob cleared Alice's recent searches")
	}
	// Opting out immediately purges stored recent searches
	_, _ = store.UpdatePreferences(accAlice.ID, accAlice.ID, Preferences{
		SafeSearch:          "strict",
		Theme:               "dark",
		Language:            "ko",
		RecentSearchesOptIn: false,
	})
	recentAfterOptOut, _ := store.GetRecentSearches(accAlice.ID, accAlice.ID)
	if len(recentAfterOptOut) != 0 {
		t.Fatalf("recent searches must be purged on opt-out, got %v", recentAfterOptOut)
	}

	// 5. Verify BOLA on session revocation: Bob cannot revoke Alice's session
	if err := store.RevokeSessionByID(accBob.ID, sessAlice.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("BOLA: Bob revoked Alice's session: %v", err)
	}
	// Verify Bob can revoke his own session and token becomes invalid immediately
	if err := store.RevokeSession(tokenBob); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.VerifySession(tokenBob); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("revoked session token must be rejected")
	}
	_ = sessBob

	// 6. Verify BOLA on organization and publisher site access
	store.SeedOrganizationSite("org-alice", accAlice.ID, "owner", "site-alice-1", "https://alice.example.org")
	store.SeedOrganizationSite("org-bob", accBob.ID, "owner", "site-bob-1", "https://bob.example.org")
	if _, err := store.GetOrganizationSite(accAlice.ID, "org-alice", "site-alice-1"); err != nil {
		t.Fatalf("Alice should access own org site: %v", err)
	}
	if _, err := store.GetOrganizationSite(accAlice.ID, "org-bob", "site-bob-1"); !errors.Is(err, ErrForbidden) {
		t.Fatal("BOLA: Alice accessed Bob's org site")
	}
	if _, err := store.GetOrganizationSite(accAlice.ID, "org-alice", "site-bob-1"); !errors.Is(err, ErrForbidden) {
		t.Fatal("BOLA: Alice accessed Bob's site via org-alice")
	}

	// 7. MFA enrollment and TOTP verification
	secret, uri, err := store.EnrollMFA(accAlice.ID)
	if err != nil || secret == "" || uri == "" {
		t.Fatalf("EnrollMFA failed: %v", err)
	}
	if err := store.VerifyAndActivateMFA(tokenAlice, "000000"); !errors.Is(err, ErrInvalidMFA) {
		t.Fatal("invalid TOTP code must be rejected")
	}
	validCode, err := GenerateTOTP(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.VerifyAndActivateMFA(tokenAlice, validCode); err != nil {
		t.Fatalf("VerifyAndActivateMFA failed with valid code: %v", err)
	}

	// 8. Account recovery revokes existing sessions and single-use recovery code cannot be reused
	recoveredToken, _, err := store.RecoverAccount("alice@example.org", recoveryAlice[0])
	if err != nil || recoveredToken == "" {
		t.Fatalf("RecoverAccount failed: %v", err)
	}
	if _, _, err := store.VerifySession(tokenAlice); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old session must be revoked after account recovery")
	}
	if _, _, err := store.VerifySession(recoveredToken); err != nil {
		t.Fatalf("new recovered session must be valid: %v", err)
	}
	if _, _, err := store.RecoverAccount("alice@example.org", recoveryAlice[0]); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("consumed recovery code must not be reusable")
	}
}

