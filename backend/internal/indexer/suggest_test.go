package indexer

import (
	"strings"
	"testing"
)

func TestSuggestInputs(t *testing.T) {
	t.Run("title first deduped", func(t *testing.T) {
		got := SuggestInputs("검색엔진", []string{"랭킹", "검색엔진", "  ", "크롤러"})
		want := []string{"검색엔진", "랭킹", "크롤러"}
		if len(got) != len(want) {
			t.Fatalf("got %q, want %q", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %q, want %q", got, want)
			}
		}
	})
	t.Run("empty title falls back to headings", func(t *testing.T) {
		got := SuggestInputs("  ", []string{"h1"})
		if len(got) != 1 || got[0] != "h1" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("caps count and length", func(t *testing.T) {
		headings := make([]string, 20)
		for i := range headings {
			headings[i] = "h"
		}
		headings[0] = strings.Repeat("가", MaxSuggestInputRunes+50)
		got := SuggestInputs("title", headings)
		if len(got) > MaxSuggestInputs {
			t.Fatalf("too many inputs: %d", len(got))
		}
		for _, s := range got {
			if len([]rune(s)) > MaxSuggestInputRunes {
				t.Fatalf("input exceeds cap: %q", s)
			}
		}
	})
}
