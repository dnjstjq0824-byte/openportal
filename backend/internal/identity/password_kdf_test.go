package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// legacyHashFixture rebuilds the pre-standard KDF output for migration tests.
// It mirrors the retired hashPasswordWithSalt construction byte-for-byte.
func legacyHashFixture(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt + password))
	sum := h.Sum(nil)
	for i := 0; i < 5000; i++ {
		h.Reset()
		h.Write(sum)
		h.Write([]byte(salt))
		sum = h.Sum(nil)
	}
	return fmt.Sprintf("pbkdf_sha256$%s$%s", salt, hex.EncodeToString(sum))
}

func TestPasswordHashIsBcrypt(t *testing.T) {
	encoded, err := hashPasswordWithSalt("SecureP@ssw0rd2026!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "bcrypt$") {
		t.Fatalf("expected bcrypt$ hash, got %.12s", encoded)
	}
	if !verifyPasswordHash("SecureP@ssw0rd2026!", encoded) {
		t.Fatalf("bcrypt hash does not verify")
	}
	if verifyPasswordHash("WrongPassword!", encoded) {
		t.Fatalf("wrong password verified")
	}
	if _, err := hashPasswordWithSalt("short"); err != ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword for short password, got %v", err)
	}
	if _, err := hashPasswordWithSalt(strings.Repeat("x", 73)); err != ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword for >72 byte password, got %v", err)
	}
	if verifyPasswordHash("anything", "not-a-hash") {
		t.Fatalf("malformed encoding verified")
	}
	if verifyPasswordHash("anything", "bcrypt$not-valid-bcrypt") {
		t.Fatalf("corrupt bcrypt encoding verified")
	}
}

func TestLegacyHashMigratesOnLogin(t *testing.T) {
	store := NewStore()
	email := "legacy@open-scanner.org"
	password := "LegacyP@ssw0rd2026!"
	acc, err := store.RegisterLocalAccount(email, password, "Legacy User")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := store.VerifyAccountEmail(email); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Plant a pre-standard hash as if the account predates the migration.
	store.passwordHashes[acc.ID] = legacyHashFixture(password, "0123456789abcdef0123456789abcdef")
	if _, _, _, err := store.AuthenticateLocal(email, password); err != nil {
		t.Fatalf("legacy login: %v", err)
	}
	migrated := store.passwordHashes[acc.ID]
	if !strings.HasPrefix(migrated, "bcrypt$") {
		t.Fatalf("legacy hash was not migrated, got %.14s", migrated)
	}
	if _, _, _, err := store.AuthenticateLocal(email, password); err != nil {
		t.Fatalf("post-migration login: %v", err)
	}
	if _, _, _, err := store.AuthenticateLocal(email, "WrongPassword!"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}
