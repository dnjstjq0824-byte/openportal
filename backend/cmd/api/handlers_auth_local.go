package main

import (
	"net/http"
)

func (a *app) handleLocalRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	acc, err := a.ensureStore().RegisterLocalAccount(body.Email, body.Password, body.DisplayName)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	tok, err := a.ensureEmailService().SendRegistrationVerification(r.Context(), acc.Email, acc.DisplayName)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to send verification email: " + err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{
		"status":     "pending_verification",
		"account_id": acc.ID,
		"email":      acc.Email,
		"code":       tok.Code,
		"message":    "인증 이메일이 발송되었습니다. 6자리 코드를 입력하여 가입을 완료하세요.",
	})
}

func (a *app) handleEmailVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := a.ensureEmailService().VerifyRegistrationToken(body.Email, body.Code); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := a.ensureStore().VerifyAccountEmail(body.Email); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "verified",
		"email":   body.Email,
		"message": "이메일 인증이 완료되었습니다. 이제 로그인할 수 있습니다.",
	})
}

func (a *app) handleLocalLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	sess, token, acc, err := a.ensureStore().AuthenticateLocal(body.Email, body.Password)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":       "authenticated",
		"token":        token,
		"session_id":   sess.ID,
		"account_id":   acc.ID,
		"email":        acc.Email,
		"display_name": acc.DisplayName,
		"roles":        acc.Roles,
		"expires_at":   sess.ExpiresAt,
	})
}

func (a *app) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	resetTok, err := a.ensureEmailService().SendPasswordReset(r.Context(), body.Email)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "reset_email_sent",
		"email":   body.Email,
		"token":   resetTok.Token,
		"message": "비밀번호 재설정 이메일이 발송되었습니다.",
	})
}

func (a *app) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Email       string `json:"email"`
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := a.ensureEmailService().VerifyPasswordResetToken(body.Email, body.Token); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := a.ensureStore().UpdatePassword(body.Email, body.NewPassword); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "password_updated",
		"email":   body.Email,
		"message": "비밀번호가 안전하게 변경되었습니다. 새 비밀번호로 로그인하세요.",
	})
}
