package main

import (
	"net/http/httptest"
	"testing"
)

func TestProductionDeniesUnverifiedIdentityAndMutationRoutes(t *testing.T) {
	a := &app{}
	// All unverified identity, mutation, and operational paths remain behind
	// productionRoutes allowlist (404) until persistent storage and phase gates pass.
	denyPaths := []string{
		"/api/v1/auth/challenge",
		"/api/v1/auth/verify",
		"/api/v1/auth/session",
		"/api/v1/auth/revoke",
		"/api/v1/auth/mfa/enroll",
		"/api/v1/auth/mfa/verify",
		"/api/v1/auth/recover",
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
		"/api/v1/control/moderation/review",
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
}
