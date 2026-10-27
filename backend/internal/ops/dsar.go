package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DSARType defines the type of Data Subject Request under GDPR/PIPA.
type DSARType string

const (
	DSARAccess  DSARType = "access"
	DSARExport  DSARType = "export"
	DSARErasure DSARType = "erasure"
	DSARRevoke  DSARType = "revoke_consent"
)

// DSARRequest tracks user privacy requests (Task 084).
type DSARRequest struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"account_id"`
	Type        DSARType   `json:"type"`
	Status      string     `json:"status"` // pending, completed, rejected
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ExportData  string     `json:"export_data,omitempty"`
}

// UserDataArchive bundles personal data for portability export.
type UserDataArchive struct {
	AccountID      string    `json:"account_id"`
	Email          string    `json:"email"`
	Preferences    any       `json:"preferences"`
	RecentSearches []string  `json:"recent_searches"`
	ExportedAt     time.Time `json:"exported_at"`
}

// DSARManager coordinates user privacy rights and data purging (Tasks 084, 085).
type DSARManager struct {
	mu       sync.RWMutex
	requests map[string]DSARRequest
}

// NewDSARManager creates a manager instance.
func NewDSARManager() *DSARManager {
	return &DSARManager{
		requests: make(map[string]DSARRequest),
	}
}

// SubmitRequest registers a new privacy request.
func (m *DSARManager) SubmitRequest(accountID string, reqType DSARType) (*DSARRequest, error) {
	if accountID == "" {
		return nil, errors.New("account_id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("dsar-%s-%d", accountID, time.Now().UnixNano())
	req := DSARRequest{
		ID:        id,
		AccountID: accountID,
		Type:      reqType,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	m.requests[id] = req
	return &req, nil
}

// ProcessExport packages the user's data into a portable archive.
func (m *DSARManager) ProcessExport(requestID string, archive UserDataArchive) (*DSARRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[requestID]
	if !exists {
		return nil, errors.New("dsar request not found")
	}

	dataBytes, err := json.Marshal(archive)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	req.Status = "completed"
	req.CompletedAt = &now
	req.ExportData = string(dataBytes)
	m.requests[requestID] = req
	return &req, nil
}

// ExecuteErasure coordinates the hard purge of account data and tombstone propagation (Task 085).
func (m *DSARManager) ExecuteErasure(requestID string, purgeHook func(accountID string) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[requestID]
	if !exists {
		return errors.New("dsar request not found")
	}

	if err := purgeHook(req.AccountID); err != nil {
		return fmt.Errorf("data purge failed: %w", err)
	}

	now := time.Now()
	req.Status = "completed"
	req.CompletedAt = &now
	m.requests[requestID] = req
	return nil
}
