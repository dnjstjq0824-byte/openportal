package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// classifyProbeError maps a dependency failure to a short public cause without
// leaking connection strings or internal topology. Full errors go to the log.
func classifyProbeError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded"):
		return "timeout"
	case strings.Contains(msg, "connection refused"):
		return "connection_refused"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dns"):
		return "dns_error"
	case strings.Contains(msg, "password") || strings.Contains(msg, "authentication"):
		return "auth_failed"
	case strings.Contains(msg, "index_not_found") || strings.Contains(msg, "404"):
		return "index_missing"
	default:
		return "unreachable"
	}
}

// readyz verifies live dependencies: OpenSearch is always required, and
// PostgreSQL is required whenever a database handle is configured (B-2).
// Failures return 503 with the failing check and a short cause.
func (a *app) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := a.c.Search(ctx, "__readiness__", "", 1, 1); err != nil {
		log.Printf("readyz: opensearch probe failed: %v", err)
		writeJSON(w, 503, map[string]string{"status": "unavailable", "check": "opensearch", "cause": classifyProbeError(err)})
		return
	}
	postgres := "not_configured"
	if a.db != nil {
		if err := a.db.PingContext(ctx); err != nil {
			log.Printf("readyz: postgres ping failed: %v", err)
			writeJSON(w, 503, map[string]string{"status": "unavailable", "check": "postgres", "cause": classifyProbeError(err), "opensearch": "ok"})
			return
		}
		postgres = "ok"
	}
	writeJSON(w, 200, map[string]string{"status": "ready", "opensearch": "ok", "postgres": postgres})
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/readyz", a.readyz)
	mux.HandleFunc("/metrics", a.handleMetrics)
	// Public read paths — no authentication required.
	mux.HandleFunc("/api/v1/search", a.cors(a.search))
	mux.HandleFunc("/api/v1/suggest", a.cors(a.suggest))
	// Legacy /v1 aliases kept for the public autocomplete contract.
	mux.HandleFunc("/v1/search", a.cors(a.search))
	mux.HandleFunc("/v1/search/suggest", a.cors(a.suggest))
	mux.HandleFunc("/api/v1/portal/config", a.cors(a.portalConfig))
	// Open Scanner Superplatform Endpoints
	mux.HandleFunc("/api/v1/superplatform/apps", a.cors(a.handleSuperplatformApps))
	mux.HandleFunc("/api/v1/superplatform/mail", a.cors(a.handleSuperplatformMail))
	mux.HandleFunc("/api/v1/superplatform/developer", a.cors(a.handleSuperplatformDeveloper))
	// Authentication paths — challenge/verify are public entry points;
	// session/revoke/MFA/recover require a valid session.
	mux.HandleFunc("/api/v1/auth/register", a.cors(a.handleLocalRegister))
	mux.HandleFunc("/api/v1/auth/login", a.cors(a.handleLocalLogin))
	mux.HandleFunc("/api/v1/auth/email/verify", a.cors(a.handleEmailVerify))
	mux.HandleFunc("/api/v1/auth/password/reset-request", a.cors(a.handlePasswordResetRequest))
	mux.HandleFunc("/api/v1/auth/password/reset-confirm", a.cors(a.handlePasswordResetConfirm))
	mux.HandleFunc("/api/v1/auth/challenge", a.cors(a.handleAuthChallenge))
	mux.HandleFunc("/api/v1/auth/verify", a.cors(a.handleAuthVerify))
	mux.HandleFunc("/api/v1/auth/session", a.cors(a.requireAuth(a.handleAuthSession)))
	mux.HandleFunc("/api/v1/auth/revoke", a.cors(a.requireAuth(a.handleAuthRevoke)))
	mux.HandleFunc("/api/v1/auth/mfa/enroll", a.cors(a.requireAuth(a.handleMFAEnroll)))
	mux.HandleFunc("/api/v1/auth/mfa/verify", a.cors(a.requireAuth(a.handleMFAVerify)))
	mux.HandleFunc("/api/v1/auth/recover", a.cors(a.handleAuthRecover))
	// Account & organization subroutes — BOLA enforced inside handlers.
	mux.HandleFunc("/api/v1/accounts/", a.cors(a.requireAuth(a.handleAccountSubroutes)))
	mux.HandleFunc("/api/v1/organizations/", a.cors(a.requireAuth(a.handleOrgSubroutes)))
	// Moderation write paths — require authentication (G021: no anonymous access).
	mux.HandleFunc("/api/v1/control/moderation/report", a.cors(a.moderationAuth(a.handleModerationReport)))
	mux.HandleFunc("/api/v1/control/moderation/appeal", a.cors(a.moderationAuth(a.handleModerationAppeal)))
	mux.HandleFunc("/api/v1/control/moderation/review", a.cors(a.requireAuth(a.requireRole("admin", a.handleModerationReview))))
	// Operational / admin / partner / vertical / ads / privacy / launch paths
	// remain behind productionRoutes allowlist until respective phase gates pass.
	mux.HandleFunc("/api/v1/vertical/gate-status", a.cors(a.handleVerticalGateStatus))
	mux.HandleFunc("/api/v1/vertical/search", a.cors(a.handleVerticalSearch))
	mux.HandleFunc("/api/v1/vertical/poi/correct", a.cors(a.handlePOICorrection))
	mux.HandleFunc("/api/v1/vertical/shopping/report", a.cors(a.handleShoppingDiscrepancyReport))
	mux.HandleFunc("/api/v1/vertical/news/syndicate", a.cors(a.handleNewsSyndicate))
	mux.HandleFunc("/api/v1/vertical/shopping/feed", a.cors(a.handleShoppingFeedIngest))
	mux.HandleFunc("/api/v1/ai/gate-status", a.cors(a.handleAIGateStatus))
	mux.HandleFunc("/api/v1/ai/synthesize", a.cors(a.handleAISynthesize))
	mux.HandleFunc("/api/v1/ai/feedback", a.cors(a.handleAIFeedback))
	mux.HandleFunc("/api/v1/ads/gate-status", a.cors(a.handleAdsGateStatus))
	mux.HandleFunc("/api/v1/ads/click", a.cors(a.handleAdsClick))
	mux.HandleFunc("/api/v1/ads/reconcile", a.cors(a.handleAdsReconcile))
	mux.HandleFunc("/api/v1/webmaster/verify", a.cors(a.handleWebmasterVerify))
	mux.HandleFunc("/api/v1/webmaster/sitemap", a.cors(a.handleWebmasterSitemap))
	mux.HandleFunc("/api/v1/ads/auction", a.cors(a.handleAdsAuction))
	mux.HandleFunc("/api/v1/ops/gate-status", a.cors(a.handleOpsGateStatus))
	mux.HandleFunc("/api/v1/ops/pam/elevate", a.cors(a.handlePAMElevate))
	mux.HandleFunc("/api/v1/ops/idp-verify", a.cors(a.handleIdPVerify))
	mux.HandleFunc("/api/v1/privacy/dsar", a.cors(a.handlePrivacyDSAR))
	mux.HandleFunc("/api/v1/ops/dual-approval", a.cors(a.handleDualApproval))
	mux.HandleFunc("/api/v1/launch/gate-status", a.cors(a.handleLaunchGateStatus))
	mux.HandleFunc("/api/v1/launch/benchmarks", a.cors(a.handleLaunchBenchmarks))
	mux.HandleFunc("/api/v1/launch/dr-drill", a.cors(a.handleLaunchDRDrill))
	mux.HandleFunc("/api/v1/launch/defects", a.cors(a.handleLaunchDefects))
	return a.withGatewayHeaders(mux)
}

// productionRoutes exposes the read-only index API. Browser accounts and mail
// use the persistent web gateway; prototype handlers stay inaccessible here.
//
// B-2 note: only OpenSearch-backed read handlers are exposed here. Identity,
// moderation, vertical, ads, ops, and launch handlers run on in-memory stores
// (identity.Store, DSARManager, etc.) with no PostgreSQL wiring, so they stay
// behind the allowlist until a DB-backed implementation lands.
func (a *app) productionRoutes() http.Handler {
	internal := a.routes()
	limit := newRateLimiter(120, time.Minute)
	// B-5: telemetry wraps the production tree, so allowlist rejections (404/429)
	// are counted alongside served routes.
	return a.withTelemetry(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz", "/metrics":
			internal.ServeHTTP(w, r)
		case "/api/v1/search", "/api/v1/suggest", "/v1/search", "/v1/search/suggest":
			if ok, retry := limit.allow(clientIP(r), time.Now()); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry/time.Second)+1))
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded", "code": "RATE_LIMITED"})
				return
			}
			internal.ServeHTTP(w, r)
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint unavailable", "code": "ENDPOINT_UNAVAILABLE"})
		}
	}))
}
