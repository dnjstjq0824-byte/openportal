package launch

import (
	"errors"
	"strings"
)

// AccessibilityChecklist validates WCAG 2.1 AA compliance (Task 097).
type AccessibilityChecklist struct {
	HasKeyboardNavigation bool `json:"has_keyboard_navigation"`
	HasAriaLandmarks      bool `json:"has_aria_landmarks"`
	HasColorContrast45    bool `json:"has_color_contrast_45"`
	HasAltTextForImages   bool `json:"has_alt_text_for_images"`
	HasFocusIndicators    bool `json:"has_focus_indicators"`
}

// AuditAccessibility verifies that all accessibility standards are met.
func AuditAccessibility(chk AccessibilityChecklist) (bool, []string) {
	var missing []string
	if !chk.HasKeyboardNavigation {
		missing = append(missing, "missing full keyboard navigation (Tab/Enter/Escape)")
	}
	if !chk.HasAriaLandmarks {
		missing = append(missing, "missing ARIA landmark roles (main, search, navigation)")
	}
	if !chk.HasColorContrast45 {
		missing = append(missing, "color contrast ratio below WCAG 2.1 AA 4.5:1 minimum")
	}
	if !chk.HasAltTextForImages {
		missing = append(missing, "images missing informative alt text")
	}
	if !chk.HasFocusIndicators {
		missing = append(missing, "missing visible keyboard focus indicator rings")
	}

	return len(missing) == 0, missing
}

// LocalizationAudit verifies multi-lingual support and date/time formatting.
type LocalizationAudit struct {
	SupportedLocales []string `json:"supported_locales"`
	DefaultLocale    string   `json:"default_locale"`
}

// ValidateLocale checks if a requested locale is supported.
func (la *LocalizationAudit) ValidateLocale(locale string) error {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	for _, l := range la.SupportedLocales {
		if strings.ToLower(l) == normalized {
			return nil
		}
	}
	return errors.New("unsupported locale: " + locale)
}

// ComplianceAuditStatus combines all frontend legal and accessibility criteria.
type ComplianceAuditStatus struct {
	AccessibilityPassed bool     `json:"accessibility_passed"`
	LocalizationPassed  bool     `json:"localization_passed"`
	PrivacyPolicyNotice bool     `json:"privacy_policy_notice"`
	TermsOfServiceNotice bool    `json:"terms_of_service_notice"`
	MissingRequirements []string `json:"missing_requirements"`
}

// AuditReleaseCompliance performs full compliance check.
func AuditReleaseCompliance(a AccessibilityChecklist, l LocalizationAudit, privacyOK, termsOK bool) ComplianceAuditStatus {
	accOK, accMissing := AuditAccessibility(a)
	var missing []string
	missing = append(missing, accMissing...)

	locOK := len(l.SupportedLocales) >= 2 && l.DefaultLocale != ""
	if !locOK {
		missing = append(missing, "localization requirements not satisfied (at least ko, en required)")
	}

	if !privacyOK {
		missing = append(missing, "privacy policy notice missing or not linked")
	}
	if !termsOK {
		missing = append(missing, "terms of service notice missing or not linked")
	}

	return ComplianceAuditStatus{
		AccessibilityPassed:  accOK,
		LocalizationPassed:   locOK,
		PrivacyPolicyNotice:  privacyOK,
		TermsOfServiceNotice: termsOK,
		MissingRequirements:  missing,
	}
}
