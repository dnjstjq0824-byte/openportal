package main

import (
	"errors"
	"net/http"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
)

func (a *app) handleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if !a.ensureStore().VerifierConfigured() {
		writeJSON(w, 503, map[string]string{"error": "authentication provider not configured", "code": "AUTH_NOT_CONFIGURED"})
		return
	}
	if report := a.ensureAbuseDetector().InspectQuery(clientIP(r), "auth:challenge"); report.IsAbusive {
		writeJSON(w, 429, map[string]string{"error": "too many authentication attempts", "code": "RATE_LIMITED"})
		return
	}
	var body struct {
		ChallengeType string `json:"challenge_type"`
		Email         string `json:"email"`
		RPID          string `json:"rp_id"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	ch, err := a.ensureStore().CreateChallenge(body.ChallengeType, body.Email, body.RPID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, ch)
}

func (a *app) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if !a.ensureStore().VerifierConfigured() {
		writeJSON(w, 503, map[string]string{"error": "authentication provider not configured", "code": "AUTH_NOT_CONFIGURED"})
		return
	}
	if report := a.ensureAbuseDetector().InspectQuery(clientIP(r), "auth:verify"); report.IsAbusive {
		writeJSON(w, 429, map[string]string{"error": "too many authentication attempts", "code": "RATE_LIMITED"})
		return
	}
	var body struct {
		ChallengeID       string `json:"challenge_id"`
		ChallengeResponse string `json:"challenge_response"`
		DisplayName       string `json:"display_name"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	token, sess, acc, codes, err := a.ensureStore().CompleteChallengeContext(r.Context(), body.ChallengeID, body.ChallengeResponse, body.DisplayName)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "op_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   12 * 3600,
	})
	out := map[string]interface{}{
		"session_token": token,
		"session":       sess,
		"account":       acc,
	}
	if len(codes) > 0 {
		out["recovery_codes"] = codes
	}
	writeJSON(w, 200, out)
}

func (a *app) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	tok := extractBearerToken(r)
	sess, acc, err := a.ensureStore().VerifySession(tok)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	prefs, _ := a.ensureStore().GetPreferences(acc.ID, acc.ID)
	writeJSON(w, 200, map[string]interface{}{
		"session":     sess,
		"account":     acc,
		"preferences": prefs,
	})
}

func (a *app) handleAuthRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	tok := extractBearerToken(r)
	if err := a.ensureStore().RevokeSession(tok); err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "op_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		MaxAge:   -1,
	})
	writeJSON(w, 200, map[string]string{"status": "revoked"})
}

func (a *app) handleMFAEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	tok := extractBearerToken(r)
	_, acc, err := a.ensureStore().VerifySession(tok)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	secret, uri, err := a.ensureStore().EnrollMFA(acc.ID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"secret": secret, "otpauth_uri": uri})
}

func (a *app) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	tok := extractBearerToken(r)
	var body struct {
		Code string `json:"code"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := a.ensureStore().VerifyAndActivateMFA(tok, body.Code); err != nil {
		status := 400
		if errors.Is(err, identity.ErrUnauthorized) {
			status = 401
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"mfa_verified": true})
}

func (a *app) handleAuthRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Email        string `json:"email"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	newToken, sess, err := a.ensureStore().RecoverAccount(body.Email, body.RecoveryCode)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"session_token": newToken,
		"session":       sess,
	})
}

// handleAccountSubroutes enforces object-level authorization (BOLA) on /api/v1/accounts/{accountID}/...
