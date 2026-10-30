package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"example.com/openportal-search/backend/internal/identity"
	"example.com/openportal-search/backend/internal/search"
	"example.com/openportal-search/backend/internal/vertical"
)

// registerTestUser performs the OIDC challenge/verify flow and returns a session
// token for the given email/display name. Used by tests that need an authenticated
// session (e.g. moderation endpoints which now require auth per G021).
func registerTestUser(t *testing.T, h http.Handler, email, name string) string {
	t.Helper()
	chBody := bytes.NewBufferString(`{"challenge_type":"passkey_register","email":"` + email + `"}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/auth/challenge", chBody))
	if w.Code != 200 {
		t.Fatalf("challenge failed: %d %s", w.Code, w.Body.String())
	}
	var ch identity.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}

	vBody := bytes.NewBufferString(`{"challenge_id":"` + ch.ID + `","challenge_response":"signed:` + ch.ChallengeText + `","display_name":"` + name + `"}`)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest("POST", "/api/v1/auth/verify", vBody))
	if w2.Code != 200 {
		t.Fatalf("verify failed: %d %s", w2.Code, w2.Body.String())
	}
	var out struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	return out.SessionToken
}

func TestHTTPBoundary(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
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
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
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

func TestRankingAbuseAndModerationEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":2},"hits":[
			{"_id":"doc-clean","_score":2.5,"_source":{"document_id":"doc-clean","title":"정상 기술 문서","url":"https://example.org/tech","description":"클린 내용","language":"ko"}},
			{"_id":"doc-adult","_score":2.8,"_source":{"document_id":"doc-adult","title":"성인인증 음란물 야동","url":"https://adult.test/xxx","description":"유해","language":"ko"}}
		]}}`))
	}))
	defer upstream.Close()

	a := &app{
		c:          search.New(upstream.URL, "documents_v1", "", ""),
		idStore:    identity.NewStoreWithVerifier(fixtureVerifier{}),
		corsOrigin: "http://127.0.0.1:5173",
	}
	h := a.routes()

	// 1. Abuse / PII rejection (Task 042)
	{
		req := httptest.NewRequest("GET", "/api/v1/search?q="+url.QueryEscape("주민등록번호 950101-1234567 조회"), nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("expected 400 for PII leak query, got %d", w.Code)
		}
		var errResp struct {
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		if errResp.Code != "pii_leakage" {
			t.Fatalf("expected code 'pii_leakage', got %s", errResp.Code)
		}
	}

	// 2. SafeSearch Strict Mode Content Filtering (Task 044)
	{
		req := httptest.NewRequest("GET", "/api/v1/search?q=테스트&safe_search=strict", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var sResp search.Response
		_ = json.Unmarshal(w.Body.Bytes(), &sResp)
		if len(sResp.Hits) != 1 || sResp.Hits[0].DocumentID != "doc-clean" {
			t.Fatalf("expected only clean hit in strict safesearch, got %d hits", len(sResp.Hits))
		}
		// Regression: reranking must preserve hit language metadata.
		if sResp.Hits[0].Language != "ko" {
			t.Fatalf("reranker dropped language field, got %q", sResp.Hits[0].Language)
		}
	}

	// 3. Moderation Case Report (Task 044) — requires authenticated session (G021)
	var caseID string
	var tokMod string
	{
		// Authenticate as moderator first; anonymous moderation access is disabled.
		tokMod = registerTestUser(t, h, "moderator@example.org", "Moderator")
		body := `{"document_id":"doc-malware-01","reporter":"moderator@example.org","category":"malware","details":"다운로드 시 랜섬웨어 감염"}`
		req := httptest.NewRequest("POST", "/api/v1/control/moderation/report", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokMod)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("moderation report failed: %d %s", w.Code, w.Body.String())
		}
		var cOut struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &cOut)
		if cOut.ID == "" || cOut.Status != "pending" {
			t.Fatalf("invalid report response: %+v", cOut)
		}
		caseID = cOut.ID
	}

	// 4. Moderation Case Appeal (Task 044) — requires authenticated session (G021)
	{
		body := `{"case_id":"` + caseID + `","appellant":"moderator@example.org","reason":"보안 분석용 샘플 코드로 확인되었습니다"}`
		req := httptest.NewRequest("POST", "/api/v1/control/moderation/appeal", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokMod)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("moderation appeal failed: %d %s", w.Code, w.Body.String())
		}
		var aOut struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &aOut)
		if aOut.ID == "" || aOut.Status != "pending" {
			t.Fatalf("invalid appeal response: %+v", aOut)
		}
	}
}

func TestPhase5VerticalGateAndEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
	h := a.routes()

	// 1. Vertical gate status endpoint
	{
		req := httptest.NewRequest("GET", "/api/v1/vertical/gate-status", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for gate status, got %d", w.Code)
		}
		var rep struct {
			Phase           int      `json:"phase"`
			Decision        string   `json:"decision"`
			GateReason      string   `json:"gate_reason"`
			UnresolvedRisks []string `json:"unresolved_risks"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
			t.Fatalf("failed to decode gate status JSON: %v", err)
		}
		if rep.Phase != 5 {
			t.Errorf("expected phase 5, got %d", rep.Phase)
		}
		if rep.Decision != "NO-GO" {
			t.Errorf("expected NO-GO decision pending legal sign-off, got %s", rep.Decision)
		}
		if len(rep.UnresolvedRisks) == 0 {
			t.Errorf("expected unresolved risks list, got 0")
		}
	}

	// 2. POI Correction endpoint
	{
		_ = a.ensurePOIManager().RegisterPlace(vertical.POIPlace{
			ID:       "poi-test-1",
			Name:     "테스트 장소",
			Location: vertical.GeoPoint{Latitude: 37.5, Longitude: 127.0},
		})
		body := `{"place_id":"poi-test-1","field_name":"phone","old_value":"02-000-0000","new_value":"02-111-1111","account_id":"acc-test"}`
		req := httptest.NewRequest("POST", "/api/v1/vertical/poi/correct", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("poi correction failed: %d %s", w.Code, w.Body.String())
		}
	}

	// 3. Shopping Discrepancy Report endpoint
	{
		_, _ = a.ensureShoppingManager().IngestOffer(vertical.ProductOffer{
			ID:            "offer-test-1",
			Title:         "테스트 상품",
			Price:         10000,
			Currency:      "KRW",
			FeedUpdatedAt: time.Now(),
		})
		body := `{"offer_id":"offer-test-1","reported_price":10000,"actual_price":12000,"reason":"결제 시 가격 인상","account_id":"acc-test"}`
		req := httptest.NewRequest("POST", "/api/v1/vertical/shopping/report", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("shopping discrepancy report failed: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPhase6AIEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
	h := a.routes()

	// 1. AI Gate Status
	{
		req := httptest.NewRequest("GET", "/api/v1/ai/gate-status", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for AI gate status, got %d", w.Code)
		}
		var rep struct {
			Phase    int    `json:"phase"`
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
			t.Fatalf("failed to decode gate status JSON: %v", err)
		}
		if rep.Phase != 6 || rep.Decision != "NO-GO" {
			t.Errorf("expected phase 6 NO-GO, got phase=%d, decision=%s", rep.Phase, rep.Decision)
		}
	}

	// 2. AI Synthesize Answer (RAG)
	{
		body := `{
			"query": "OpenPortal architecture overview",
			"passages": [
				{
					"chunk_id": "c1",
					"document_id": "doc-1",
					"source_url": "https://openportal.org/docs",
					"heading_context": "Overview",
					"content": "OpenPortal integrates PostgreSQL and OpenSearch for high-throughput search queries."
				}
			]
		}`
		req := httptest.NewRequest("POST", "/api/v1/ai/synthesize", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("AI synthesize failed: %d %s", w.Code, w.Body.String())
		}
		var sOut struct {
			Answer            string `json:"answer"`
			FallbackTriggered bool   `json:"fallback_triggered"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &sOut); err != nil {
			t.Fatalf("failed to decode synthesis response: %v", err)
		}
		if sOut.FallbackTriggered {
			t.Errorf("expected successful synthesis, but fallback triggered")
		}
	}

	// 3. AI Feedback
	{
		body := `{"query_hash":"hash-1234","is_helpful":true,"category":"helpful","comment":"정확한 답변 감사합니다."}`
		req := httptest.NewRequest("POST", "/api/v1/ai/feedback", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("AI feedback failed: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPhase7AdsAndWebmasterEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
	h := a.routes()

	// 1. Ads Gate Status
	{
		req := httptest.NewRequest("GET", "/api/v1/ads/gate-status", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for Ads gate status, got %d", w.Code)
		}
		var rep struct {
			Phase    int    `json:"phase"`
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
			t.Fatalf("failed to decode gate status JSON: %v", err)
		}
		if rep.Phase != 7 || rep.Decision != "NO-GO" {
			t.Errorf("expected phase 7 NO-GO, got phase=%d, decision=%s", rep.Phase, rep.Decision)
		}
	}

	// 2. Webmaster verification flow
	var expectedToken string
	{
		body := `{"account_id":"acc-wm-test","domain":"example.org","method":"dns_txt"}`
		req := httptest.NewRequest("POST", "/api/v1/webmaster/verify", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("webmaster verification token generation failed: %d %s", w.Code, w.Body.String())
		}
		var tokOut struct {
			ExpectedToken string `json:"expected_token"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &tokOut)
		expectedToken = tokOut.ExpectedToken
		if expectedToken == "" {
			t.Fatalf("empty token generated")
		}
	}
	{
		body := `{"account_id":"acc-wm-test","domain":"example.org","token":"` + expectedToken + `"}`
		req := httptest.NewRequest("POST", "/api/v1/webmaster/verify", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("webmaster verification confirm failed: %d %s", w.Code, w.Body.String())
		}
	}

	// 3. Webmaster sitemap submission
	{
		body := `{"account_id":"acc-wm-test","sitemap_url":"https://example.org/sitemap.xml"}`
		req := httptest.NewRequest("POST", "/api/v1/webmaster/sitemap", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("sitemap submission failed: %d %s", w.Code, w.Body.String())
		}
	}

	// 4. GSP Auction execution
	{
		body := `{
			"top_slots": 1,
			"bids": [
				{
					"ad_id": "ad-test-1",
					"max_cpc_krw": 400.0,
					"quality_score": 8.0,
					"headline": "테스트 검색 광고",
					"description": "설명",
					"target_url": "https://ad.example.org"
				}
			]
		}`
		req := httptest.NewRequest("POST", "/api/v1/ads/auction", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("ad auction failed: %d %s", w.Code, w.Body.String())
		}
		var aOut struct {
			Results []struct {
				AdID           string `json:"ad_id"`
				IsSponsored    bool   `json:"is_sponsored"`
				SponsoredLabel string `json:"sponsored_label"`
			} `json:"results"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &aOut)
		if len(aOut.Results) != 1 || !aOut.Results[0].IsSponsored || aOut.Results[0].SponsoredLabel != "광고 / Sponsored" {
			t.Fatalf("invalid auction response: %+v", aOut)
		}
	}
}

func TestPhase8OpsEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
	h := a.routes()

	// 1. Ops gate status
	{
		req := httptest.NewRequest("GET", "/api/v1/ops/gate-status", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("ops gate-status failed: %d %s", w.Code, w.Body.String())
		}
		var gateOut struct {
			Phase    int    `json:"phase"`
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &gateOut); err != nil {
			t.Fatal(err)
		}
		if gateOut.Phase != 8 || gateOut.Decision != "NO-GO" {
			t.Fatalf("expected phase 8 NO-GO, got %+v", gateOut)
		}
	}

	// 2. PAM elevation
	{
		body := `{"account_id":"acc-pam-alice","role":"secops","reason":"incident-mitigation","mfa_verified":true}`
		req := httptest.NewRequest("POST", "/api/v1/ops/pam/elevate", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("pam elevate failed: %d %s", w.Code, w.Body.String())
		}
		var pOut struct {
			ID      string `json:"id"`
			Role    string `json:"role"`
			Revoked bool   `json:"revoked"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &pOut)
		if pOut.ID == "" || pOut.Role != "secops" || pOut.Revoked {
			t.Fatalf("expected valid secops PAM elevation, got %+v", pOut)
		}
	}

	// 3. Privacy DSAR submission
	{
		body := `{"account_id":"acc-dsar-bob","type":"export"}`
		req := httptest.NewRequest("POST", "/api/v1/privacy/dsar", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("dsar submit failed: %d %s", w.Code, w.Body.String())
		}
		var dOut struct {
			Status string `json:"status"`
			Type   string `json:"type"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &dOut)
		if dOut.Status != "pending" || dOut.Type != "export" {
			t.Fatalf("expected pending export DSAR, got %+v", dOut)
		}
	}

	// 4. Dual Approval workflow
	var reqID string
	{
		body := `{"requester_id":"op-requester-1","action_type":"index_flush","payload":"truncate crawler_corpus"}`
		req := httptest.NewRequest("POST", "/api/v1/ops/dual-approval", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("dual approval create failed: %d %s", w.Code, w.Body.String())
		}
		var cOut struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &cOut)
		if cOut.ID == "" || cOut.Status != "pending" {
			t.Fatalf("expected pending dual approval request, got %+v", cOut)
		}
		reqID = cOut.ID
	}
	{
		// Approve by a different operator
		body := `{"request_id":"` + reqID + `","approver_id":"op-approver-2"}`
		req := httptest.NewRequest("POST", "/api/v1/ops/dual-approval", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("dual approval approve failed: %d %s", w.Code, w.Body.String())
		}
		var aOut struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &aOut)
		if aOut.Status != "approved" {
			t.Fatalf("expected approved dual approval, got %+v", aOut)
		}
	}
}

func TestPhase9LaunchEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStoreWithVerifier(fixtureVerifier{}), corsOrigin: "http://127.0.0.1:5173"}
	h := a.routes()

	// 1. Launch Gate Status (Master 10-phase report)
	{
		req := httptest.NewRequest("GET", "/api/v1/launch/gate-status", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("launch gate-status failed: %d %s", w.Code, w.Body.String())
		}
		var rep struct {
			OverallDecision string `json:"overall_decision"`
			TotalPhases     int    `json:"total_phases"`
			CanaryReady     bool   `json:"canary_ready"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &rep)
		if rep.TotalPhases != 10 {
			t.Fatalf("expected 10 phases evaluated, got %d", rep.TotalPhases)
		}
		if rep.OverallDecision == "" {
			t.Fatal("empty overall decision")
		}
	}

	// 2. Benchmarks endpoint
	{
		req := httptest.NewRequest("GET", "/api/v1/launch/benchmarks", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("launch benchmarks failed: %d %s", w.Code, w.Body.String())
		}
		var bOut struct {
			TotalRequests int `json:"total_requests"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &bOut)
		if bOut.TotalRequests < 0 {
			t.Fatalf("invalid total requests: %d", bOut.TotalRequests)
		}
	}

	// 3. DR Drill rehearsal recording
	{
		body := `{
			"drill_name": "postgres_warm_standby_drill",
			"target_rto": 1800,
			"actual_rto": 300,
			"target_rpo": 300,
			"actual_rpo": 15,
			"snapshot_data": "raw-backup-stream-bytes-checksum-valid",
			"verified_by": "secops-lead"
		}`
		req := httptest.NewRequest("POST", "/api/v1/launch/dr-drill", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("launch dr-drill failed: %d %s", w.Code, w.Body.String())
		}
		var dOut struct {
			Status           string `json:"status"`
			ChecksumVerified bool   `json:"checksum_verified"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &dOut)
		if dOut.Status != "PASSED" || !dOut.ChecksumVerified {
			t.Fatalf("expected PASSED drill, got %+v", dOut)
		}
	}

	// 4. Defect register endpoints (POST & GET)
	{
		body := `{"severity":"P3","title":"Footer padding on ultra-wide 4k display","domain":"web","mitigation":"CSS max-width container"}`
		req := httptest.NewRequest("POST", "/api/v1/launch/defects", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("launch defects POST failed: %d %s", w.Code, w.Body.String())
		}

		getReq := httptest.NewRequest("GET", "/api/v1/launch/defects", nil)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, getReq)
		if w.Code != 200 {
			t.Fatalf("launch defects GET failed: %d %s", w.Code, w.Body.String())
		}
		var defOut struct {
			Pass    bool `json:"pass"`
			P0Count int  `json:"p0_count"`
			P1Count int  `json:"p1_count"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &defOut)
		if !defOut.Pass || defOut.P0Count != 0 || defOut.P1Count != 0 {
			t.Fatalf("expected passing release readiness with 0 P0/P1, got %+v", defOut)
		}
	}
}

func TestVerticalFederatedSearchAndAdsClickEndpoints(t *testing.T) {
	a := &app{}
	prodHandler := a.productionRoutes()

	// 1. Image vertical search via productionRoutes
	{
		req := httptest.NewRequest("GET", "/api/v1/vertical/search?vertical=image&q=검색엔진", nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for vertical image search, got %d %s", w.Code, w.Body.String())
		}
		var resp vertical.VerticalResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode vertical image response: %v", err)
		}
		if resp.TotalHits == 0 || resp.Attribution == "" {
			t.Fatalf("expected image hits and legal attribution, got %+v", resp)
		}
	}

	// 2. Video vertical search
	{
		req := httptest.NewRequest("GET", "/api/v1/vertical/search?vertical=video&q=트래픽", nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for vertical video search, got %d", w.Code)
		}
	}

	// 3. News vertical search
	{
		req := httptest.NewRequest("GET", "/api/v1/vertical/search?vertical=news&q=Phase", nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for vertical news search, got %d", w.Code)
		}
	}

	// 4. Map vertical search
	{
		req := httptest.NewRequest("GET", "/api/v1/vertical/search?vertical=map&q=시청", nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for vertical map search, got %d", w.Code)
		}
	}

	// 5. Shopping vertical search
	{
		req := httptest.NewRequest("GET", "/api/v1/vertical/search?vertical=shopping&q=노트북", nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for vertical shopping search, got %d", w.Code)
		}
	}

	// 6. Ads click tracking and ledger charge via productionRoutes
	{
		body := `{"ad_id":"ad-cloud-001","cpc_krw":550}`
		req := httptest.NewRequest("POST", "/api/v1/ads/click", bytes.NewBufferString(body))
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/130.0")
		req.Header.Set("X-Forwarded-For", "203.0.113.195")
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for ads click, got %d %s", w.Code, w.Body.String())
		}
		var clickOut struct {
			IsValid        bool    `json:"is_valid"`
			BillableAmount float64 `json:"billable_amount"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &clickOut); err != nil || !clickOut.IsValid {
			t.Fatalf("expected valid click validation, got %+v", clickOut)
		}
	}
}
