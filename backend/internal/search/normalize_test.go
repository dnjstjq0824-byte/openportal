package search

import (
	"testing"
)

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"", "", true},
		{"   ", "", true},
		{"  hello   world  ", "hello world", false},
		{"검색엔진   프로젝트", "검색엔진 프로젝트", false},
	}

	for _, tt := range tests {
		got, err := NormalizeQuery(tt.input)
		if (err != nil) != tt.expectError {
			t.Errorf("NormalizeQuery(%q) err = %v, expectError = %v", tt.input, err, tt.expectError)
			continue
		}
		if got != tt.expected {
			t.Errorf("NormalizeQuery(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSuggestCorrection(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"rjatordpswls", "검색엔진"},
		{"dhvmsvhxjf", "오픈포털"},
		{"gksrmf", "한글"},
		{"openprotal", "OpenPortal"},
		{"검섹엔진", "검색엔진"},
		{"검색엔짇", "검색엔진"}, // Levenshtein dist 1
		{"크롤렁", "크롤러"},   // Levenshtein dist 1
		{"databse", "database"}, // Levenshtein dist 1
		{"normal text", ""},
	}

	for _, tt := range tests {
		got := SuggestCorrection(tt.input)
		if got != tt.expected {
			t.Errorf("SuggestCorrection(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDefaultVerticalTabs(t *testing.T) {
	tabs := DefaultVerticalTabs()
	if len(tabs) == 0 {
		t.Fatalf("expected non-empty vertical tabs")
	}
	hasWeb := false
	for _, tab := range tabs {
		if tab.ID == "web" {
			hasWeb = true
			if !tab.Enabled {
				t.Errorf("expected web vertical to be enabled")
			}
		} else {
			if tab.Enabled {
				t.Errorf("expected vertical %q to be disabled in Phase 1", tab.ID)
			}
		}
	}
	if !hasWeb {
		t.Errorf("web vertical tab not found")
	}
}
