package ops

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// DualApprovalRequest models a high-impact operation requiring 4-eyes authorization (Task 089).
type DualApprovalRequest struct {
	ID          string     `json:"id"`
	ActionType  string     `json:"action_type"` // "bulk_export", "index_purge", "key_rotation"
	Payload     string     `json:"payload"`
	RequesterID string     `json:"requester_id"`
	ApproverID  string     `json:"approver_id,omitempty"`
	Status      string     `json:"status"` // pending, approved, rejected
	CreatedAt   time.Time  `json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
}

// DualCustodyManager enforces that no single administrator can execute high-risk operations unilaterally.
type DualCustodyManager struct {
	mu        sync.RWMutex
	approvals map[string]DualApprovalRequest
}

// NewDualCustodyManager creates a new manager instance.
func NewDualCustodyManager() *DualCustodyManager {
	return &DualCustodyManager{
		approvals: make(map[string]DualApprovalRequest),
	}
}

// CreateRequest initiates a 4-eyes approval workflow.
func (m *DualCustodyManager) CreateRequest(requesterID, actionType, payload string) (*DualApprovalRequest, error) {
	if requesterID == "" || actionType == "" {
		return nil, errors.New("requester_id and action_type are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("dual-%d", time.Now().UnixNano())
	req := DualApprovalRequest{
		ID:          id,
		ActionType:  actionType,
		Payload:     payload,
		RequesterID: requesterID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	m.approvals[id] = req
	return &req, nil
}

// ApproveRequest approves a pending action.
// CRITICAL: Requester cannot approve their own action (RequesterID != ApproverID).
func (m *DualCustodyManager) ApproveRequest(requestID, approverID string) (*DualApprovalRequest, error) {
	if approverID == "" {
		return nil, errors.New("approver_id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.approvals[requestID]
	if !exists {
		return nil, errors.New("request not found")
	}

	if req.Status != "pending" {
		return nil, fmt.Errorf("cannot approve request in %s state", req.Status)
	}

	// 4-eyes principle check
	if req.RequesterID == approverID {
		return nil, errors.New("dual custody violation: requester cannot approve their own operation")
	}

	now := time.Now()
	req.ApproverID = approverID
	req.Status = "approved"
	req.ApprovedAt = &now
	m.approvals[requestID] = req

	return &req, nil
}
