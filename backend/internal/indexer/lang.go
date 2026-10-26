package indexer

import (
	"strings"
	"unicode"
)

// DetectLanguage inspects Unicode blocks and character frequencies to determine
// whether text is primarily Korean ('ko'), English ('en'), or other ('unknown').
func DetectLanguage(text string) string {
	clean := strings.TrimSpace(text)
	if len(clean) == 0 {
		return "en" // default fallback
	}

	var totalAlpha int
	var hangulCount int
	var latinCount int
	var cjkCount int

	for _, r := range clean {
		if unicode.IsLetter(r) {
			totalAlpha++
			switch {
			case isHangul(r):
				hangulCount++
			case isLatin(r):
				latinCount++
			case isCJK(r):
				cjkCount++
			}
		}
	}

	if totalAlpha == 0 {
		return "en"
	}

	hangulRatio := float64(hangulCount) / float64(totalAlpha)
	latinRatio := float64(latinCount) / float64(totalAlpha)
	cjkRatio := float64(cjkCount) / float64(totalAlpha)

	// If at least 15% of characters are Hangul, consider it Korean
	if hangulRatio >= 0.15 || (hangulCount > 5 && hangulRatio > 0.08) {
		return "ko"
	}

	if latinRatio > 0.5 {
		return "en"
	}

	if cjkRatio > 0.4 {
		return "zh"
	}

	return "en"
}

func isHangul(r rune) bool {
	// Hangul Syllables (AC00-D7AF), Hangul Jamo (1100-11FF), Hangul Compatibility Jamo (3130-318F)
	return (r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0x1100 && r <= 0x11FF) ||
		(r >= 0x3130 && r <= 0x318F)
}

func isLatin(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= 0x00C0 && r <= 0x024F) // Latin-1 Supplement & Extended
}

func isCJK(r rune) bool {
	// CJK Unified Ideographs
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF)
}

