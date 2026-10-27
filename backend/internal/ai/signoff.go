package ai

import (
	"fmt"
	"time"
)

// GateDecision represents binary Go / No-Go determination for Phase 6.
type GateDecision string

const (
	GateDecisionGo   GateDecision = "GO"
	GateDecisionNoGo GateDecision = "NO-GO"
)

// AIAuditChecklist tracks compliance across all Phase 6 criteria.
type AIAuditChecklist struct {
	PromptInjectionDefensePassed bool `json:"prompt_injection_defense_passed"`
	YMYLDisclaimersEnforced      bool `json:"ymyl_disclaimers_enforced"`
	GroundingCitationPassed      bool `json:"grounding_citation_passed"`
	SafeFallbackVerified         bool `json:"safe_fallback_verified"`
	BudgetLimitEnforced          bool `json:"budget_limit_enforced"`
	EnterpriseModelSLASigned     bool `json:"enterprise_model_sla_signed"`
	LegalIndemnificationRecorded bool `json:"legal_indemnification_recorded"`
}

// Phase6GateReport details the audit evaluation for AI search release.
type Phase6GateReport struct {
	Phase           int              `json:"phase"`
	EvaluatedAt     time.Time        `json:"evaluated_at"`
	Decision        GateDecision     `json:"decision"`
	GateReason      string           `json:"gate_reason"`
	Checklist       AIAuditChecklist `json:"checklist"`
	UnresolvedRisks []string         `json:"unresolved_risks"`
}

// EvaluatePhase6ReleaseGate evaluates Phase 6 readiness:
// Adhering to AGENTS.md rule: "Phase gate needs runtime evidence; mark blocked work as blocked."
// Because enterprise commercial LLM deployment requires an executed Model SLA and legal copyright indemnification contract,
// the gate evaluates strictly and marks unapproved commercial contracts as blocked (NO-GO).
func EvaluatePhase6ReleaseGate(checklist AIAuditChecklist) Phase6GateReport {
	report := Phase6GateReport{
		Phase:       6,
		EvaluatedAt: time.Now(),
		Checklist:   checklist,
	}

	var risks []string

	if !checklist.PromptInjectionDefensePassed {
		risks = append(risks, "Prompt injection and jailbreak defense evaluation incomplete")
	}
	if !checklist.YMYLDisclaimersEnforced {
		risks = append(risks, "YMYL medical, legal, and financial disclaimer enforcement incomplete")
	}
	if !checklist.GroundingCitationPassed {
		risks = append(risks, "Synthesis grounding verification threshold (<80%) or citation linking incomplete")
	}
	if !checklist.SafeFallbackVerified {
		risks = append(risks, "Safe fallback to organic search upon model failure incomplete")
	}
	if !checklist.BudgetLimitEnforced {
		risks = append(risks, "Per-query token and daily spend cap controls incomplete")
	}
	if !checklist.EnterpriseModelSLASigned {
		risks = append(risks, "Enterprise LLM Model provider SLA and uptime guarantee agreement not legally executed")
	}
	if !checklist.LegalIndemnificationRecorded {
		risks = append(risks, "Third-party AI IP/copyright infringement legal indemnification agreement pending")
	}

	report.UnresolvedRisks = risks

	if len(risks) == 0 {
		report.Decision = GateDecisionGo
		report.GateReason = "All technical and legal Phase 6 AI search criteria satisfied"
	} else {
		report.Decision = GateDecisionNoGo
		report.GateReason = fmt.Sprintf("Phase 6 Gate NO-GO: %d requirement(s) pending (e.g. %s)", len(risks), risks[len(risks)-1])
	}

	return report
}
