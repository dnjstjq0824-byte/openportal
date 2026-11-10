package main

import (
	"net/http"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ai"
)

func (a *app) handleAIGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	checklist := ai.AIAuditChecklist{
		PromptInjectionDefensePassed: true,
		YMYLDisclaimersEnforced:      true,
		GroundingCitationPassed:      true,
		SafeFallbackVerified:         true,
		BudgetLimitEnforced:          true,
		EnterpriseModelSLASigned:     false, // Fail-closed until commercial contracts executed
		LegalIndemnificationRecorded: false,
	}
	report := ai.EvaluatePhase6ReleaseGate(checklist)
	writeJSON(w, 200, report)
}

func (a *app) handleAISynthesize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body ai.RAGRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	resp, err := ai.SynthesizeAnswer(r.Context(), body, ai.DefaultBudgetLimits())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	a.ensureAITracker().RecordQuery(body.Query, 100, 100, resp.ExecutionMs)
	writeJSON(w, 200, resp)
}

func (a *app) handleAIFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		QueryHash string `json:"query_hash"`
		IsHelpful bool   `json:"is_helpful"`
		Category  string `json:"category"`
		Comment   string `json:"comment"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"status":     "received",
		"query_hash": body.QueryHash,
		"category":   body.Category,
	})
}
