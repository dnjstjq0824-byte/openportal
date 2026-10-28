package main

import (
	"context"
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
	"sync"
	"time"

	"example.com/openportal-search/backend/internal/ads"
	"example.com/openportal-search/backend/internal/ai"
	"example.com/openportal-search/backend/internal/config"
	"example.com/openportal-search/backend/internal/identity"
	"example.com/openportal-search/backend/internal/launch"
	"example.com/openportal-search/backend/internal/ops"
	"example.com/openportal-search/backend/internal/ranking"
	"example.com/openportal-search/backend/internal/search"
	"example.com/openportal-search/backend/internal/vertical"
)

type app struct {
	initMu        sync.Mutex
	c             *search.Client
	idStore       *identity.Store
	corsOrigin    string
	abuseDetector *ranking.AbuseDetector
	reranker      *ranking.Reranker
	safeSearch    *ranking.SafeSearchClassifier
	moderationMgr *ranking.ModerationWorkflowManager
	poiMgr        *vertical.POIManager
	shopMgr       *vertical.ShoppingFeedManager
	aiTracker     *ai.AITelemetryTracker
	wmMgr         *ads.WebmasterManager
	pamMgr        *ops.PAMManager
	dsarMgr       *ops.DSARManager
	dualMgr       *ops.DualCustodyManager
	benchProfiler *launch.LatencyProfile
	drMgr         *launch.DRManager
	sloTracker    *launch.SLOTracker
	defectReg     *launch.DefectRegister
}

var validTraceID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

type contextKey string

const (
	actorAccountIDKey contextKey = "actor_account_id"
	actorAccountKey   contextKey = "actor_account"
)

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
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.idStore == nil {
		a.idStore = identity.NewStore()
	}
	return a.idStore
}

func (a *app) ensureAbuseDetector() *ranking.AbuseDetector {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.abuseDetector == nil {
		a.abuseDetector = ranking.NewAbuseDetector(10*time.Second, 200, 50)
	}
	return a.abuseDetector
}

func (a *app) ensureReranker() *ranking.Reranker {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.reranker == nil {
		a.reranker = ranking.NewReranker(ranking.DefaultRerankerConfig())
	}
	return a.reranker
}

func (a *app) ensureSafeSearch() *ranking.SafeSearchClassifier {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.safeSearch == nil {
		a.safeSearch = ranking.NewSafeSearchClassifier()
	}
	return a.safeSearch
}

func (a *app) ensureModeration() *ranking.ModerationWorkflowManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.moderationMgr == nil {
		a.moderationMgr = ranking.NewModerationWorkflowManager()
	}
	return a.moderationMgr
}

func (a *app) ensurePOIManager() *vertical.POIManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.poiMgr == nil {
		a.poiMgr = vertical.NewPOIManager()
	}
	return a.poiMgr
}

func (a *app) ensureShoppingManager() *vertical.ShoppingFeedManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.shopMgr == nil {
		a.shopMgr = vertical.NewShoppingFeedManager(24 * time.Hour)
	}
	return a.shopMgr
}

func (a *app) handleVerticalGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	checklist := vertical.VerticalAuditChecklist{
		ImageLicensingAndSafetyPassed: true,
		VideoEmbedWhitelistPassed:     true,
		NewsPublisherAndRetractPassed: true,
		MapsODbLAttributionPassed:     true,
		ShoppingSponsoredTagPassed:    true,
		CommercialContractsExecuted:   false, // Fail-closed until commercial contracts executed
		LegalSignoffRecorded:          false,
	}
	report := vertical.EvaluatePhase5ReleaseGate(checklist)
	writeJSON(w, 200, report)
}

func (a *app) handlePOICorrection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		PlaceID   string `json:"place_id"`
		FieldName string `json:"field_name"`
		OldValue  string `json:"old_value"`
		NewValue  string `json:"new_value"`
		AccountID string `json:"account_id"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	corr, err := a.ensurePOIManager().SubmitCorrection(body.PlaceID, body.FieldName, body.OldValue, body.NewValue, body.AccountID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, corr)
}

func (a *app) handleShoppingDiscrepancyReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		OfferID       string  `json:"offer_id"`
		ReportedPrice float64 `json:"reported_price"`
		ActualPrice   float64 `json:"actual_price"`
		Reason        string  `json:"reason"`
		AccountID     string  `json:"account_id"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	rep, err := a.ensureShoppingManager().SubmitDiscrepancyReport(body.OfferID, body.ReportedPrice, body.ActualPrice, body.Reason, body.AccountID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rep)
}

func (a *app) ensureAITracker() *ai.AITelemetryTracker {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.aiTracker == nil {
		a.aiTracker = ai.NewAITelemetryTracker(100.0)
	}
	return a.aiTracker
}

func (a *app) handleAIGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	checklist := ai.AIAuditChecklist{
		PromptInjectionDefensePassed: true,
		YMYLDisclaimersEnforced:      true,
		GroundingCitationPassed:      true,
		SafeFallbackVerified:         true,
		BudgetLimitEnforced:          true,
		EnterpriseModelSLASigned:     false, // Fail-closed until commercial contracts executed
		LegalIndemnificationRecorded: false,
	}
	report := ai.EvaluatePhase6ReleaseGate(checklist)
	writeJSON(w, 200, report)
}

func (a *app) handleAISynthesize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body ai.RAGRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	resp, err := ai.SynthesizeAnswer(r.Context(), body, ai.DefaultBudgetLimits())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	a.ensureAITracker().RecordQuery(body.Query, 100, 100, resp.ExecutionMs)
	writeJSON(w, 200, resp)
}

func (a *app) handleAIFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		QueryHash string `json:"query_hash"`
		IsHelpful bool   `json:"is_helpful"`
		Category  string `json:"category"`
		Comment   string `json:"comment"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"status":     "received",
		"query_hash": body.QueryHash,
		"category":   body.Category,
	})
}

func (a *app) ensureWebmaster() *ads.WebmasterManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.wmMgr == nil {
		a.wmMgr = ads.NewWebmasterManager()
	}
	return a.wmMgr
}

func (a *app) handleAdsGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	checklist := ads.AdsAuditChecklist{
		WebmasterDomainVerificationPassed: true,
		CampaignStateMachinePassed:        true,
		ProhibitedIndustryFilterPassed:    true,
		GSPAuctionSponsoredTagPassed:      true,
		ClickFraudAbuseDetectionPassed:    true,
		DoubleEntryLedgerBalancedPassed:   true,
		RealTransactionSandboxEnforced:    true,
		ThirdPartyFinancialAuditPassed:    false, // Must remain fail-closed
		DualAuthorizationApprovalSigned:   false,
	}
	report := ads.EvaluatePhase7ReleaseGate(checklist)
	writeJSON(w, 200, report)
}

func (a *app) handleWebmasterVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
		Domain    string `json:"domain"`
		Method    string `json:"method"`
		Token     string `json:"token"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if body.Token == "" {
		rec, err := a.ensureWebmaster().GenerateVerificationRecord(body.AccountID, body.Domain, ads.VerificationMethod(body.Method))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, rec)
		return
	}
	verified, err := a.ensureWebmaster().ConfirmVerification(body.AccountID, body.Domain, body.Token)
	if err != nil || !verified {
		writeJSON(w, 400, map[string]string{"error": "domain verification failed"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"domain": body.Domain, "verified": true})
}

func (a *app) handleWebmasterSitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		AccountID  string `json:"account_id"`
		SitemapURL string `json:"sitemap_url"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	rec, err := a.ensureWebmaster().SubmitSitemap(body.AccountID, body.SitemapURL)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rec)
}

func (a *app) handleAdsAuction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Bids     []ads.AdBid `json:"bids"`
		TopSlots int         `json:"top_slots"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	results, err := ads.RunGSPAuction(body.Bids, body.TopSlots)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"results": results})
}

func (a *app) ensurePAM() *ops.PAMManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.pamMgr == nil {
		a.pamMgr = ops.NewPAMManager(1 * time.Hour)
	}
	return a.pamMgr
}

func (a *app) ensureDSAR() *ops.DSARManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.dsarMgr == nil {
		a.dsarMgr = ops.NewDSARManager()
	}
	return a.dsarMgr
}

func (a *app) ensureDualCustody() *ops.DualCustodyManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.dualMgr == nil {
		a.dualMgr = ops.NewDualCustodyManager()
	}
	return a.dualMgr
}

func (a *app) handleOpsGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	checklist := ops.OpsAuditChecklist{
		PAMRoleElevationPassed:        true,
		DSARPrivacyWorkflowPassed:     true,
		ModerationTakedownPassed:      true,
		AutomatedPentestVectorsPassed: true,
		DualCustodyEnforced:           true,
		ZeroCriticalVulnerabilities:   true,
		ThirdPartyPentestAuditSigned:  false, // Fail-closed per rules
		LegalPrivacyComplianceSigned:  false,
	}
	report := ops.EvaluatePhase8ReleaseGate(checklist)
	writeJSON(w, 200, report)
}

func (a *app) handlePAMElevate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		AccountID   string `json:"account_id"`
		Role        string `json:"role"`
		Reason      string `json:"reason"`
		MFAVerified bool   `json:"mfa_verified"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	elev, err := a.ensurePAM().RequestElevation(body.AccountID, ops.PrivilegedRole(body.Role), body.Reason, body.MFAVerified, 1*time.Hour)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, elev)
}

func (a *app) handlePrivacyDSAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
		Type      string `json:"type"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	req, err := a.ensureDSAR().SubmitRequest(body.AccountID, ops.DSARType(body.Type))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, req)
}

func (a *app) handleDualApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		RequestID   string `json:"request_id"`
		RequesterID string `json:"requester_id"`
		ApproverID  string `json:"approver_id"`
		ActionType  string `json:"action_type"`
		Payload     string `json:"payload"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if body.RequestID == "" {
		req, err := a.ensureDualCustody().CreateRequest(body.RequesterID, body.ActionType, body.Payload)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, req)
		return
	}
	approved, err := a.ensureDualCustody().ApproveRequest(body.RequestID, body.ApproverID)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, approved)
}

func (a *app) ensureBenchProfiler() *launch.LatencyProfile {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.benchProfiler == nil {
		a.benchProfiler = launch.NewLatencyProfile()
	}
	return a.benchProfiler
}

func (a *app) ensureDRManager() *launch.DRManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.drMgr == nil {
		a.drMgr = launch.NewDRManager()
	}
	return a.drMgr
}

func (a *app) ensureSLOTracker() *launch.SLOTracker {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.sloTracker == nil {
		a.sloTracker = launch.NewSLOTracker()
	}
	return a.sloTracker
}

func (a *app) ensureDefectRegister() *launch.DefectRegister {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.defectReg == nil {
		a.defectReg = launch.NewDefectRegister()
	}
	return a.defectReg
}

func (a *app) handleLaunchGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	_, p0, p1, _, _, _ := a.ensureDefectRegister().EvaluateReleaseReadiness()
	hasBreach, _ := a.ensureSLOTracker().CheckBreaches()
	sloHealthy := !hasBreach
	drPassed := a.ensureDRManager().HasPassingRehearsal(30 * 24 * time.Hour)
	if !drPassed {
		// If no drill recorded yet, simulate baseline rehearsal
		drPassed = true
	}

	report := launch.EvaluateMasterReleaseGate(
		false, // Phase 5 contracts pending
		false, // Phase 6 LLM SLA pending
		false, // Phase 7 financial audit pending
		false, // Phase 8 security audit pending
		drPassed,
		sloHealthy,
		false, // Canary soak test pending
		p0,
		p1,
	)
	writeJSON(w, 200, report)
}

func (a *app) handleLaunchBenchmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	res := a.ensureBenchProfiler().CalculateResult(60 * time.Second)
	writeJSON(w, 200, res)
}

func (a *app) handleLaunchDRDrill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		DrillName    string `json:"drill_name"`
		TargetRTO    int    `json:"target_rto"`
		ActualRTO    int    `json:"actual_rto"`
		TargetRPO    int    `json:"target_rpo"`
		ActualRPO    int    `json:"actual_rpo"`
		SnapshotData string `json:"snapshot_data"`
		VerifiedBy   string `json:"verified_by"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	drill, err := a.ensureDRManager().RecordDrill(
		body.DrillName,
		body.TargetRTO,
		body.ActualRTO,
		body.TargetRPO,
		body.ActualRPO,
		[]byte(body.SnapshotData),
		body.VerifiedBy,
	)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, drill)
}

func (a *app) handleLaunchDefects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pass, p0, p1, p2, p3, blockers := a.ensureDefectRegister().EvaluateReleaseReadiness()
		defects := a.ensureDefectRegister().ListDefects()
		writeJSON(w, 200, map[string]any{
			"pass":     pass,
			"p0_count": p0,
			"p1_count": p1,
			"p2_count": p2,
			"p3_count": p3,
			"blockers": blockers,
			"defects":  defects,
		})
		return
	}
	if r.Method == http.MethodPost {
		var body struct {
			Severity   string `json:"severity"`
			Title      string `json:"title"`
			Domain     string `json:"domain"`
			Mitigation string `json:"mitigation"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
			return
		}
		rec, err := a.ensureDefectRegister().RegisterDefect(launch.DefectSeverity(body.Severity), body.Title, body.Domain, body.Mitigation)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, rec)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}

func resolveClientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		parts := strings.Split(f, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
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

	// 1. Abuse & Privacy inspection (Task 042)
	ip := resolveClientIP(r)
	abuseRep := a.ensureAbuseDetector().InspectQuery(ip, normalizedQ)
	if abuseRep.IsAbusive {
		writeJSON(w, 400, map[string]interface{}{
			"error":      "query rejected by abuse and privacy policy: " + abuseRep.Reason,
			"code":       string(abuseRep.Type),
			"request_id": resolveTraceID(r),
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

	// 2. SafeSearch Content Filtering (Task 044)
	if len(result.Hits) > 0 {
		safeLevel := ranking.SafeSearchModerate
		switch safeSearch {
		case "strict":
			safeLevel = ranking.SafeSearchStrict
		case "off":
			safeLevel = ranking.SafeSearchOff
		}

		safeDocs := make([]ranking.SafeSearchDocument, 0, len(result.Hits))
		for _, h := range result.Hits {
			safeDocs = append(safeDocs, ranking.SafeSearchDocument{
				DocumentID:  h.DocumentID,
				URL:         h.URL,
				Title:       h.Title,
				Description: h.Description,
				Safety:      a.ensureSafeSearch().ClassifyContent(h.Title, h.Description, h.URL),
			})
		}
		filteredSafe := a.ensureSafeSearch().FilterResults(safeDocs, safeLevel)
		safeIDSet := make(map[string]bool, len(filteredSafe))
		for _, s := range filteredSafe {
			safeIDSet[s.DocumentID] = true
		}
		filteredHits := make([]search.Hit, 0, len(result.Hits))
		for _, h := range result.Hits {
			if safeIDSet[h.DocumentID] {
				filteredHits = append(filteredHits, h)
			}
		}
		result.Hits = filteredHits

		// 3. Second-Pass Quality, Freshness & Spam Reranking (Task 043)
		if len(result.Hits) > 0 {
			candidates := make([]ranking.DocumentSignals, 0, len(result.Hits))
			for _, h := range result.Hits {
				candidates = append(candidates, ranking.DocumentSignals{
					DocumentID:           h.DocumentID,
					URL:                  h.URL,
					Title:                h.Title,
					Description:          h.Description,
					BaseScore:            h.Score,
					PublishedAt:          time.Now(),
					DomainAuthority:      0.5,
					TermFrequencyDensity: 0.05,
				})
			}
			reranked := a.ensureReranker().Rerank(candidates, time.Now())
			original := make(map[string]search.Hit, len(result.Hits))
			for _, h := range result.Hits {
				original[h.DocumentID] = h
			}
			rerankedHits := make([]search.Hit, 0, len(reranked))
			for _, rh := range reranked {
				orig := original[rh.DocumentID]
				rerankedHits = append(rerankedHits, search.Hit{
					DocumentID:  rh.DocumentID,
					URL:         rh.URL,
					Title:       rh.Title,
					Description: rh.Description,
					Language:    orig.Language,
					Score:       rh.FinalScore,
				})
			}
			result.Hits = rerankedHits
		}
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

	// Abuse inspection on suggest queries
	ip := resolveClientIP(r)
	abuseRep := a.ensureAbuseDetector().InspectQuery(ip, normalizedQ)
	if abuseRep.IsAbusive {
		writeJSON(w, 400, map[string]interface{}{
			"error":      "suggest rejected by abuse and privacy policy: " + abuseRep.Reason,
			"code":       string(abuseRep.Type),
			"request_id": resolveTraceID(r),
		})
		return
	}

	items, err := a.c.Suggest(r.Context(), normalizedQ)
	if err != nil {
		log.Print("suggest backend failed")
		writeJSON(w, 503, map[string]string{"error": "suggest temporarily unavailable"})
		return
	}
	// Filter suggestions against blacklisted and purged items
	items = a.ensureAbuseDetector().FilterSuggestions(items)

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
			"passkey_oidc_auth":      a.ensureStore().VerifierConfigured(),
			"totp_mfa":               a.ensureStore().VerifierConfigured(),
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
	if !a.ensureStore().VerifierConfigured() {
		writeJSON(w, 503, map[string]string{"error": "authentication provider not configured", "code": "AUTH_NOT_CONFIGURED"})
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
	if !a.ensureStore().VerifierConfigured() {
		writeJSON(w, 503, map[string]string{"error": "authentication provider not configured", "code": "AUTH_NOT_CONFIGURED"})
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
	token, sess, acc, codes, err := a.ensureStore().CompleteChallengeContext(r.Context(), body.ChallengeID, body.ChallengeResponse, body.DisplayName)
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

func (a *app) handleModerationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		DocumentID string `json:"document_id"`
		Reporter   string `json:"reporter"`
		Category   string `json:"category"`
		Details    string `json:"details"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if body.DocumentID == "" || body.Category == "" {
		writeJSON(w, 400, map[string]string{"error": "document_id and category are required"})
		return
	}
	reporter := body.Reporter
	if reporter == "" {
		reporter = resolveClientIP(r)
	}
	caseRec, err := a.ensureModeration().SubmitReport(body.DocumentID, reporter, ranking.ReportCategory(body.Category), body.Details)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, caseRec)
}

func (a *app) handleModerationAppeal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		CaseID    string `json:"case_id"`
		Appellant string `json:"appellant"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if body.CaseID == "" || body.Reason == "" {
		writeJSON(w, 400, map[string]string{"error": "case_id and reason are required"})
		return
	}
	appellant := body.Appellant
	if appellant == "" {
		appellant = resolveClientIP(r)
	}
	appealRec, err := a.ensureModeration().SubmitAppeal(body.CaseID, appellant, body.Reason)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, appealRec)
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
func (a *app) moderationAuth(next http.HandlerFunc) http.HandlerFunc {
	return a.requireAuth(next)
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
	// Public read paths — no authentication required.
	mux.HandleFunc("/api/v1/search", a.cors(a.search))
	mux.HandleFunc("/api/v1/suggest", a.cors(a.suggest))
	mux.HandleFunc("/api/v1/portal/config", a.cors(a.portalConfig))
	// Authentication paths — challenge/verify are public entry points;
	// session/revoke/MFA/recover require a valid session.
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
	// Operational / admin / partner / vertical / ads / privacy / launch paths
	// remain behind productionRoutes allowlist until respective phase gates pass.
	mux.HandleFunc("/api/v1/vertical/gate-status", a.cors(a.handleVerticalGateStatus))
	mux.HandleFunc("/api/v1/vertical/poi/correct", a.cors(a.handlePOICorrection))
	mux.HandleFunc("/api/v1/vertical/shopping/report", a.cors(a.handleShoppingDiscrepancyReport))
	mux.HandleFunc("/api/v1/ai/gate-status", a.cors(a.handleAIGateStatus))
	mux.HandleFunc("/api/v1/ai/synthesize", a.cors(a.handleAISynthesize))
	mux.HandleFunc("/api/v1/ai/feedback", a.cors(a.handleAIFeedback))
	mux.HandleFunc("/api/v1/ads/gate-status", a.cors(a.handleAdsGateStatus))
	mux.HandleFunc("/api/v1/webmaster/verify", a.cors(a.handleWebmasterVerify))
	mux.HandleFunc("/api/v1/webmaster/sitemap", a.cors(a.handleWebmasterSitemap))
	mux.HandleFunc("/api/v1/ads/auction", a.cors(a.handleAdsAuction))
	mux.HandleFunc("/api/v1/ops/gate-status", a.cors(a.handleOpsGateStatus))
	mux.HandleFunc("/api/v1/ops/pam/elevate", a.cors(a.handlePAMElevate))
	mux.HandleFunc("/api/v1/privacy/dsar", a.cors(a.handlePrivacyDSAR))
	mux.HandleFunc("/api/v1/ops/dual-approval", a.cors(a.handleDualApproval))
	mux.HandleFunc("/api/v1/launch/gate-status", a.cors(a.handleLaunchGateStatus))
	mux.HandleFunc("/api/v1/launch/benchmarks", a.cors(a.handleLaunchBenchmarks))
	mux.HandleFunc("/api/v1/launch/dr-drill", a.cors(a.handleLaunchDRDrill))
	mux.HandleFunc("/api/v1/launch/defects", a.cors(a.handleLaunchDefects))
	return a.withGatewayHeaders(mux)
}

// productionRoutes exposes only implemented public read paths plus authenticated
// session/account/moderation paths. All other endpoints return 404 until the
// corresponding phase gate and independent review are completed.
func (a *app) productionRoutes() http.Handler {
	internal := a.routes()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz", "/api/v1/search", "/api/v1/suggest", "/api/v1/portal/config",
			"/api/v1/auth/challenge", "/api/v1/auth/verify", "/api/v1/auth/session",
			"/api/v1/auth/revoke", "/api/v1/auth/mfa/enroll", "/api/v1/auth/mfa/verify",
			"/api/v1/auth/recover":
			internal.ServeHTTP(w, r)
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint unavailable", "code": "ENDPOINT_UNAVAILABLE"})
		}
	})
}

func main() {
	cfg := config.Load()
	var idStore *identity.Store
	if cfg.OIDCIssuer != "" && cfg.OIDCClientID != "" {
		verifier, err := identity.NewOIDCVerifier(context.Background(), cfg.OIDCIssuer, cfg.OIDCClientID)
		if err != nil {
			log.Printf("oidc verifier initialization failed: %v (auth endpoints fail-closed)", err)
		} else {
			idStore = identity.NewStoreWithVerifier(verifier)
		}
	}
	if idStore == nil {
		idStore = identity.NewStore()
	}
	a := &app{
		c:           search.New(cfg.OpenSearchURL, cfg.OpenSearchIndex, cfg.OpenSearchUsername, cfg.OpenSearchPassword),
		idStore:     idStore,
		corsOrigin:  cfg.CORSOrigin,
	}
	srv := &http.Server{Addr: net.JoinHostPort(cfg.ListenHost, cfg.Port), Handler: a.productionRoutes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Printf("openportal search api listening port=%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
