package main

import (
	"net/http"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
)

func (a *app) handleModerationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		DocumentID string `json:"document_id"`
		Reporter   string `json:"reporter"`
		Category   string `json:"category"`
		Details    string `json:"details"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if body.DocumentID == "" || body.Category == "" {
		writeJSON(w, 400, map[string]string{"error": "document_id and category are required"})
		return
	}
	reporter := body.Reporter
	if reporter == "" {
		reporter = resolveClientIP(r)
	}
	caseRec, err := a.ensureModeration().SubmitReport(body.DocumentID, reporter, ranking.ReportCategory(body.Category), body.Details)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, caseRec)
}

func (a *app) handleModerationAppeal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		CaseID    string `json:"case_id"`
		Appellant string `json:"appellant"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	appeal, err := a.ensureModeration().SubmitAppeal(body.CaseID, body.Appellant, body.Reason)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, appeal)
}

func (a *app) handleModerationReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		CaseID  string `json:"case_id"`
		Action  string `json:"action"`
		Dismiss bool   `json:"dismiss"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	c, err := a.ensureModeration().ReviewCase(body.CaseID, body.Action, body.Dismiss)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, c)
}
// requireAuth wraps a handler with session authentication. Unauthenticated
// requests receive 401. The authenticated account is stored in the request
// context under the actorAccountIDKey for downstream BOLA checks.
