package ranking

import (
	"fmt"
	"testing"
)

func TestTeamDraftHandComputed(t *testing.T) {
	a := []string{"a1", "a2", "a3"}
	b := []string{"b1", "a1", "b2"}
	got := TeamDraft(a, b, VariantControl, 0)
	want := []InterleavedItem{
		{URL: "a1", Team: VariantControl, RankA: 1, RankB: 2},
		{URL: "b1", Team: VariantTreatment, RankA: 0, RankB: 1},
		{URL: "a2", Team: VariantControl, RankA: 2, RankB: 0},
		{URL: "b2", Team: VariantTreatment, RankA: 0, RankB: 3},
		{URL: "a3", Team: VariantControl, RankA: 3, RankB: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("draft length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pick %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTeamDraftTreatmentFirstAndCap(t *testing.T) {
	got := TeamDraft([]string{"a1", "a2"}, []string{"b1", "b2"}, VariantTreatment, 3)
	if len(got) != 3 {
		t.Fatalf("capped length = %d", len(got))
	}
	if got[0].Team != VariantTreatment || got[0].URL != "b1" {
		t.Fatalf("first pick = %+v", got[0])
	}
	if got[1].URL != "a1" || got[2].URL != "b2" {
		t.Fatalf("order = %v", got)
	}
}

func TestTeamDraftEmptyAndDuplicates(t *testing.T) {
	if got := TeamDraft(nil, nil, VariantControl, 0); len(got) != 0 {
		t.Fatalf("empty draft = %v", got)
	}
	// Duplicates inside one list collapse to a single exposure.
	got := TeamDraft([]string{"x", "x", "y"}, nil, VariantControl, 0)
	if len(got) != 2 || got[0].URL != "x" || got[1].URL != "y" {
		t.Fatalf("dedup draft = %v", got)
	}
	// One-sided exhaustion keeps serving the other team.
	got = TeamDraft([]string{"a1", "a2", "a3"}, []string{"b1"}, VariantTreatment, 0)
	if len(got) != 4 {
		t.Fatalf("exhaustion draft = %v", got)
	}
}

func TestFirstTeamDeterministicAndBalanced(t *testing.T) {
	if FirstTeam("exp", "salt", "user-1") != FirstTeam("exp", "salt", "user-1") {
		t.Fatal("first team is not deterministic")
	}
	var treatment int
	const n = 2000
	for i := 0; i < n; i++ {
		if FirstTeam("exp", "salt", fmt.Sprintf("id-%d", i)) == VariantTreatment {
			treatment++
		}
	}
	ratio := float64(treatment) / n
	if ratio < 0.4 || ratio > 0.6 {
		t.Fatalf("treatment share = %.3f, want within 0.4..0.6", ratio)
	}
}

func TestInferPreference(t *testing.T) {
	items := TeamDraft([]string{"a1", "a2"}, []string{"b1", "b2"}, VariantControl, 0)
	if got := InferPreference(items, []Click{{URL: "a1"}, {URL: "a2"}}); got != PrefControl {
		t.Fatalf("control clicks = %q", got)
	}
	if got := InferPreference(items, []Click{{URL: "b1"}}); got != PrefTreatment {
		t.Fatalf("treatment click = %q", got)
	}
	if got := InferPreference(items, []Click{{URL: "a1"}, {URL: "b1"}}); got != PrefTie {
		t.Fatalf("split clicks = %q", got)
	}
	if got := InferPreference(items, nil); got != PrefTie {
		t.Fatalf("no clicks = %q", got)
	}
	if got := InferPreference(items, []Click{{URL: "https://unknown.example/"}}); got != PrefTie {
		t.Fatalf("unknown click = %q", got)
	}
}

func TestTallyPreferences(t *testing.T) {
	c, tr, ties := TallyPreferences([]Preference{PrefControl, PrefTreatment, PrefTreatment, PrefTie, "bogus"})
	if c != 1 || tr != 2 || ties != 2 {
		t.Fatalf("tally = %d/%d/%d", c, tr, ties)
	}
}
