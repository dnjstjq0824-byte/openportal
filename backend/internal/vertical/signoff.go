package vertical

import (
	"fmt"
	"time"
)

// GateDecision represents the binary Go / No-Go determination for Phase 5 release.
type GateDecision string

const (
	GateDecisionGo   GateDecision = "GO"
	GateDecisionNoGo GateDecision = "NO-GO"
)

// VerticalAuditChecklist captures the state of each vertical verification criteria.
type VerticalAuditChecklist struct {
	ImageLicensingAndSafetyPassed bool `json:"image_licensing_and_safety_passed"`
	VideoEmbedWhitelistPassed     bool `json:"video_embed_whitelist_passed"`
	NewsPublisherAndRetractPassed bool `json:"news_publisher_and_retract_passed"`
	MapsODbLAttributionPassed     bool `json:"maps_odbl_attribution_passed"`
	ShoppingSponsoredTagPassed    bool `json:"shopping_sponsored_tag_passed"`
	CommercialContractsExecuted   bool `json:"commercial_contracts_executed"`
	LegalSignoffRecorded          bool `json:"legal_signoff_recorded"`
}

// Phase5GateEvaluation contains the comprehensive release gate report for Phase 5.
type Phase5GateReport struct {
	Phase           int                    `json:"phase"`
	EvaluatedAt     time.Time              `json:"evaluated_at"`
	Decision        GateDecision           `json:"decision"`
	GateReason      string                 `json:"gate_reason"`
	Checklist       VerticalAuditChecklist `json:"checklist"`
	UnresolvedRisks []string               `json:"unresolved_risks"`
}

// EvaluatePhase5ReleaseGate evaluates Phase 5 readiness according to AGENTS.md and CODEX rules:
// Even though all code, parsers, filters, and safety classifiers pass local tests,
// Phase 5 requires executed commercial publisher agreements and third-party commercial syndication contracts.
// Therefore, the gate evaluates strictly and marks unapproved commercial contracts as blocked.
func EvaluatePhase5ReleaseGate(checklist VerticalAuditChecklist) Phase5GateReport {
	report := Phase5GateReport{
		Phase:       5,
		EvaluatedAt: time.Now(),
		Checklist:   checklist,
	}

	var risks []string

	if !checklist.ImageLicensingAndSafetyPassed {
		risks = append(risks, "Image licensing or content safety classification incomplete")
	}
	if !checklist.VideoEmbedWhitelistPassed {
		risks = append(risks, "Video embed domain whitelist or security guardrails incomplete")
	}
	if !checklist.NewsPublisherAndRetractPassed {
		risks = append(risks, "News publisher licensing verification or retraction workflow incomplete")
	}
	if !checklist.MapsODbLAttributionPassed {
		risks = append(risks, "Maps OpenStreetMap ODbL attribution or POI correction logging incomplete")
	}
	if !checklist.ShoppingSponsoredTagPassed {
		risks = append(risks, "Shopping FTC sponsored disclosure or price freshness guard incomplete")
	}
	if !checklist.CommercialContractsExecuted {
		risks = append(risks, "Commercial news syndication contracts and shopping affiliate agreements not legally executed")
	}
	if !checklist.LegalSignoffRecorded {
		risks = append(risks, "Independent legal review and copyright compliance sign-off pending")
	}

	report.UnresolvedRisks = risks

	if len(risks) == 0 {
		report.Decision = GateDecisionGo
		report.GateReason = "All technical and legal Phase 5 vertical criteria satisfied"
	} else {
		report.Decision = GateDecisionNoGo
		report.GateReason = fmt.Sprintf("Phase 5 Gate NO-GO: %d requirement(s) pending (e.g. %s)", len(risks), risks[len(risks)-1])
	}

	return report
}
