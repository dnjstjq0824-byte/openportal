package identity

import (
	"testing"
)

func TestLocalRegistrationAndAuthenticationFlow(t *testing.T) {
	store := NewStore()
	email := "testuser@open-scanner.org"
	password := "SecureP@ssw0rd2026!"
	displayName := "홍길동"

	// 1. Password shorter than 8 chars rejected
	if _, err := store.RegisterLocalAccount(email, "short", displayName); err != ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}

	// 2. Successful local registration (pending status)
	acc, err := store.RegisterLocalAccount(email, password, displayName)
	if err != nil {
		t.Fatalf("expected successful registration, got %v", err)
	}
	if acc.Email != email || acc.DisplayName != displayName || acc.Status != "pending_verification" {
		t.Fatalf("unexpected account state: %+v", acc)
	}

	// 3. Duplicate email registration rejected
	if _, err := store.RegisterLocalAccount(email, password, displayName); err != ErrAccountExists {
		t.Fatalf("expected ErrAccountExists, got %v", err)
	}

	// 4. Login before email verification fails
	if _, _, _, err := store.AuthenticateLocal(email, password); err != ErrEmailUnverified {
		t.Fatalf("expected ErrEmailUnverified, got %v", err)
	}

	// 5. Verify email
	if err := store.VerifyAccountEmail(email); err != nil {
		t.Fatalf("failed to verify email: %v", err)
	}

	// 6. Wrong password fails
	if _, _, _, err := store.AuthenticateLocal(email, "WrongPassword!"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// 7. Successful login returns valid session and bearer token
	sess, token, authedAcc, err := store.AuthenticateLocal(email, password)
	if err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}
	if sess == nil || len(token) == 0 || authedAcc.Email != email {
		t.Fatalf("invalid authentication result: sess=%+v, token=%s, acc=%+v", sess, token, authedAcc)
	}

	// 8. Session validation through VerifySession
	storedSess, storedAcc, err := store.VerifySession(token)
	if err != nil || storedSess.AccountID != acc.ID || storedAcc.ID != acc.ID {
		t.Fatalf("failed to retrieve valid session via token: %v", err)
	}

	// 9. Update password and verify new password works
	newPassword := "BrandNewP@ssw0rd2026!"
	if err := store.UpdatePassword(email, newPassword); err != nil {
		t.Fatalf("failed to update password: %v", err)
	}
	// Old password now fails
	if _, _, _, err := store.AuthenticateLocal(email, password); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for old password, got %v", err)
	}
	// New password succeeds
	_, newToken, _, err := store.AuthenticateLocal(email, newPassword)
	if err != nil || len(newToken) == 0 {
		t.Fatalf("failed to authenticate with updated password: %v", err)
	}
}
