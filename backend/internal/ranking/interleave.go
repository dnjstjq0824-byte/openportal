// Team-draft interleaving for online ranking comparison (EVAL-02, §6.3).
//
// Two rankers (control=A, treatment=B) contribute documents turn by turn;
// clicks are credited to the contributing team and the per-query winner is
// the team with more clicks (ties stay ties). Assignment is deterministic
// (SHA-256 over experiment id + salt + identifier), mirroring
// ExperimentPlatform.AssignDeterministic. State stays in-memory: online use
// requires the 049 persistence follow-up (G020).
package ranking

import (
	"crypto/sha256"
	"encoding/binary"
)

// InterleavedItem is one exposed document with its contributing team and its
// 1-based ranks in the source lists (0 when the document is absent there).
type InterleavedItem struct {
	URL   string  `json:"url"`
	Team  Variant `json:"team"`
	RankA int     `json:"rank_a"`
	RankB int     `json:"rank_b"`
}

// Click is one counted click, matched to the interleaved list by URL.
type Click struct {
	URL string `json:"url"`
}

// Preference is the per-query team outcome.
type Preference string

const (
	PrefControl   Preference = "control"
	PrefTreatment Preference = "treatment"
	PrefTie       Preference = "tie"
)

// FirstTeam deterministically picks which team drafts first for an identifier
// (request/experiment id). Split is ~50/50 over the identifier space.
func FirstTeam(expID, salt, identifier string) Variant {
	sum := sha256.Sum256([]byte(expID + ":" + salt + ":" + identifier))
	if binary.BigEndian.Uint32(sum[:4])%2 == 0 {
		return VariantControl
	}
	return VariantTreatment
}

// TeamDraft merges two ranked URL lists via team draft. Teams alternate picks
// starting with first; each pick takes the highest-ranked remaining document
// from its own list. Already-taken documents are skipped. Output stops at
// maxLen (<=0 means no cap) or when both lists are exhausted.
func TeamDraft(a, b []string, first Variant, maxLen int) []InterleavedItem {
	rankA := make(map[string]int, len(a))
	for i, url := range a {
		if _, seen := rankA[url]; !seen {
			rankA[url] = i + 1
		}
	}
	rankB := make(map[string]int, len(b))
	for i, url := range b {
		if _, seen := rankB[url]; !seen {
			rankB[url] = i + 1
		}
	}
	taken := make(map[string]bool, len(a)+len(b))
	var out []InterleavedItem
	turn := first
	if turn != VariantControl && turn != VariantTreatment {
		turn = VariantControl
	}
	for {
		if maxLen > 0 && len(out) >= maxLen {
			break
		}
		var pick string
		var team Variant
		if turn == VariantControl {
			pick = nextAvailable(a, taken)
			team = VariantControl
		} else {
			pick = nextAvailable(b, taken)
			team = VariantTreatment
		}
		if pick == "" {
			// Current team exhausted: offer the turn to the other team once.
			if turn == VariantControl {
				pick = nextAvailable(b, taken)
				team = VariantTreatment
			} else {
				pick = nextAvailable(a, taken)
				team = VariantControl
			}
			if pick == "" {
				break
			}
		}
		taken[pick] = true
		out = append(out, InterleavedItem{URL: pick, Team: team, RankA: rankA[pick], RankB: rankB[pick]})
		if turn == VariantControl {
			turn = VariantTreatment
		} else {
			turn = VariantControl
		}
	}
	return out
}

func nextAvailable(list []string, taken map[string]bool) string {
	for _, url := range list {
		if !taken[url] {
			return url
		}
	}
	return ""
}

// InferPreference credits each click to its team (matched by URL) and returns
// the winning team. Clicks on unexposed URLs are ignored (tie-neutral).
func InferPreference(items []InterleavedItem, clicks []Click) Preference {
	teamOf := make(map[string]Variant, len(items))
	for _, item := range items {
		teamOf[item.URL] = item.Team
	}
	var control, treatment int
	for _, click := range clicks {
		switch teamOf[click.URL] {
		case VariantControl:
			control++
		case VariantTreatment:
			treatment++
		}
	}
	switch {
	case treatment > control:
		return PrefTreatment
	case control > treatment:
		return PrefControl
	default:
		return PrefTie
	}
}

// TallyPreferences aggregates per-query outcomes into win counts.
func TallyPreferences(prefs []Preference) (controlWins, treatmentWins, ties int) {
	for _, pref := range prefs {
		switch pref {
		case PrefTreatment:
			treatmentWins++
		case PrefControl:
			controlWins++
		default:
			ties++
		}
	}
	return controlWins, treatmentWins, ties
}
