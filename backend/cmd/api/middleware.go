package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
)

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

// requireAuth wraps a handler with session authentication. Unauthenticated
// requests receive 401. The authenticated account is stored in the request
// context under the actorAccountIDKey for downstream BOLA checks.
func (a *app) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := extractBearerToken(r)
		if tok == "" {
			writeJSON(w, 401, map[string]string{"error": "unauthenticated", "code": "AUTH_REQUIRED"})
			return
		}
		_, acc, err := a.ensureStore().VerifySession(tok)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthenticated", "code": "SESSION_INVALID"})
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, actorAccountIDKey, acc.ID)
		ctx = context.WithValue(ctx, actorAccountKey, acc)
		next(w, r.WithContext(ctx))
	}
}

// moderationAuth wraps moderation mutation endpoints. It requires an
// authenticated session. Anonymous reporting/appeal is intentionally
// disabled until operator review (G021).

// moderationAuth wraps moderation mutation endpoints. It requires an
// authenticated session. Anonymous reporting/appeal is intentionally
// disabled until operator review (G021).
func (a *app) moderationAuth(next http.HandlerFunc) http.HandlerFunc {
	return a.requireAuth(next)
}

func (a *app) requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := r.Context().Value(actorAccountKey).(identity.Account)
		if !ok {
			writeJSON(w, 401, map[string]string{"error": "unauthenticated", "code": "AUTH_REQUIRED"})
			return
		}
		for _, rr := range actor.Roles {
			if rr == role {
				next(w, r)
				return
			}
		}
		writeJSON(w, 403, map[string]string{"error": "forbidden: insufficient role", "code": "ROLE_REQUIRED"})
	}
}

// A-6: the fabricated superplatform catalog was removed. No app backend exists,
// so the endpoint honestly reports not-implemented.
