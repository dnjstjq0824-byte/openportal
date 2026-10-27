package ads

import (
	"errors"
	"fmt"
	"strings"
)

// CampaignStatus defines the lifecycle status of an advertising campaign.
type CampaignStatus string

const (
	StatusDraft         CampaignStatus = "draft"
	StatusPendingReview CampaignStatus = "pending_review"
	StatusApproved      CampaignStatus = "approved"
	StatusRejected      CampaignStatus = "rejected"
	StatusActive        CampaignStatus = "active"
	StatusPaused        CampaignStatus = "paused"
	StatusArchived      CampaignStatus = "archived"
)

// AdCreative represents an ad copy and destination URL.
type AdCreative struct {
	ID             string         `json:"id"`
	CampaignID     string         `json:"campaign_id"`
	Headline       string         `json:"headline"`
	Description    string         `json:"description"`
	DestinationURL string         `json:"destination_url"`
	Status         CampaignStatus `json:"status"`
	RejectionNote  string         `json:"rejection_note,omitempty"`
}

// Prohibited industry keywords (Task 074)
var prohibitedIndustries = []string{
	"불법도박", "사설토토", "고리대금", "무담보 즉시대출", "위조품", "짝퉁", "불법 마약",
	"illegal gambling", "predatory loan", "counterfeit replica", "unauthorized pharmacy",
}

// Prohibited micro-targeting traits
var prohibitedTargetingTraits = []string{
	"racial_origin", "religious_belief", "political_affiliation", "health_condition", "sexual_orientation",
}

// ValidateCampaignStateTransition validates allowed state transitions in the state machine (Task 073).
func ValidateCampaignStateTransition(from, to CampaignStatus) error {
	allowed := map[CampaignStatus][]CampaignStatus{
		StatusDraft:         {StatusPendingReview, StatusArchived},
		StatusPendingReview: {StatusApproved, StatusRejected, StatusDraft},
		StatusApproved:      {StatusActive, StatusPaused, StatusArchived},
		StatusRejected:      {StatusDraft, StatusArchived},
		StatusActive:        {StatusPaused, StatusArchived},
		StatusPaused:        {StatusActive, StatusArchived},
		StatusArchived:      {}, // Terminal state
	}

	for _, target := range allowed[from] {
		if target == to {
			return nil
		}
	}
	return fmt.Errorf("invalid campaign state transition from %s to %s", from, to)
}

// ReviewCreative checks ad creative copy and destination against industry policies and targeting restrictions (Task 074).
func ReviewCreative(c AdCreative, targetingTraits []string) (bool, string) {
	combinedText := strings.ToLower(c.Headline + " " + c.Description + " " + c.DestinationURL)

	// 1. Prohibited industry check
	for _, kw := range prohibitedIndustries {
		if strings.Contains(combinedText, kw) {
			return false, fmt.Sprintf("creative contains prohibited category content: %s", kw)
		}
	}

	// 2. Sensitive personal data micro-targeting restrictions
	for _, trait := range targetingTraits {
		lowTrait := strings.ToLower(trait)
		for _, prohibited := range prohibitedTargetingTraits {
			if strings.EqualFold(lowTrait, prohibited) {
				return false, fmt.Sprintf("targeting by sensitive category %s is strictly prohibited by privacy policy", trait)
			}
		}
	}

	// 3. Destination URL validation
	if !strings.HasPrefix(strings.ToLower(c.DestinationURL), "https://") {
		return false, "destination URL must use secure HTTPS protocol"
	}

	return true, "approved"
}

// ApplyTransition updates the creative status after checking transition validity and policy rules.
func ApplyTransition(c *AdCreative, target CampaignStatus, actorRole string) error {
	if err := ValidateCampaignStateTransition(c.Status, target); err != nil {
		return err
	}

	// Only authorized reviewers can approve or reject
	if target == StatusApproved || target == StatusRejected {
		if actorRole != "ad_reviewer" && actorRole != "security_auditor" {
			return errors.New("unauthorized: approval and rejection require ad_reviewer role")
		}
	}

	c.Status = target
	return nil
}

