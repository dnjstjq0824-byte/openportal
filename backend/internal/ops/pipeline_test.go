package ops

import (
	"testing"
	"time"
)

func TestPhase8EndToEndOpsPipeline(t *testing.T) {
	// 1. PAM JIT Elevation
	pamMgr := NewPAMManager(1 * time.Hour)
	elev, err := pamMgr.RequestElevation("acc-admin", RoleSecurityAuditor, "Quarterly compliance review", true, 30*time.Minute)
	if err != nil {
		t.Fatalf("pam elevation failed: %v", err)
	}
	active, _ := pamMgr.IsElevationActive(elev.ID)
	if !active {
		t.Fatalf("elevation not active")
	}

	// 2. DSAR Request & Erasure
	dsarMgr := NewDSARManager()
	dsarReq, err := dsarMgr.SubmitRequest("acc-user-gdpr", DSARErasure)
	if err != nil {
		t.Fatalf("dsar submit failed: %v", err)
	}
	purged := false
	err = dsarMgr.ExecuteErasure(dsarReq.ID, func(acc string) error {
		purged = true
		return nil
	})
	if err != nil || !purged {
		t.Fatalf("dsar erasure execution failed")
	}

	// 3. Moderation & Takedown
	modMgr := NewModerationTakedownManager()
	docURL := "https://example.org/copyrighted-article"
	tdRec, err := modMgr.ExecuteTakedown(docURL, ReasonCopyrightDMCA, "dmca@owner.org", "DMCA Section 512")
	if err != nil || !tdRec.Tombstoned {
		t.Fatalf("moderation takedown failed")
	}
	if !modMgr.IsTombstoned(docURL) {
		t.Fatalf("document not tombstoned")
	}

	// 4. Automated Pentest Checks
	resSSRF := EvaluatePentestPayload(VectorSSRF, "http://169.254.169.254/")
	if !resSSRF.Blocked {
		t.Fatalf("pentest SSRF failed to block payload")
	}
	resSQLi := EvaluatePentestPayload(VectorSQLInjection, "' UNION SELECT 1,2,3 --")
	if !resSQLi.Blocked {
		t.Fatalf("pentest SQLi failed to block payload")
	}

	// 5. Dual Custody 4-Eyes Principle
	dualMgr := NewDualCustodyManager()
	dReq, err := dualMgr.CreateRequest("operator-1", "index_purge", "all_indexes")
	if err != nil {
		t.Fatalf("dual custody request creation failed: %v", err)
	}
	// Self-approval must fail
	if _, err := dualMgr.ApproveRequest(dReq.ID, "operator-1"); err == nil {
		t.Fatalf("dual custody self-approval should have failed")
	}
	// Distinct operator must succeed
	if _, err := dualMgr.ApproveRequest(dReq.ID, "operator-2"); err != nil {
		t.Fatalf("dual custody approval failed: %v", err)
	}

	// 6. Release Gate Evaluation (Technical PASS, Third-party pentest audit pending -> NO-GO)
	checklist := OpsAuditChecklist{
		PAMRoleElevationPassed:        true,
		DSARPrivacyWorkflowPassed:     true,
		ModerationTakedownPassed:      true,
		AutomatedPentestVectorsPassed: true,
		DualCustodyEnforced:           true,
		ZeroCriticalVulnerabilities:   true,
		ThirdPartyPentestAuditSigned:  false, // Pending external auditor
		LegalPrivacyComplianceSigned:  false,
	}
	report := EvaluatePhase8ReleaseGate(checklist)
	if report.Decision != GateDecisionNoGo {
		t.Fatalf("expected GateDecisionNoGo due to missing third-party pentest audit, got %s", report.Decision)
	}
	if len(report.UnresolvedRisks) != 2 {
		t.Fatalf("expected 2 unresolved risks, got %d", len(report.UnresolvedRisks))
	}
}

