package main

import (
	"log"
	"net/http"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ai"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
)

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
		// C-8: no vertical index backend is connected (image/news/video/
		// shopping ingestion lands with the Phase 5 licensing gate), so the
		// only honest status is 501, not an empty result set.
		writeJSON(w, 501, map[string]string{
			"error": "vertical '" + vertical + "' is not implemented: no index backend connected (Phase 5 licensing gate)",
			"code":  "VERTICAL_NOT_IMPLEMENTED",
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

	// C-5: SafeSearch filters inside OpenSearch (index-time labels), so total and
	// pagination already reflect the level. The old post-filter block that
	// rewrote the page while leaving total untouched was removed.
	// C-6: hybrid RRF over keyword + recall paths (flag-off: keyword only).
	searchCtx := r.Context()
	var result search.Response
	if hybridSearchEnabled() {
		result, err = a.c.SearchHybrid(searchCtx, normalizedQ, lang, page, 10, safeSearch)
	} else {
		result, err = a.c.SearchWithSafety(searchCtx, normalizedQ, lang, page, 10, safeSearch)
	}
	if err != nil {
		log.Print("search backend failed")
		writeJSON(w, 503, map[string]string{"error": "search temporarily unavailable"})
		return
	}
	traceID := w.Header().Get("X-Request-Id")
	result.RequestID = traceID
	// C-4: index-grounded correction (layout rescue + phrase suggester).
	// Best-effort enrichment: a correction failure must not fail the search.
	if corr, err := a.c.SuggestCorrection(r.Context(), normalizedQ); err != nil {
		log.Print("suggest correction failed")
	} else {
		result.CorrectedQuery = corr
	}
	intent := a.ensureIntentClassifier().Classify(normalizedQ, lang, "")
	result.Intent = string(intent.Intent)
	if safeSearch != "" {
		result.SafeSearch = safeSearch
	}
	if vertical != "" {
		result.Vertical = vertical
	}

	// 2. SafeSearch is enforced inside OpenSearch via SearchWithSafety (C-5), so
	// total and pagination already reflect the level. The old page post-filter
	// (which rewrote hits while leaving total untouched) was removed.

	// 2.5 Hybrid RRF fusion now happens inside SearchHybrid (C-6) over two
	// independent retrieval paths. The old same-list reorder was removed.

	// 3. Second-Pass Quality, Freshness & Spam Reranking (Task 043)
	if len(result.Hits) > 0 {
		now := time.Now()
		candidates := make([]ranking.DocumentSignals, 0, len(result.Hits))
		for _, h := range result.Hits {
			candidates = append(candidates, ranking.DocumentSignals{
				DocumentID:           h.DocumentID,
				URL:                  h.URL,
				Title:                h.Title,
				Description:          h.Description,
				BaseScore:            h.Score,
				PublishedAt:          resolveHitPublishedAt(h, now),
				DomainAuthority:      resolveHitDomainAuthority(h),
				TermFrequencyDensity: resolveHitTermDensity(h),
			})
		}
		reranked := a.ensureReranker().Rerank(candidates, now)
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

	// 4. Grounded AI Overview: DISABLED by default (A-2). No LLM/RAG pipeline is
	// configured in this build, so the operational search path must not fabricate
	// an AI summary block from extractive snippets. Enable only with
	// AI_OVERVIEW_ENABLED=true once a real, evaluated pipeline is wired.
	if aiOverviewEnabled() && len(result.Hits) > 0 && (r.URL.Query().Get("ai") == "1" || result.Intent == "informational") {
		passages := make([]ai.DocumentChunk, 0, len(result.Hits))
		for i, h := range result.Hits {
			if i >= 4 {
				break
			}
			passages = append(passages, ai.DocumentChunk{
				DocumentID:     h.DocumentID,
				SourceURL:      h.URL,
				HeadingContext: h.Title,
				Content:        h.Description,
			})
		}
		if synth, err := ai.SynthesizeAnswer(r.Context(), ai.RAGRequest{Query: normalizedQ, Passages: passages}, ai.DefaultBudgetLimits()); err == nil && !synth.FallbackTriggered {
			citations := make([]search.AICitation, 0, len(synth.Citations))
			for _, c := range synth.Citations {
				citations = append(citations, search.AICitation{
					Index:    c.Index,
					SourceID: c.DocumentID,
					URL:      c.URL,
					Title:    c.Title,
					Snippet:  c.Snippet,
				})
			}
			result.AIOverview = &search.AIOverviewSummary{
				Summary:     synth.Answer,
				Citations:   citations,
				Confidence:  synth.Grounding.OverallFaithScore,
				IsFaithful:  synth.Grounding.IsFaithful,
				Disclaimer:  synth.Disclaimer,
				ExecutionMs: synth.ExecutionMs,
			}
		}
	}

	// 5. Sponsored Ads: NO ad engine or ads DB is connected (A-1). The fabricated
	// "ad-cloud-001" injection was removed; the slot stays an explicit empty
	// array until a real ads backend is wired behind a feature flag.
	result.SponsoredAds = []search.SponsoredAdItem{}

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
	if corr, err := a.c.SuggestCorrection(r.Context(), normalizedQ); err != nil {
		log.Print("suggest correction failed")
	} else if corr != "" {
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
