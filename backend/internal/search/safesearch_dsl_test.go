package search

import (
	"encoding/json"
	"strings"
	"testing"
)

// C-5: each SafeSearch level renders the documented must_not clauses.
func TestSafetyFilterClauses(t *testing.T) {
	strict := safetyFilterClauses("strict")
	raw, _ := json.Marshal(strict)
	if !strings.Contains(string(raw), `"safety":["adult","gambling","malware"]`) {
		t.Fatalf("strict: %s", raw)
	}
	moderate := safetyFilterClauses("moderate")
	raw, _ = json.Marshal(moderate)
	if !strings.Contains(string(raw), `"safety":["gambling","malware"]`) || !strings.Contains(string(raw), `"gte":0.7`) {
		t.Fatalf("moderate: %s", raw)
	}
	def := safetyFilterClauses("")
	rawDef, _ := json.Marshal(def)
	rawMod, _ := json.Marshal(moderate)
	if string(rawDef) != string(rawMod) {
		t.Fatal("empty level must equal moderate")
	}
	off := safetyFilterClauses("off")
	raw, _ = json.Marshal(off)
	if !strings.Contains(string(raw), `"safety":"malware"`) || !strings.Contains(string(raw), `"gte":0.9`) {
		t.Fatalf("off: %s", raw)
	}
	if strings.Contains(string(raw), `"safety":["gambling","malware"]`) {
		t.Fatalf("off must not block gambling: %s", raw)
	}
	// Unknown levels fail closed to moderate.
	unknown := safetyFilterClauses("bogus")
	rawUnk, _ := json.Marshal(unknown)
	if string(rawUnk) != string(rawMod) {
		t.Fatal("unknown level must fail closed to moderate")
	}
}

func TestBuildQueryIncludesSafetyFilter(t *testing.T) {
	parsed := ParseQuery("test")
	parsed.SafeSearch = "strict"
	raw, _ := json.Marshal(parsed.BuildOpenSearchQuery("ko"))
	if !strings.Contains(string(raw), `"safety":["adult","gambling","malware"]`) {
		t.Fatalf("DSL missing strict filter: %s", raw)
	}
}
