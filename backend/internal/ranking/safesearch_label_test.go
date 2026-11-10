package ranking

import "testing"

// C-5: index-time labels must drive exactly the same decisions as the runtime
// FilterResults across all levels.
func TestIndexLabelMatchesFilterResults(t *testing.T) {
	classifier := NewSafeSearchClassifier()
	cases := []struct {
		name        string
		title       string
		description string
		url         string
		wantLabel   string
	}{
		{"clean", "OpenPortal developer documentation", "Example English document", "https://example.org/docs", SafetyClean},
		{"adult-explicit", "adult xxx explicit videos", "free porn streaming", "https://example.org/a", SafetyAdult},
		{"gambling", "online casino bonus poker", "betting odds sportsbook", "https://example.org/g", SafetyGambling},
		{"malware", "free cracked software download", "keygen trojan payload", "https://example.org/m", SafetyMalware},
	}
	for _, tc := range cases {
		got, _, _ := classifier.IndexLabel(tc.title, tc.description, tc.url)
		if got != tc.wantLabel {
			t.Fatalf("case=%s: label=%s, want %s (fixture must trigger its category)", tc.name, got, tc.wantLabel)
		}
	}
	for _, level := range []SafeSearchLevel{SafeSearchStrict, SafeSearchModerate, SafeSearchOff} {
		for _, tc := range cases {
			safety := classifier.ClassifyContent(tc.title, tc.description, tc.url)
			doc := SafeSearchDocument{DocumentID: "d", Title: tc.title, Description: tc.description, URL: tc.url, Safety: safety}
			kept := len(classifier.FilterResults([]SafeSearchDocument{doc}, level)) == 1
			category, adultScore, malwareScore := classifier.IndexLabel(tc.title, tc.description, tc.url)
			keptByLabel := safetyLabelPasses(category, adultScore, malwareScore, level)
			if kept != keptByLabel {
				t.Fatalf("level=%s case=%s: runtime kept=%v but label(%s,%.2f,%.2f) kept=%v",
					level, tc.name, kept, category, adultScore, malwareScore, keptByLabel)
			}
		}
	}
}

// safetyLabelPasses mirrors the OpenSearch must_not clauses for the level.
func safetyLabelPasses(category string, adultScore, malwareScore float64, level SafeSearchLevel) bool {
	switch level {
	case SafeSearchStrict:
		return category == SafetyClean
	case SafeSearchOff:
		return !(category == SafetyMalware && malwareScore >= 0.9)
	default:
		if category == SafetyGambling || category == SafetyMalware {
			return false
		}
		if category == SafetyAdult && adultScore >= 0.7 {
			return false
		}
		return true
	}
}
