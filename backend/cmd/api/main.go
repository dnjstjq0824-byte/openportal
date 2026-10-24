package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/openportal-search/backend/internal/config"
	"example.com/openportal-search/backend/internal/identity"
	"example.com/openportal-search/backend/internal/search"
)

type app struct {
	c          *search.Client
	idStore    *identity.Store
	corsOrigin string
}

var validTraceID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

func newTraceID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "tr-" + hex.EncodeToString(b)
}

func resolveTraceID(r *http.Request) string {
	for _, h := range []string{"X-Request-Id", "X-Correlation-Id"} {
		v := strings.TrimSpace(r.Header.Get(h))
		if validTraceID.MatchString(v) {
			return v
		}
	}
	return newTraceID()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *app) ensureStore() *identity.Store {
	if a.idStore == nil {
		a.idStore = identity.NewStore()
	}
	return a.idStore
}

func (a *app) withGatewayHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := resolveTraceID(r)
		w.Header().Set("X-Request-Id", traceID)
		w.Header().Set("X-Correlation-Id", traceID)
		if a.corsOrigin != "" && r.Header.Get("Origin") == a.corsOrigin {
			w.Header().Set("Access-Control-Allow-Origin", a.corsOrigin)
			w.Header().Set("Vary", "Origin")
		}
		// Separate host/domain boundary enforcement for admin/partner origins (Task 018)
		host := strings.ToLower(strings.Split(r.Host, ":")[0])
		if strings.HasPrefix(host, "admin.") || strings.HasPrefix(host, "partner.") {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error":      "console origin access denied: privileged MFA and tenant authorization required",
				"code":       "CONSOLE_BOUNDARY_DENIED",
				"request_id": traceID,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.corsOrigin != "" && r.Header.Get("Origin") == a.corsOrigin {
			w.Header().Set("Access-Control-Allow-Origin", a.corsOrigin)
			w.Header().Set("Vary", "Origin")
		}
		next(w, r)
	}
}

func validateQ(q string) error {
	_, err := search.NormalizeQuery(q)
	return err
}

func pageParam(s string) (int, error) {
	if s == "" {
		return 1, nil
	}
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 100 {
		return 0, errors.New("page must be 1..100")
	}
	return p, nil
}

func extractBearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if c, err := r.Cookie("op_session"); err == nil && c.Value != "" {
		return strings.TrimSpace(c.Value)
	}
	return ""
}

func (a *app) search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	normalizedQ, err := search.NormalizeQuery(r.URL.Query().Get("q"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	page, err := pageParam(r.URL.Query().Get("page"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang != "" && lang != "ko" && lang != "en" {
		writeJSON(w, 400, map[string]string{"error": "lang must be ko or en"})
		return
	}
	safeSearch := r.URL.Query().Get("safe_search")
	if safeSearch != "" && safeSearch != "strict" && safeSearch != "moderate" && safeSearch != "off" {
		writeJSON(w, 400, map[string]string{"error": "safe_search must be strict, moderate, or off"})
		return
	}
	vertical := r.URL.Query().Get("vertical")
	if vertical != "" && vertical != "web" {
		writeJSON(w, 403, map[string]string{
			"error": "requested vertical is disabled by feature flag until Phase 5 licensing gate",
			"code":  "VERTICAL_DISABLED",
		})
		return
	}
	result, err := a.c.Search(r.Context(), normalizedQ, lang, page, 10)
	if err != nil {
		log.Print("search backend failed")
		writeJSON(w, 503, map[string]string{"error": "search temporarily unavailable"})
		return
	}
	traceID := w.Header().Get("X-Request-Id")
	result.RequestID = traceID
	result.CorrectedQuery = search.SuggestCorrection(normalizedQ)
	if safeSearch != "" {
		result.SafeSearch = safeSearch
	}
	if vertical != "" {
		result.Vertical = vertical
	}
	if tok := extractBearerToken(r); tok != "" {
		if _, acc, authErr := a.ensureStore().VerifySession(tok); authErr == nil {
			a.ensureStore().RecordRecentSearch(acc.ID, normalizedQ)
		}
	}
	writeJSON(w, 200, result)
}

func (a *app) suggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	normalizedQ, err := search.NormalizeQuery(r.URL.Query().Get("q"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	items, err := a.c.Suggest(r.Context(), normalizedQ)
	if err != nil {
		log.Print("suggest backend failed")
		writeJSON(w, 503, map[string]string{"error": "suggest temporarily unavailable"})
		return
	}
	resp := map[string]interface{}{"suggestions": items}
	if corr := search.SuggestCorrection(normalizedQ); corr != "" {
		resp["corrected_query"] = corr
	}
	writeJSON(w, 200, resp)
}

func (a *app) portalConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"tabs": search.DefaultVerticalTabs(),
		"features": map[string]bool{
			"recent_searches_opt_in": true,
			"query_correction":       true,
			"passkey_oidc_auth":      true,
			"totp_mfa":               true,
		},
		"consoles": map[string]string{
			"admin":   "denied_by_default",
			"partner": "denied_by_default",
		},
	})
}

func decodeJSONBody(r *http.Request, dst any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func (a *app) handleAuthChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
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
	var body struct {
		ChallengeID       string `json:"challenge_id"`
		ChallengeResponse string `json:"challenge_response"`
		DisplayName       string `json:"display_name"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	token, sess, acc, codes, err := a.ensureStore().CompleteChallenge(body.ChallengeID, body.ChallengeResponse, body.DisplayName)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "op_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
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
func (a *app) handleAccountSubroutes(w http.ResponseWriter, r *http.Request) {
	tok := extractBearerToken(r)
	_, actor, err := a.ensureStore().VerifySession(tok)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/accounts/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	targetAccountID := parts[0]
	resource := parts[1]

	switch resource {
	case "preferences":
		if r.Method == http.MethodGet {
			prefs, pErr := a.ensureStore().GetPreferences(actor.ID, targetAccountID)
			if errors.Is(pErr, identity.ErrForbidden) {
				writeJSON(w, 403, map[string]string{"error": "forbidden: cross-account access denied (BOLA)"})
				return
			}
			if pErr != nil {
				writeJSON(w, 404, map[string]string{"error": pErr.Error()})
				return
			}
			writeJSON(w, 200, prefs)
			return
		}
		if r.Method == http.MethodPut {
			var next identity.Preferences
			if err := decodeJSONBody(r, &next); err != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
				return
			}
			updated, uErr := a.ensureStore().UpdatePreferences(actor.ID, targetAccountID, next)
			if errors.Is(uErr, identity.ErrForbidden) {
				writeJSON(w, 403, map[string]string{"error": "forbidden: cross-account modification denied (BOLA)"})
				return
			}
			if errors.Is(uErr, identity.ErrInvalidInput) {
				writeJSON(w, 400, map[string]string{"error": uErr.Error()})
				return
			}
			if uErr != nil {
				writeJSON(w, 404, map[string]string{"error": uErr.Error()})
				return
			}
			writeJSON(w, 200, updated)
			return
		}
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	case "recent-searches":
		if r.Method == http.MethodGet {
			items, rErr := a.ensureStore().GetRecentSearches(actor.ID, targetAccountID)
			if errors.Is(rErr, identity.ErrForbidden) {
				writeJSON(w, 403, map[string]string{"error": "forbidden: cross-account access denied (BOLA)"})
				return
			}
			writeJSON(w, 200, map[string]interface{}{"recent_searches": items})
			return
		}
		if r.Method == http.MethodDelete {
			if cErr := a.ensureStore().ClearRecentSearches(actor.ID, targetAccountID); errors.Is(cErr, identity.ErrForbidden) {
				writeJSON(w, 403, map[string]string{"error": "forbidden: cross-account deletion denied (BOLA)"})
				return
			}
			writeJSON(w, 200, map[string]string{"status": "cleared"})
			return
		}
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	case "sessions":
		if len(parts) == 3 && r.Method == http.MethodDelete {
			if actor.ID != targetAccountID {
				writeJSON(w, 403, map[string]string{"error": "forbidden: cross-account session revocation denied (BOLA)"})
				return
			}
			err := a.ensureStore().RevokeSessionByID(actor.ID, parts[2])
			if errors.Is(err, identity.ErrForbidden) {
				writeJSON(w, 403, map[string]string{"error": "forbidden: session belongs to another account"})
				return
			}
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, map[string]string{"status": "revoked"})
			return
		}
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

// handleOrgSubroutes enforces tenant & object-level authorization (BOLA) on /api/v1/organizations/{orgID}/sites/{siteID}
func (a *app) handleOrgSubroutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	tok := extractBearerToken(r)
	_, actor, err := a.ensureStore().VerifySession(tok)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/organizations/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 || parts[1] != "sites" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	site, sErr := a.ensureStore().GetOrganizationSite(actor.ID, parts[0], parts[2])
	if errors.Is(sErr, identity.ErrForbidden) {
		writeJSON(w, 403, map[string]string{"error": "forbidden: tenant isolation boundary violation (BOLA)"})
		return
	}
	if sErr != nil {
		writeJSON(w, 404, map[string]string{"error": sErr.Error()})
		return
	}
	writeJSON(w, 200, site)
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		if _, err := a.c.Search(r.Context(), "__readiness__", "", 1, 1); err != nil {
			writeJSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/api/v1/search", a.cors(a.search))
	mux.HandleFunc("/api/v1/suggest", a.cors(a.suggest))
	mux.HandleFunc("/api/v1/portal/config", a.cors(a.portalConfig))
	mux.HandleFunc("/api/v1/auth/challenge", a.cors(a.handleAuthChallenge))
	mux.HandleFunc("/api/v1/auth/verify", a.cors(a.handleAuthVerify))
	mux.HandleFunc("/api/v1/auth/session", a.cors(a.handleAuthSession))
	mux.HandleFunc("/api/v1/auth/revoke", a.cors(a.handleAuthRevoke))
	mux.HandleFunc("/api/v1/auth/mfa/enroll", a.cors(a.handleMFAEnroll))
	mux.HandleFunc("/api/v1/auth/mfa/verify", a.cors(a.handleMFAVerify))
	mux.HandleFunc("/api/v1/auth/recover", a.cors(a.handleAuthRecover))
	mux.HandleFunc("/api/v1/accounts/", a.cors(a.handleAccountSubroutes))
	mux.HandleFunc("/api/v1/organizations/", a.cors(a.handleOrgSubroutes))
	return a.withGatewayHeaders(mux)
}

func main() {
	cfg := config.Load()
	a := &app{
		c:          search.New(cfg.OpenSearchURL, cfg.OpenSearchIndex, cfg.OpenSearchUsername, cfg.OpenSearchPassword),
		idStore:    identity.NewStore(),
		corsOrigin: cfg.CORSOrigin,
	}
	srv := &http.Server{Addr: net.JoinHostPort(cfg.ListenHost, cfg.Port), Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Printf("openportal search api listening port=%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
