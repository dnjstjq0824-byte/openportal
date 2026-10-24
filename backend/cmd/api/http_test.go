package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/openportal-search/backend/internal/identity"
	"example.com/openportal-search/backend/internal/search"
)

func TestHTTPBoundary(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore(), corsOrigin: "http://127.0.0.1:5173"}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/search", 400}, {"GET", "/api/v1/search?q=x&page=101", 400},
		{"GET", "/api/v1/search?q=x&lang=xx", 400}, {"POST", "/api/v1/search?q=x", 405},
		{"POST", "/api/v1/suggest?q=x", 405}, {"GET", "/api/v1/suggest?q=", 400},
		{"GET", "/admin", 404}, {"GET", "/api/v1/admin", 404}, {"GET", "/partner", 404},
		{"GET", "/api/v1/search?q=x&vertical=image", 403},
		{"GET", "/api/v1/search?q=x&safe_search=invalid", 400},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
	}
	if calls != 0 {
		t.Fatal("invalid requests reached upstream")
	}
	for _, path := range []string{"/api/v1/search?q=한글", "/api/v1/suggest?q=Open", "/readyz", "/api/v1/portal/config"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Origin", "https://untrusted.example")
		r.Header.Set("X-Request-Id", "trace-req-0001")
		a.routes().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("untrusted origin allowed")
		}
		if w.Header().Get("X-Request-Id") != "trace-req-0001" || w.Header().Get("X-Correlation-Id") != "trace-req-0001" {
			t.Fatalf("missing trace correlation headers: %v", w.Header())
		}
		if !json.Valid(w.Body.Bytes()) {
			t.Fatal("invalid JSON")
		}
	}
	upstream.Close()
	for _, path := range []string{"/api/v1/search?q=test", "/api/v1/suggest?q=test", "/readyz"} {
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 503 {
			t.Fatalf("%s unavailable returned %d", path, w.Code)
		}
	}
}

func TestGatewayAuthMFARecoveryAndBOLAEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_id":"fixture-001","_score":2.0,"_source":{"document_id":"fixture-001","title":"오픈포털 검색엔진 프로젝트","url":"https://example.org/intro","description":"예시","language":"ko"}}]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore(), corsOrigin: "http://127.0.0.1:5173"}
	h := a.routes()

	// 1. Separate console origin boundary check (Task 018)
	for _, host := range []string{"admin.openportal.local:8080", "partner.openportal.local"} {
		req := httptest.NewRequest("GET", "/api/v1/search?q=한글", nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("expected 403 for console host %s, got %d", host, w.Code)
		}
	}

	// 2. Query normalization & typo correction (Task 012, 014)
	{
		req := httptest.NewRequest("GET", "/api/v1/search?q=rjatordpswls", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res search.Response
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		if res.CorrectedQuery != "검색엔진" || res.RequestID == "" {
			t.Fatalf("expected corrected_query='검색엔진' and request_id, got %+v", res)
		}
	}

	// 3. Passkey/OIDC challenge & verification for Alice and Bob (Task 015)
	registerUser := func(email, name string) (token, accID, sessID string, recovery []string) {
		chBody := bytes.NewBufferString(`{"challenge_type":"passkey_register","email":"` + email + `"}`)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/auth/challenge", chBody))
		if w.Code != 200 {
			t.Fatalf("challenge failed: %d %s", w.Code, w.Body.String())
		}
		var ch identity.Challenge
		_ = json.Unmarshal(w.Body.Bytes(), &ch)

		vBody := bytes.NewBufferString(`{"challenge_id":"` + ch.ID + `","challenge_response":"signed:` + ch.ChallengeText + `","display_name":"` + name + `"}`)
		w2 := httptest.NewRecorder()
		h.ServeHTTP(w2, httptest.NewRequest("POST", "/api/v1/auth/verify", vBody))
		if w2.Code != 200 {
			t.Fatalf("verify failed: %d %s", w2.Code, w2.Body.String())
		}
		var out struct {
			SessionToken  string           `json:"session_token"`
			Session       identity.Session `json:"session"`
			Account       identity.Account `json:"account"`
			RecoveryCodes []string         `json:"recovery_codes"`
		}
		_ = json.Unmarshal(w2.Body.Bytes(), &out)
		return out.SessionToken, out.Account.ID, out.Session.ID, out.RecoveryCodes
	}

	tokAlice, idAlice, sessAlice, codesAlice := registerUser("alice@example.org", "Alice")
	tokBob, idBob, _, _ := registerUser("bob@example.org", "Bob")

	// 4. BOLA regression checks via HTTP API (Task 016)
	{
		// Alice tries to GET Bob's preferences -> 403
		req := httptest.NewRequest("GET", "/api/v1/accounts/"+idBob+"/preferences", nil)
		req.Header.Set("Authorization", "Bearer "+tokAlice)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("expected 403 on cross-account preferences GET, got %d", w.Code)
		}

		// Alice tries to PUT Bob's preferences -> 403
		req = httptest.NewRequest("PUT", "/api/v1/accounts/"+idBob+"/preferences", bytes.NewBufferString(`{"safe_search":"off","theme":"dark","language":"en"}`))
		req.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("expected 403 on cross-account preferences PUT, got %d", w.Code)
		}

		// Bob tries to revoke Alice's session -> 403
		req = httptest.NewRequest("DELETE", "/api/v1/accounts/"+idAlice+"/sessions/"+sessAlice, nil)
		req.Header.Set("Authorization", "Bearer "+tokBob)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("expected 403 on cross-account session DELETE, got %d", w.Code)
		}

		// Tenant site BOLA check
		a.idStore.SeedOrganizationSite("org-alice", idAlice, "owner", "site-alice", "https://alice.example.org")
		a.idStore.SeedOrganizationSite("org-bob", idBob, "owner", "site-bob", "https://bob.example.org")

		req = httptest.NewRequest("GET", "/api/v1/organizations/org-bob/sites/site-bob", nil)
		req.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("expected 403 on cross-tenant site GET, got %d", w.Code)
		}

		req = httptest.NewRequest("GET", "/api/v1/organizations/org-alice/sites/site-alice", nil)
		req.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 on own-tenant site GET, got %d", w.Code)
		}
	}

	// 5. Preferences & Recent searches opt-in flow (Task 012, 017)
	{
		// Opt-in to recent searches for Alice
		req := httptest.NewRequest("PUT", "/api/v1/accounts/"+idAlice+"/preferences", bytes.NewBufferString(`{"safe_search":"strict","theme":"dark","language":"ko","personalized_search":false,"recent_searches_opt_in":true,"search_correction_opt_in":true}`))
		req.Header.Set("Authorization", "Bearer "+tokAlice)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("update own preferences failed: %d %s", w.Code, w.Body.String())
		}

		// Perform a search authenticated as Alice
		sReq := httptest.NewRequest("GET", "/api/v1/search?q=검색엔진", nil)
		sReq.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, sReq)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}

		// Fetch recent searches
		rReq := httptest.NewRequest("GET", "/api/v1/accounts/"+idAlice+"/recent-searches", nil)
		rReq.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, rReq)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		var rOut struct {
			RecentSearches []string `json:"recent_searches"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &rOut)
		if len(rOut.RecentSearches) != 1 || rOut.RecentSearches[0] != "검색엔진" {
			t.Fatalf("expected recorded recent search, got %v", rOut.RecentSearches)
		}
	}

	// 6. MFA enrollment & verification, recovery, and session revocation (Task 015)
	{
		eReq := httptest.NewRequest("POST", "/api/v1/auth/mfa/enroll", nil)
		eReq.Header.Set("Authorization", "Bearer "+tokAlice)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, eReq)
		if w.Code != 200 {
			t.Fatalf("mfa enroll failed: %d", w.Code)
		}
		var mfaOut struct {
			Secret string `json:"secret"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &mfaOut)
		code, _ := identity.GenerateTOTP(mfaOut.Secret, time.Now())

		vReq := httptest.NewRequest("POST", "/api/v1/auth/mfa/verify", bytes.NewBufferString(`{"code":"`+code+`"}`))
		vReq.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, vReq)
		if w.Code != 200 {
			t.Fatalf("mfa verify failed: %d %s", w.Code, w.Body.String())
		}

		// Account recovery revokes tokAlice and returns a new token
		recReq := httptest.NewRequest("POST", "/api/v1/auth/recover", bytes.NewBufferString(`{"email":"alice@example.org","recovery_code":"`+codesAlice[0]+`"}`))
		w = httptest.NewRecorder()
		h.ServeHTTP(w, recReq)
		if w.Code != 200 {
			t.Fatalf("account recover failed: %d %s", w.Code, w.Body.String())
		}

		// Old token must now return 401 on /api/v1/auth/session
		sReq := httptest.NewRequest("GET", "/api/v1/auth/session", nil)
		sReq.Header.Set("Authorization", "Bearer "+tokAlice)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, sReq)
		if w.Code != 401 {
			t.Fatalf("expected 401 for revoked old token, got %d", w.Code)
		}

		// Revoke Bob's session explicitly
		revReq := httptest.NewRequest("POST", "/api/v1/auth/revoke", nil)
		revReq.Header.Set("Authorization", "Bearer "+tokBob)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, revReq)
		if w.Code != 200 {
			t.Fatalf("revoke failed: %d", w.Code)
		}
	}
}
