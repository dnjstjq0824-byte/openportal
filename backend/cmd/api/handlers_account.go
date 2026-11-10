package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
)

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
