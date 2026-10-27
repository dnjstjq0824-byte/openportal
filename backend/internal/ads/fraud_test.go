package ads

import (
	"testing"
	"time"
)

func TestClickFraudDetector(t *testing.T) {
	detector := NewClickFraudDetector(1 * time.Second)

	ip := "203.0.113.10"
	adID := "ad-100"
	realUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	botUA := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

	// 1. Legitimate click
	res1 := detector.ValidateClick(ip, realUA, adID, 350.0)
	if !res1.IsValid || res1.BillableAmount != 350.0 {
		t.Errorf("expected genuine click valid, got %+v", res1)
	}

	// 2. Immediate burst click from same IP and Ad within 1s window -> blocked
	res2 := detector.ValidateClick(ip, realUA, adID, 350.0)
	if res2.IsValid || res2.BillableAmount != 0.0 {
		t.Errorf("expected burst click to be invalidated, got %+v", res2)
	}
	if res2.InvalidReason != "rapid_click_burst_deduplicated" {
		t.Errorf("unexpected invalid reason: %s", res2.InvalidReason)
	}

	// 3. Bot user-agent click -> blocked
	resBot := detector.ValidateClick("203.0.113.50", botUA, "ad-200", 200.0)
	if resBot.IsValid || resBot.BillableAmount != 0.0 {
		t.Errorf("expected bot click to be invalidated")
	}
	if resBot.InvalidReason != "automated_bot_user_agent" {
		t.Errorf("unexpected invalid reason: %s", resBot.InvalidReason)
	}
}

