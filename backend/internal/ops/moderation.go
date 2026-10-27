package ops

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// TakedownReason represents legal grounds for content removal.
type TakedownReason string

const (
	ReasonCopyrightDMCA  TakedownReason = "copyright_dmca"
	ReasonDefamation     TakedownReason = "defamation"
	ReasonPrivacyLeak    TakedownReason = "privacy_leak"
	ReasonCourtOrder     TakedownReason = "court_order"
	ReasonRightToForget  TakedownReason = "right_to_be_forgotten"
)

// TakedownRecord models an executed takedown action.
type TakedownRecord struct {
	ID             string         `json:"id"`
	DocumentURL    string         `json:"document_url"`
	Reason         TakedownReason `json:"reason"`
	RequesterEmail string         `json:"requester_email"`
	LegalBasis     string         `json:"legal_basis"`
	Tombstoned     bool           `json:"tombstoned"`
	IssuedAt       time.Time      `json:"issued_at"`
	AppealStatus   string         `json:"appeal_status"` // none, under_review, reinstated, rejected
}

// ModerationTakedownManager coordinates content removal and appeals (Task 083).
type ModerationTakedownManager struct {
	mu        sync.RWMutex
	takedowns map[string]TakedownRecord // key: document URL
}

// NewModerationTakedownManager creates a manager instance.
func NewModerationTakedownManager() *ModerationTakedownManager {
	return &ModerationTakedownManager{
		takedowns: make(map[string]TakedownRecord),
	}
}

// ExecuteTakedown records legal takedown and tombstones the document URL.
func (m *ModerationTakedownManager) ExecuteTakedown(docURL string, reason TakedownReason, requester, legalBasis string) (*TakedownRecord, error) {
	if docURL == "" || requester == "" || legalBasis == "" {
		return nil, errors.New("document_url, requester, and legal_basis are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("td-%d", time.Now().UnixNano())
	rec := TakedownRecord{
		ID:             id,
		DocumentURL:    docURL,
		Reason:         reason,
		RequesterEmail: requester,
		LegalBasis:     legalBasis,
		Tombstoned:     true,
		IssuedAt:       time.Now(),
		AppealStatus:   "none",
	}

	m.takedowns[docURL] = rec
	return &rec, nil
}

// IsTombstoned checks if an URL is removed under legal takedown.
func (m *ModerationTakedownManager) IsTombstoned(docURL string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, exists := m.takedowns[docURL]
	return exists && rec.Tombstoned
}

// SubmitAppeal records a counter-notice or appeal from the publisher.
func (m *ModerationTakedownManager) SubmitAppeal(docURL, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, exists := m.takedowns[docURL]
	if !exists {
		return errors.New("takedown record not found for URL")
	}

	rec.AppealStatus = "under_review"
	m.takedowns[docURL] = rec
	return nil
}

