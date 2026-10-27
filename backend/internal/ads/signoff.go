package ads

import (
	"fmt"
	"time"
)

// GateDecision represents binary Go / No-Go determination for Phase 7.
type GateDecision string

const (
	GateDecisionGo   GateDecision = "GO"
	GateDecisionNoGo GateDecision = "NO-GO"
)

// AdsAuditChecklist tracks compliance across all Phase 7 criteria.
type AdsAuditChecklist struct {
	WebmasterDomainVerificationPassed bool `json:"webmaster_domain_verification_passed"`
	CampaignStateMachinePassed        bool `json:"campaign_state_machine_passed"`
	ProhibitedIndustryFilterPassed    bool `json:"prohibited_industry_filter_passed"`
	GSPAuctionSponsoredTagPassed      bool `json:"gsp_auction_sponsored_tag_passed"`
	ClickFraudAbuseDetectionPassed    bool `json:"click_fraud_abuse_detection_passed"`
	DoubleEntryLedgerBalancedPassed   bool `json:"double_entry_ledger_balanced_passed"`
	RealTransactionSandboxEnforced    bool `json:"real_transaction_sandbox_enforced"`
	ThirdPartyFinancialAuditPassed    bool `json:"third_party_financial_audit_passed"`
	DualAuthorizationApprovalSigned   bool `json:"dual_authorization_approval_signed"`
}

// Phase7GateReport details the audit evaluation for advertising and payments.
type Phase7GateReport struct {
	Phase           int               `json:"phase"`
	EvaluatedAt     time.Time         `json:"evaluated_at"`
	Decision        GateDecision      `json:"decision"`
	GateReason      string            `json:"gate_reason"`
	Checklist       AdsAuditChecklist `json:"checklist"`
	UnresolvedRisks []string          `json:"unresolved_risks"`
}

// EvaluatePhase7ReleaseGate evaluates Phase 7 readiness:
// Adhering strictly to AGENTS.md rule: "Avoid enabling payments, deploys or crawling arbitrary domains without authorized infrastructure and legal review... mark blocked work as blocked."
// Because actual real monetary payment processing requires dual executive authorization and an independent third-party SOC1/SOC2 financial audit,
// the gate evaluates strictly and marks unapproved financial flows as blocked (NO-GO).
func EvaluatePhase7ReleaseGate(checklist AdsAuditChecklist) Phase7GateReport {
	report := Phase7GateReport{
		Phase:       7,
		EvaluatedAt: time.Now(),
		Checklist:   checklist,
	}

	var risks []string

	if !checklist.WebmasterDomainVerificationPassed {
		risks = append(risks, "Webmaster domain ownership verification and sitemap ingestion incomplete")
	}
	if !checklist.CampaignStateMachinePassed {
		risks = append(risks, "Advertising campaign review state machine transition audit trail incomplete")
	}
	if !checklist.ProhibitedIndustryFilterPassed {
		risks = append(risks, "Prohibited industry and personal targeting constraint enforcement incomplete")
	}
	if !checklist.GSPAuctionSponsoredTagPassed {
		risks = append(risks, "Generalized Second Price auction mandatory Sponsored disclosure incomplete")
	}
	if !checklist.ClickFraudAbuseDetectionPassed {
		risks = append(risks, "Click fraud burst deduplication and scraper bot detection incomplete")
	}
	if !checklist.DoubleEntryLedgerBalancedPassed {
		risks = append(risks, "Double-entry accounting ledger debit/credit consistency incomplete")
	}
	if !checklist.RealTransactionSandboxEnforced {
		risks = append(risks, "Real monetary transaction gateway must remain fail-closed in sandbox mode")
	}
	if !checklist.ThirdPartyFinancialAuditPassed {
		risks = append(risks, "Independent third-party financial accounting and tax compliance audit pending")
	}
	if !checklist.DualAuthorizationApprovalSigned {
		risks = append(risks, "Dual-authorization executive approval signature for live commercial payments pending")
	}

	report.UnresolvedRisks = risks

	if len(risks) == 0 {
		report.Decision = GateDecisionGo
		report.GateReason = "All technical and financial Phase 7 advertising criteria satisfied"
	} else {
		report.Decision = GateDecisionNoGo
		report.GateReason = fmt.Sprintf("Phase 7 Gate NO-GO: %d requirement(s) pending (e.g. %s)", len(risks), risks[len(risks)-1])
	}

	return report
}
