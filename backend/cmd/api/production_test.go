package main

import (
	"net/http/httptest"
	"testing"
)

func TestProductionDeniesUnverifiedIdentityAndMutationRoutes(t *testing.T) {
	a := &app{}
	// These mutation/operational paths remain behind productionRoutes allowlist (404)
	// until the corresponding phase gate and independent review are completed.
	denyPaths := []string{
		"/api/v1/accounts/victim/preferences",
		"/api/v1/organizations/x",
		"/api/v1/vertical/poi/correct",
		"/api/v1/vertical/shopping/report",
		"/api/v1/ai/gate-status",
		"/api/v1/ai/synthesize",
		"/api/v1/ai/feedback",
		"/api/v1/ads/gate-status",
		"/api/v1/ads/auction",
		"/api/v1/webmaster/verify",
		"/api/v1/webmaster/sitemap",
		"/api/v1/ops/gate-status",
		"/api/v1/ops/pam/elevate",
		"/api/v1/privacy/dsar",
		"/api/v1/ops/dual-approval",
		"/api/v1/launch/gate-status",
		"/api/v1/launch/benchmarks",
		"/api/v1/launch/dr-drill",
		"/api/v1/launch/defects",
		"/api/v1/control/moderation/report",
		"/api/v1/control/moderation/appeal",
		"/admin", "/partner",
	}
	for _, path := range denyPaths {
		for _, method := range []string{"GET", "POST", "OPTIONS"} {
			response := httptest.NewRecorder()
			a.productionRoutes().ServeHTTP(response, httptest.NewRequest(method, path, nil))
			if response.Code != 404 {
				t.Fatalf("%s %s: %d", method, path, response.Code)
			}
		}
	}

	// Auth entry points are intentionally exposed in productionRoutes so users
	// can register, sign in, and manage sessions. challenge/verify accept POST
	// only; session/revoke/mfa/recover require a valid session cookie.
	authPaths := []string{
		"/api/v1/auth/challenge",
		"/api/v1/auth/verify",
		"/api/v1/auth/session",
		"/api/v1/auth/revoke",
		"/api/v1/auth/mfa/enroll",
		"/api/v1/auth/mfa/verify",
		"/api/v1/auth/recover",
	}
	for _, path := range authPaths {
		response := httptest.NewRecorder()
		a.productionRoutes().ServeHTTP(response, httptest.NewRequest("OPTIONS", path, nil))
		if response.Code == 404 {
			t.Fatalf("OPTIONS %s should not be 404, got %d", path, response.Code)
		}
	}
}
