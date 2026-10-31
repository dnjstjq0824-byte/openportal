package email

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestEmailServiceRegistrationFlow(t *testing.T) {
	memSender := NewMemorySender()
	svc := NewService(memSender, "test@open-scanner.org")

	ctx := context.Background()
	email := "user@example.org"
	dispName := "홍길동"

	// 1. Send registration verification email
	tok, err := svc.SendRegistrationVerification(ctx, email, dispName)
	if err != nil {
		t.Fatalf("failed to send registration verification: %v", err)
	}
	if tok.Email != email || len(tok.Code) != 6 {
		t.Fatalf("invalid token data: %+v", tok)
	}

	// 2. Verify email delivered to MemorySender
	latest := memSender.LatestMessageFor(email)
	if latest == nil {
		t.Fatalf("expected message in sender, got nil")
	}
	if !strings.Contains(latest.Subject, tok.Code) {
		t.Fatalf("subject does not contain PIN code: %s", latest.Subject)
	}
	if !strings.Contains(latest.HTMLBody, tok.Code) {
		t.Fatalf("HTML body does not contain PIN code")
	}

	// 3. Test verification with wrong code -> fail
	if err := svc.VerifyRegistrationToken(email, "000000"); err != ErrTokenMismatch {
		t.Fatalf("expected ErrTokenMismatch, got %v", err)
	}

	// 4. Test verification with correct code -> pass
	if err := svc.VerifyRegistrationToken(email, tok.Code); err != nil {
		t.Fatalf("expected success verifying token, got %v", err)
	}

	// 5. Re-verify consumed token -> fail
	if err := svc.VerifyRegistrationToken(email, tok.Code); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound on consumed token, got %v", err)
	}
}

func TestEmailServicePasswordResetFlow(t *testing.T) {
	memSender := NewMemorySender()
	svc := NewService(memSender, "test@open-scanner.org")

	ctx := context.Background()
	email := "secops@open-scanner.org"

	// 1. Request password reset
	reset, err := svc.SendPasswordReset(ctx, email)
	if err != nil {
		t.Fatalf("failed to send password reset: %v", err)
	}
	if reset.Email != email || len(reset.Token) == 0 {
		t.Fatalf("invalid reset token: %+v", reset)
	}

	// 2. Check message in memory
	latest := memSender.LatestMessageFor(email)
	if latest == nil || !strings.Contains(latest.HTMLBody, reset.Token) {
		t.Fatalf("reset message missing token")
	}

	// 3. Verify reset token
	if err := svc.VerifyPasswordResetToken(email, reset.Token); err != nil {
		t.Fatalf("expected success verifying reset token, got %v", err)
	}

	// 4. Verify consumed token fails
	if err := svc.VerifyPasswordResetToken(email, reset.Token); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestEmailServiceRateLimiting(t *testing.T) {
	memSender := NewMemorySender()
	svc := NewService(memSender, "test@open-scanner.org")

	ctx := context.Background()
	email := "burst@open-scanner.org"

	for i := 0; i < 5; i++ {
		_, err := svc.SendRegistrationVerification(ctx, email, "User")
		if err != nil {
			t.Fatalf("expected success on send %d, got %v", i, err)
		}
	}

	// 6th send should hit rate limit
	_, err := svc.SendRegistrationVerification(ctx, email, "User")
	if err != ErrRateLimitExceeded {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
	}
}

func TestEmailServiceTokenExpiration(t *testing.T) {
	memSender := NewMemorySender()
	svc := NewService(memSender, "test@open-scanner.org")
	svc.tokenLifetime = -1 * time.Second // already expired

	ctx := context.Background()
	email := "expired@open-scanner.org"

	tok, err := svc.SendRegistrationVerification(ctx, email, "User")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if err := svc.VerifyRegistrationToken(email, tok.Code); err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

