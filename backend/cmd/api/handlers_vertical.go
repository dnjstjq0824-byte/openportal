package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/vertical"
)

func (a *app) handleVerticalSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	vParam := strings.TrimSpace(r.URL.Query().Get("vertical"))
	if vParam == "" {
		vParam = "image"
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	safeSearch := r.URL.Query().Get("safe_search")

	vq := vertical.VerticalQuery{
		Vertical:   vertical.VerticalType(vParam),
		Query:      query,
		Page:       page,
		PageSize:   pageSize,
		SafeSearch: safeSearch,
	}

	resp, err := a.ensureVerticalService().ExecuteVerticalQuery(r.Context(), vq)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, resp)
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

func (a *app) handleNewsSyndicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Action       string `json:"action"` // "ingest" or "retract"
		ArticleID    string `json:"article_id"`
		PublisherID  string `json:"publisher_id"`
		Title        string `json:"title"`
		Summary      string `json:"summary"`
		CanonicalURL string `json:"canonical_url"`
		Section      string `json:"section"`
		Reason       string `json:"reason"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.EqualFold(body.Action, "retract") || strings.EqualFold(body.Action, "delete") {
		rec, err := a.ensureVerticalService().RetractSyndicatedArticle(body.ArticleID, body.Reason)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{
			"status":      "retracted_tombstoned",
			"sla_window":  "<= 15 minutes (immediate propagation)",
			"retraction":  rec,
		})
		return
	}
	art, err := a.ensureVerticalService().IngestSyndicatedArticle(vertical.NewsArticle{
		ID:           body.ArticleID,
		PublisherID:  body.PublisherID,
		Title:        body.Title,
		Summary:      body.Summary,
		CanonicalURL: body.CanonicalURL,
		Section:      body.Section,
		PublishedAt:  time.Now(),
	})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":  "ingested",
		"article": art,
	})
}

func (a *app) handleShoppingFeedIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		ID           string  `json:"id"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		MerchantName string  `json:"merchant_name"`
		Price        float64 `json:"price"`
		Currency     string  `json:"currency"`
		Availability string  `json:"availability"`
		AffiliateURL string  `json:"affiliate_url"`
		IsSponsored  bool    `json:"is_sponsored"`
		StaleHours   int     `json:"stale_hours"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	updatedAt := time.Now()
	if body.StaleHours > 0 {
		updatedAt = time.Now().Add(-time.Duration(body.StaleHours) * time.Hour)
	}
	avail := vertical.InStock
	if body.Availability != "" {
		avail = vertical.AvailabilityStatus(body.Availability)
	}
	offer, err := a.ensureShoppingManager().IngestOffer(vertical.ProductOffer{
		ID:            body.ID,
		Title:         body.Title,
		Description:   body.Description,
		MerchantName:  body.MerchantName,
		Price:         body.Price,
		Currency:      body.Currency,
		Availability:  avail,
		AffiliateURL:  body.AffiliateURL,
		IsSponsored:   body.IsSponsored,
		FeedUpdatedAt: updatedAt,
	})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, offer)
}
