package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/vertical"
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
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Atomic: hybrid search fans out parallel backend queries.
		calls.Add(1)
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
		{"GET", "/api/v1/search?q=x&vertical=image", 501},
		{"GET", "/api/v1/search?q=x&safe_search=invalid", 400},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
	}
	if calls.Load() != 0 {
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
		body, _ := io.ReadAll(r.Body)
		// C-4: model count queries honestly — only the converted term matches.
		if strings.Contains(string(body), `"suggest"`) {
			_, _ = w.Write([]byte(`{"suggest":{"correction-title":[{"text":"x","options":[]}],"correction-body":[{"text":"x","options":[]}]}}`))
			return
		}
		if strings.Contains(string(body), `"size":0`) {
			total := 0
			if strings.Contains(string(body), "검색엔진") {
				total = 1
			}
			_, _ = w.Write([]byte(`{"hits":{"total":{"value":` + strconv.Itoa(total) + `},"hits":[]}}`))
			return
		}
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
	// C-5: filtering happens inside OpenSearch (must_not in the query DSL),
	// not as a handler post-filter. The stub backend models that contract:
	// it applies the strict safety filter itself when the query carries it.
	var gotSearchQuery []byte
	var captureMu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// First request is the search; later ones are best-effort correction
		// or the parallel hybrid path. Guarded: hybrid queries race here.
		captureMu.Lock()
		if gotSearchQuery == nil {
			gotSearchQuery = body
		}
		captureMu.Unlock()
		if strings.Contains(string(body), `"safety":["adult","gambling","malware"]`) {
			_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[
				{"_id":"doc-clean","_score":2.5,"_source":{"document_id":"doc-clean","title":"정상 기술 문서","url":"https://example.org/tech","description":"클린 내용","language":"ko"}}
			]}}`))
			return
		}
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
		if !strings.Contains(string(gotSearchQuery), `"safety":["adult","gambling","malware"]`) {
			t.Fatalf("strict safesearch must travel in the backend query, got: %s", gotSearchQuery)
		}
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

// C-8: non-web verticals have no index backend; the production search path
// must answer 501 (not an empty list, not 403 theater).
func TestSearchVerticalNotImplemented(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore(), corsOrigin: "http://127.0.0.1:5173"}
	for _, v := range []string{"image", "news", "video", "shopping", "map"} {
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=x&vertical="+v, nil))
		if w.Code != 501 {
			t.Fatalf("vertical=%s: expected 501, got %d", v, w.Code)
		}
		var errResp struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)
		if errResp.Code != "VERTICAL_NOT_IMPLEMENTED" {
			t.Fatalf("vertical=%s: expected code VERTICAL_NOT_IMPLEMENTED, got %+v", v, errResp)
		}
	}
	// web vertical still served.
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=x&vertical=web", nil))
	if w.Code != 200 {
		t.Fatalf("vertical=web: expected 200, got %d", w.Code)
	}
}

func TestPrototypeVerticalFederatedSearchAndAdsClickEndpoints(t *testing.T) {
	a := &app{}
	prodHandler := a.routes() // Prototype-only; production denial is tested separately.

	// 1. Prototype image vertical handler (not publicly exposed).
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
		// C-8: prototype seed hits must be labeled as such.
		if resp.ServedFrom != "prototype-seed" {
			t.Fatalf("expected served_from=prototype-seed, got %+v", resp)
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

func TestPrototypeLocalAuthAndEmailVerificationHTTPEndpoints(t *testing.T) {
	a := &app{}
	prodHandler := a.routes() // Memory fixture only; not the browser authentication system.

	emailAddr := "newuser@open-scanner.org"
	password := "SecureP@ssw0rd2026!"
	displayName := "오픈스캐너사용자"

	// 1. Register local account -> 201
	var verifyCode string
	{
		body := fmt.Sprintf(`{"email":%q,"password":%q,"display_name":%q}`, emailAddr, password, displayName)
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 201 {
			t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
		}
		var regOut struct {
			Status string `json:"status"`
			Code   string `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &regOut); err != nil || regOut.Status != "pending_verification" {
			t.Fatalf("unexpected register output: %+v", regOut)
		}
		verifyCode = regOut.Code
		if len(verifyCode) != 6 {
			t.Fatalf("expected 6-digit PIN code, got %s", verifyCode)
		}
	}

	// 2. Login before email verification fails -> 401
	{
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, emailAddr, password)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("expected 401 for unverified login, got %d", w.Code)
		}
	}

	// 3. Verify email with wrong code fails -> 400
	{
		body := fmt.Sprintf(`{"email":%q,"code":"000000"}`, emailAddr)
		req := httptest.NewRequest("POST", "/api/v1/auth/email/verify", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("expected 400 for wrong verification code, got %d", w.Code)
		}
	}

	// 4. Verify email with correct code -> 200
	{
		body := fmt.Sprintf(`{"email":%q,"code":%q}`, emailAddr, verifyCode)
		req := httptest.NewRequest("POST", "/api/v1/auth/email/verify", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("email verify failed: %d %s", w.Code, w.Body.String())
		}
	}

	// 5. Login after verification succeeds -> 200
	var token string
	{
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, emailAddr, password)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
		}
		var loginOut struct {
			Status string `json:"status"`
			Token  string `json:"token"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &loginOut); err != nil || loginOut.Status != "authenticated" {
			t.Fatalf("unexpected login output: %+v", loginOut)
		}
		token = loginOut.Token
		if len(token) == 0 {
			t.Fatalf("empty token returned")
		}
	}

	// 6. Request password reset -> 200
	var resetToken string
	{
		body := fmt.Sprintf(`{"email":%q}`, emailAddr)
		req := httptest.NewRequest("POST", "/api/v1/auth/password/reset-request", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("password reset request failed: %d %s", w.Code, w.Body.String())
		}
		var resetOut struct {
			Status string `json:"status"`
			Token  string `json:"token"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resetOut)
		resetToken = resetOut.Token
		if len(resetToken) == 0 {
			t.Fatalf("empty reset token")
		}
	}

	// 7. Confirm password reset -> 200
	newPass := "UpdatedP@ssw0rd2026!"
	{
		body := fmt.Sprintf(`{"email":%q,"token":%q,"new_password":%q}`, emailAddr, resetToken, newPass)
		req := httptest.NewRequest("POST", "/api/v1/auth/password/reset-confirm", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("password reset confirm failed: %d %s", w.Code, w.Body.String())
		}
	}

	// 8. Old password fails -> 401
	{
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, emailAddr, password)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("expected 401 with old password, got %d", w.Code)
		}
	}

	// 9. New password succeeds -> 200
	{
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, emailAddr, newPass)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("login with new password failed: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPrototypeSuperplatformHTTPEndpoints(t *testing.T) {
	a := &app{
		idStore: identity.NewStore(),
	}
	prodHandler := a.routes() // No production access to synthetic superplatform data.

	// A-6: fabricated catalogs were removed. The endpoints stay registered but
	// must honestly report 501 NOT_IMPLEMENTED instead of fake data.
	for _, path := range []string{
		"/api/v1/superplatform/apps",
		"/api/v1/superplatform/mail?email=tester@openscanner.internal",
		"/api/v1/superplatform/developer",
	} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 501 {
			t.Fatalf("GET %s: expected 501, got %d %s", path, w.Code, w.Body.String())
		}
		var resp struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("GET %s: unmarshal error: %v", path, err)
		}
		if resp.Code != "NOT_IMPLEMENTED" {
			t.Fatalf("GET %s: expected code NOT_IMPLEMENTED, got %q", path, resp.Code)
		}
	}

	// Method not allowed on POST -> 405
	{
		req := httptest.NewRequest("POST", "/api/v1/superplatform/apps", nil)
		w := httptest.NewRecorder()
		prodHandler.ServeHTTP(w, req)
		if w.Code != 405 {
			t.Fatalf("expected 405 on POST apps, got %d", w.Code)
		}
	}
}

// A-1/A-2: the operational search path must not inject fabricated ads and must
// not serve an AI Overview block while no model pipeline is configured.
func TestSearchHonestSlotsByDefault(t *testing.T) {
	t.Setenv("AI_OVERVIEW_ENABLED", "")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"timed_out": false,
			"_shards": {"failed": 0},
			"hits": {"total": {"value": 2}, "hits": [
				{"_id": "doc-1", "_score": 2.5, "_source": {"document_id": "doc-1", "title": "클라우드 서버 개발 가이드", "url": "https://example.test/cloud", "description": "클라우드 서버 개발을 시작하는 방법. 첫 단계를 안내합니다.", "language": "ko"}},
				{"_id": "doc-2", "_score": 1.5, "_source": {"document_id": "doc-2", "title": "OpenPortal 검색 소개", "url": "https://example.test/intro", "description": "검색 시스템의 개요와 사용법. 빠르게 찾아봅니다.", "language": "ko"}}
			]}
		}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore(), corsOrigin: "http://127.0.0.1:5173"}

	for _, path := range []string{
		"/api/v1/search?q=" + url.QueryEscape("클라우드 서버 개발"),
		"/api/v1/search?q=" + url.QueryEscape("클라우드 서버 개발") + "&ai=1",
	} {
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s: got %d %s", path, w.Code, w.Body.String())
		}
		var resp struct {
			Hits         []map[string]any `json:"hits"`
			SponsoredAds *[]any           `json:"sponsored_ads"`
			AIOverview   *map[string]any  `json:"ai_overview"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("GET %s: unmarshal error: %v", path, err)
		}
		if len(resp.Hits) != 2 {
			t.Fatalf("GET %s: expected 2 passthrough hits, got %d", path, len(resp.Hits))
		}
		// A-1: the ad slot must be an explicit empty array, never a fake injection.
		if resp.SponsoredAds == nil {
			t.Fatalf("GET %s: sponsored_ads must be present as []", path)
		}
		if len(*resp.SponsoredAds) != 0 {
			t.Fatalf("GET %s: sponsored_ads must be empty without an ad backend", path)
		}
		// A-2: no AI block without a configured pipeline, even with ?ai=1.
		if resp.AIOverview != nil {
			t.Fatalf("GET %s: ai_overview must be absent while AI_OVERVIEW_ENABLED is unset", path)
		}
		raw := w.Body.String()
		if strings.Contains(raw, "ad-cloud-001") || strings.Contains(raw, "cloud.openportal.org") {
			t.Fatalf("GET %s: fabricated ad payload leaked into response", path)
		}
	}
}

// C-5: the safe_search level travels into the OpenSearch query (not a page
// post-filter), so total and pagination already reflect the level.
func TestSearchSafeSearchInQuery(t *testing.T) {
	var gotQuery []byte
	var queryMu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// First request wins; later ones are the parallel hybrid path or
		// best-effort correction. Guarded: hybrid queries race here.
		queryMu.Lock()
		if gotQuery == nil {
			gotQuery = body
		}
		queryMu.Unlock()
		_, _ = w.Write([]byte(`{
			"timed_out": false,
			"_shards": {"failed": 0},
			"hits": {"total": {"value": 42}, "hits": [
				{"_id": "doc-1", "_score": 2.5, "_source": {"document_id": "doc-1", "title": "clean guide", "url": "https://example.test/clean", "description": "A clean technical guide.", "language": "en"}}
			]}
		}`))
	}))
	defer upstream.Close()
	a := &app{c: search.New(upstream.URL, "documents_v1", "", ""), idStore: identity.NewStore(), corsOrigin: "http://127.0.0.1:5173"}

	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=guide&safe_search=strict", nil))
	if w.Code != 200 {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(string(gotQuery), `"safety":["adult","gambling","malware"]`) {
		t.Fatalf("upstream query missing strict safety filter: %s", gotQuery)
	}
	var resp struct {
		Total      int64          `json:"total"`
		Hits       []map[string]any `json:"hits"`
		SafeSearch string         `json:"safe_search"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// Total passes through untouched: filtering happened before paging.
	if resp.Total != 42 || len(resp.Hits) != 1 || resp.SafeSearch != "strict" {
		t.Fatalf("total=%d hits=%d safe=%q", resp.Total, len(resp.Hits), resp.SafeSearch)
	}
}

func TestAIOverviewFlag(t *testing.T) {
	t.Setenv("AI_OVERVIEW_ENABLED", "")
	if aiOverviewEnabled() {
		t.Fatal("expected disabled by default")
	}
	t.Setenv("AI_OVERVIEW_ENABLED", "true")
	if !aiOverviewEnabled() {
		t.Fatal("expected enabled with AI_OVERVIEW_ENABLED=true")
	}
	t.Setenv("AI_OVERVIEW_ENABLED", "1")
	if aiOverviewEnabled() {
		t.Fatal("expected only explicit 'true' to enable")
	}
}
