package ops

import (
	"testing"
	"time"
)

func TestPAMManagerLifecycle(t *testing.T) {
	mgr := NewPAMManager(30 * time.Minute)
	accountID := "acc-admin-1"

	// 1. Unverified MFA must fail
	_, err := mgr.RequestElevation(accountID, RoleAdminSuper, "Emergency incident response", false, 15*time.Minute)
	if err == nil {
		t.Errorf("expected MFA failure, got nil")
	}

	// 2. Verified MFA must succeed
	elev, err := mgr.RequestElevation(accountID, RoleAdminSuper, "Emergency incident response", true, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Active check
	active, role := mgr.IsElevationActive(elev.ID)
	if !active || role != RoleAdminSuper {
		t.Errorf("expected active elevation, got active=%v, role=%s", active, role)
	}

	// 3. Expiration check after TTL (100ms)
	time.Sleep(150 * time.Millisecond)
	activeAfter, _ := mgr.IsElevationActive(elev.ID)
	if activeAfter {
		t.Errorf("expected elevation to expire after TTL")
	}

	// 4. Explicit revocation check
	elev2, _ := mgr.RequestElevation(accountID, RoleSecurityAuditor, "Compliance check", true, 10*time.Minute)
	if err := mgr.RevokeElevation(elev2.ID); err != nil {
		t.Fatalf("failed to revoke: %v", err)
	}
	activeRevoked, _ := mgr.IsElevationActive(elev2.ID)
	if activeRevoked {
		t.Errorf("expected revoked elevation to be inactive")
	}
}

