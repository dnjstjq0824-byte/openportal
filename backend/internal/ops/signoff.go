package ops

import (
	"fmt"
	"time"
)

// GateDecision represents binary Go / No-Go determination for Phase 8.
type GateDecision string

const (
	GateDecisionGo   GateDecision = "GO"
	GateDecisionNoGo GateDecision = "NO-GO"
)

// OpsAuditChecklist tracks compliance across all Phase 8 operational security criteria.
type OpsAuditChecklist struct {
	PAMRoleElevationPassed          bool `json:"pam_role_elevation_passed"`
	DSARPrivacyWorkflowPassed       bool `json:"dsar_privacy_workflow_passed"`
	ModerationTakedownPassed        bool `json:"moderation_takedown_passed"`
	AutomatedPentestVectorsPassed   bool `json:"automated_pentest_vectors_passed"`
	DualCustodyEnforced             bool `json:"dual_custody_enforced"`
	ZeroCriticalVulnerabilities     bool `json:"zero_critical_vulnerabilities"`
	ThirdPartyPentestAuditSigned    bool `json:"third_party_pentest_audit_signed"`
	LegalPrivacyComplianceSigned    bool `json:"legal_privacy_compliance_signed"`
}

// Phase8GateReport details the audit evaluation for operational security.
type Phase8GateReport struct {
	Phase           int               `json:"phase"`
	EvaluatedAt     time.Time         `json:"evaluated_at"`
	Decision        GateDecision      `json:"decision"`
	GateReason      string            `json:"gate_reason"`
	Checklist       OpsAuditChecklist `json:"checklist"`
	UnresolvedRisks []string          `json:"unresolved_risks"`
}

// EvaluatePhase8ReleaseGate evaluates Phase 8 readiness:
// Adhering strictly to AGENTS.md rule: "P0/P1 제로, 법무 검토, 독립 보안검수 승인 서명을 요구한다... mark blocked work as blocked."
// Because live production operations require formal third-party penetration testing and legal privacy review sign-offs,
// the gate evaluates strictly and marks unapproved audit items as blocked (NO-GO).
func EvaluatePhase8ReleaseGate(checklist OpsAuditChecklist) Phase8GateReport {
	report := Phase8GateReport{
		Phase:       8,
		EvaluatedAt: time.Now(),
		Checklist:   checklist,
	}

	var risks []string

	if !checklist.PAMRoleElevationPassed {
		risks = append(risks, "Just-In-Time PAM role elevation and auto-expiry incomplete")
	}
	if !checklist.DSARPrivacyWorkflowPassed {
		risks = append(risks, "GDPR/PIPA data subject rights export and erasure synchronization incomplete")
	}
	if !checklist.ModerationTakedownPassed {
		risks = append(risks, "Content takedown and appeal review workflow incomplete")
	}
	if !checklist.AutomatedPentestVectorsPassed {
		risks = append(risks, "Automated penetration test coverage (SSRF/SQLi/BOLA) incomplete")
	}
	if !checklist.DualCustodyEnforced {
		risks = append(risks, "Dual-custody 4-eyes approval for sensitive actions incomplete")
	}
	if !checklist.ZeroCriticalVulnerabilities {
		risks = append(risks, "Zero P0/P1 vulnerability gate requirement incomplete")
	}
	if !checklist.ThirdPartyPentestAuditSigned {
		risks = append(risks, "Independent third-party penetration test and external audit report pending")
	}
	if !checklist.LegalPrivacyComplianceSigned {
		risks = append(risks, "Legal privacy regulatory review (PIPA/GDPR) sign-off pending")
	}

	report.UnresolvedRisks = risks

	if len(risks) == 0 {
		report.Decision = GateDecisionGo
		report.GateReason = "All technical and operational security Phase 8 criteria satisfied"
	} else {
		report.Decision = GateDecisionNoGo
		report.GateReason = fmt.Sprintf("Phase 8 Gate NO-GO: %d requirement(s) pending (e.g. %s)", len(risks), risks[len(risks)-1])
	}

	return report
}
