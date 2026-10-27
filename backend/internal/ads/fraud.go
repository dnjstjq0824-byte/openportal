package ads

import (
	"strings"
	"sync"
	"time"
)

// ClickValidationResult represents the fraud check outcome for an ad click event.
type ClickValidationResult struct {
	IsValid        bool   `json:"is_valid"`
	InvalidReason  string `json:"invalid_reason,omitempty"`
	BillableAmount float64 `json:"billable_amount"`
}

// ClickFraudDetector tracks click rates per client IP and detects bot abuse.
type ClickFraudDetector struct {
	mu           sync.Mutex
	recentClicks map[string]time.Time // key: IP + ":" + AdID
	dedupWindow  time.Duration
}

// NewClickFraudDetector creates a new fraud detector instance.
func NewClickFraudDetector(dedupWindow time.Duration) *ClickFraudDetector {
	if dedupWindow <= 0 {
		dedupWindow = 5 * time.Second
	}
	return &ClickFraudDetector{
		recentClicks: make(map[string]time.Time),
		dedupWindow:  dedupWindow,
	}
}

var botUserAgents = []string{
	"bot", "spider", "crawler", "headless", "phantomjs", "selenium", "puppeteer", "curl", "wget", "python-requests",
}

// ValidateClick checks if an ad click is genuine or abusive/bot-generated (Task 077).
func (d *ClickFraudDetector) ValidateClick(clientIP, userAgent, adID string, clearingCPC float64) ClickValidationResult {
	lowUA := strings.ToLower(userAgent)

	// 1. Known bot and scraper user-agent detection
	for _, bot := range botUserAgents {
		if strings.Contains(lowUA, bot) {
			return ClickValidationResult{
				IsValid:        false,
				InvalidReason:  "automated_bot_user_agent",
				BillableAmount: 0.0,
			}
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	key := clientIP + ":" + adID
	lastClick, exists := d.recentClicks[key]
	now := time.Now()

	// 2. Rapid duplicate burst click check within dedupWindow (e.g. 5 seconds)
	if exists && now.Sub(lastClick) < d.dedupWindow {
		return ClickValidationResult{
			IsValid:        false,
			InvalidReason:  "rapid_click_burst_deduplicated",
			BillableAmount: 0.0,
		}
	}

	d.recentClicks[key] = now
	return ClickValidationResult{
		IsValid:        true,
		BillableAmount: clearingCPC,
	}
}
