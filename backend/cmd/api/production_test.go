package main

import (
	"net/http/httptest"
	"testing"
)

func TestProductionDeniesUnverifiedIdentityAndMutationRoutes(t *testing.T) {
	a := &app{}
	for _, path := range []string{"/api/v1/auth/challenge", "/api/v1/auth/verify", "/api/v1/auth/recover", "/api/v1/accounts/victim/preferences", "/api/v1/organizations/x", "/api/v1/control/moderation/report", "/api/v1/vertical/poi/correct", "/admin", "/partner"} {
		for _, method := range []string{"GET", "POST", "OPTIONS"} {
			response := httptest.NewRecorder()
			a.productionRoutes().ServeHTTP(response, httptest.NewRequest(method, path, nil))
			if response.Code != 404 {
				t.Fatalf("%s %s: %d", method, path, response.Code)
			}
		}
	}
}
