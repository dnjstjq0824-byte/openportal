package ops

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// PrivilegedRole defines administrative high-privilege roles.
type PrivilegedRole string

const (
	RoleAdminSuper       PrivilegedRole = "admin_super"
	RoleSecurityAuditor  PrivilegedRole = "security_auditor"
	RoleDatabaseOperator PrivilegedRole = "db_operator"
	RoleComplianceOfficer PrivilegedRole = "compliance_officer"
)

// ElevationRequest represents a Just-In-Time role elevation request (Task 081).
type ElevationRequest struct {
	ID          string         `json:"id"`
	AccountID   string         `json:"account_id"`
	Role        PrivilegedRole `json:"role"`
	Reason      string         `json:"reason"`
	MFAVerified bool           `json:"mfa_verified"`
	GrantedAt   time.Time      `json:"granted_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
	Revoked     bool           `json:"revoked"`
}

// PAMManager handles time-bound privileged role elevations.
type PAMManager struct {
	mu         sync.RWMutex
	elevations map[string]ElevationRequest // key: elevation ID
	maxTTL     time.Duration
}

// NewPAMManager initializes a Privileged Access Management coordinator with maximum TTL (e.g. 1 hour).
func NewPAMManager(maxTTL time.Duration) *PAMManager {
	if maxTTL <= 0 {
		maxTTL = 1 * time.Hour
	}
	return &PAMManager{
		elevations: make(map[string]ElevationRequest),
		maxTTL:     maxTTL,
	}
}

// RequestElevation grants a time-bound privileged elevation if MFA is verified and rationale is provided.
func (m *PAMManager) RequestElevation(accountID string, role PrivilegedRole, reason string, mfaVerified bool, ttl time.Duration) (*ElevationRequest, error) {
	if accountID == "" || stringsTrim(reason) == "" {
		return nil, errors.New("account_id and valid emergency reason are required")
	}
	if !mfaVerified {
		return nil, errors.New("privileged elevation requires confirmed MFA verification")
	}
	if ttl <= 0 || ttl > m.maxTTL {
		ttl = m.maxTTL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("elev-%s-%d", accountID, now.UnixNano())
	req := ElevationRequest{
		ID:          id,
		AccountID:   accountID,
		Role:        role,
		Reason:      reason,
		MFAVerified: true,
		GrantedAt:   now,
		ExpiresAt:   now.Add(ttl),
		Revoked:     false,
	}

	m.elevations[id] = req
	return &req, nil
}

// IsElevationActive verifies that an elevation is valid and has not expired or been revoked.
func (m *PAMManager) IsElevationActive(elevationID string) (bool, PrivilegedRole) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, exists := m.elevations[elevationID]
	if !exists || req.Revoked {
		return false, ""
	}

	if time.Now().After(req.ExpiresAt) {
		return false, "" // Automatically expired
	}

	return true, req.Role
}

// RevokeElevation terminates a privileged elevation immediately.
func (m *PAMManager) RevokeElevation(elevationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.elevations[elevationID]
	if !exists {
		return errors.New("elevation record not found")
	}

	req.Revoked = true
	m.elevations[elevationID] = req
	return nil
}

func stringsTrim(s string) string {
	var start, end int
	for start = 0; start < len(s); start++ {
		if s[start] != ' ' && s[start] != '\t' && s[start] != '\n' && s[start] != '\r' {
			break
		}
	}
	for end = len(s); end > start; end-- {
		if s[end-1] != ' ' && s[end-1] != '\t' && s[end-1] != '\n' && s[end-1] != '\r' {
			break
		}
	}
	return s[start:end]
}

