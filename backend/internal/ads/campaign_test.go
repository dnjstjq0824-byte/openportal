package ads

import (
	"testing"
)

func TestValidateCampaignStateTransition(t *testing.T) {
	// Valid transitions
	if err := ValidateCampaignStateTransition(StatusDraft, StatusPendingReview); err != nil {
		t.Errorf("expected draft -> pending_review to be valid, got %v", err)
	}
	if err := ValidateCampaignStateTransition(StatusPendingReview, StatusApproved); err != nil {
		t.Errorf("expected pending_review -> approved to be valid, got %v", err)
	}
	if err := ValidateCampaignStateTransition(StatusApproved, StatusActive); err != nil {
		t.Errorf("expected approved -> active to be valid, got %v", err)
	}

	// Invalid transitions
	if err := ValidateCampaignStateTransition(StatusDraft, StatusActive); err == nil {
		t.Errorf("expected draft -> active to fail review check, got nil")
	}
	if err := ValidateCampaignStateTransition(StatusArchived, StatusActive); err == nil {
		t.Errorf("expected transition from archived terminal state to fail, got nil")
	}
}

func TestReviewCreative(t *testing.T) {
	// 1. Valid creative
	validCreative := AdCreative{
		Headline:       "클라우드 서버 할인 프로모션",
		Description:    "안정적인 인프라를 합리적인 가격에 시작하세요.",
		DestinationURL: "https://cloud.openportal.org/promo",
	}
	approved, reason := ReviewCreative(validCreative, []string{"region_seoul", "dev_tech_interest"})
	if !approved {
		t.Errorf("expected valid creative to be approved, got reason: %s", reason)
	}

	// 2. Prohibited industry (illegal gambling)
	gamblingCreative := AdCreative{
		Headline:       "최고 배당률 사설토토",
		Description:    "지금 가입하면 첫충전 보너스",
		DestinationURL: "https://gamble.test",
	}
	approved, _ = ReviewCreative(gamblingCreative, nil)
	if approved {
		t.Errorf("expected gambling creative to be rejected")
	}

	// 3. Prohibited micro-targeting trait (religious belief)
	approved, _ = ReviewCreative(validCreative, []string{"religious_belief"})
	if approved {
		t.Errorf("expected sensitive trait micro-targeting to be rejected")
	}

	// 4. Insecure HTTP destination
	insecureCreative := validCreative
	insecureCreative.DestinationURL = "http://insecure.test"
	approved, _ = ReviewCreative(insecureCreative, nil)
	if approved {
		t.Errorf("expected insecure HTTP destination to be rejected")
	}
}

func TestApplyTransitionRoleEnforcement(t *testing.T) {
	c := &AdCreative{
		Status: StatusPendingReview,
	}

	// Unauthorized role attempting approval
	if err := ApplyTransition(c, StatusApproved, "advertiser"); err == nil {
		t.Errorf("expected advertiser unable to self-approve ad creative")
	}

	// Authorized reviewer role
	if err := ApplyTransition(c, StatusApproved, "ad_reviewer"); err != nil {
		t.Errorf("expected ad_reviewer to approve ad creative: %v", err)
	}
	if c.Status != StatusApproved {
		t.Errorf("expected status approved, got %s", c.Status)
	}
}

