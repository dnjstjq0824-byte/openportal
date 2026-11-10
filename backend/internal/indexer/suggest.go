package indexer

import (
	"strings"
	"unicode/utf8"
)

// MaxSuggestInputs caps completion inputs per document; MaxSuggestInputRunes
// matches the mapping's max_input_length so payloads never exceed it.
const (
	MaxSuggestInputs     = 10
	MaxSuggestInputRunes = 100
)

// SuggestInputs builds completion-suggester inputs (C-4) from the extracted
// title and headings. Empty strings are dropped, over-long inputs truncated,
// duplicates removed, order preserved (title first).
func SuggestInputs(title string, headings []string) []string {
	inputs := make([]string, 0, len(headings)+1)
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		if utf8.RuneCountInString(s) > MaxSuggestInputRunes {
			s = string([]rune(s)[:MaxSuggestInputRunes])
			s = strings.TrimSpace(s)
			if s == "" || seen[s] {
				return
			}
		}
		seen[s] = true
		inputs = append(inputs, s)
	}
	add(title)
	for _, h := range headings {
		if len(inputs) >= MaxSuggestInputs {
			break
		}
		add(h)
	}
	return inputs
}
