package ops

import (
	"testing"
)

func TestDualCustodyApprovalWorkflow(t *testing.T) {
	mgr := NewDualCustodyManager()
	operatorA := "acc-op-a"
	operatorB := "acc-op-b"

	// 1. Operator A creates a sensitive bulk export request
	req, err := mgr.CreateRequest(operatorA, "bulk_export", "all_accounts")
	if err != nil {
		t.Fatalf("failed creating request: %v", err)
	}
	if req.Status != "pending" {
		t.Errorf("expected pending status, got %s", req.Status)
	}

	// 2. Operator A attempts to self-approve -> must fail
	_, err = mgr.ApproveRequest(req.ID, operatorA)
	if err == nil {
		t.Errorf("expected dual-custody self-approval violation, got nil")
	}

	// 3. Operator B approves -> must succeed
	approved, err := mgr.ApproveRequest(req.ID, operatorB)
	if err != nil {
		t.Fatalf("expected approval from operator B, got %v", err)
	}
	if approved.Status != "approved" || approved.ApproverID != operatorB {
		t.Errorf("expected approved status by operator B, got %+v", approved)
	}
}

