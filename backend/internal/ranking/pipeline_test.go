package ranking

import (
	"testing"
	"time"
)

func TestQualitySignOffGateApprovedAndRejected(t *testing.T) {
	gate := NewQualitySignOffGate(DefaultQualityGateThresholds())

	// 1. High-quality Golden Evaluation Set satisfying all criteria
	goldenRelevances := [][]int{
		{3, 2, 2, 1, 0}, // Query 1: NDCG = 1.0, first rel rank = 1
		{2, 3, 1, 0, 0}, // Query 2: NDCG = 0.96, first rel rank = 1
		{3, 2, 1, 1, 0}, // Query 3: NDCG = 1.0, first rel rank = 1
		{2, 2, 2, 0, 0}, // Query 4: NDCG = 1.0, first rel rank = 1
		{1, 3, 2, 0, 0}, // Query 5: NDCG = 0.88, first rel rank = 2
	}
	firstRelRanks := []int{1, 1, 1, 1, 2}
	hitCounts := []int64{120, 45, 80, 200, 35} // 0 zero-results
	safesearchTested := 100
	safesearchViolations := 2 // 98% accuracy >= 95%

	repApproved := gate.Evaluate(goldenRelevances, firstRelRanks, hitCounts, safesearchTested, safesearchViolations)

	if !repApproved.Passed {
		t.Fatalf("expected golden set to PASS quality sign-off gate, violations: %v", repApproved.Violations)
	}
	if repApproved.Decision != "APPROVED (GO)" {
		t.Fatalf("expected decision APPROVED (GO), got %s", repApproved.Decision)
	}
	if repApproved.NDCG10 < 0.70 {
		t.Errorf("expected NDCG10 >= 0.70, got %f", repApproved.NDCG10)
	}
	if repApproved.MRR10 < 0.60 {
		t.Errorf("expected MRR10 >= 0.60, got %f", repApproved.MRR10)
	}
	if repApproved.ZeroResultsRate > 0.05 {
		t.Errorf("expected ZeroResultsRate <= 0.05, got %f", repApproved.ZeroResultsRate)
	}

	// 2. Deficient / Degraded Model (low NDCG, high zero results, safe violations)
	deficientRelevances := [][]int{
		{0, 0, 1, 1, 2}, // Rel documents ranked at bottom
		{0, 1, 0, 0, 0},
		{0, 0, 0, 0, 0},
	}
	deficientFirstRanks := []int{5, 2, 0}
	deficientHitCounts := []int64{10, 0, 0} // 2 out of 3 zero results (66%)
	deficientSafeViolations := 15           // 85% accuracy < 95%

	repRejected := gate.Evaluate(deficientRelevances, deficientFirstRanks, deficientHitCounts, safesearchTested, deficientSafeViolations)

	if repRejected.Passed {
		t.Fatalf("expected deficient model to be strictly REJECTED (fail-closed)")
	}
	if repRejected.Decision != "REJECTED (NO-GO)" {
		t.Fatalf("expected decision REJECTED (NO-GO), got %s", repRejected.Decision)
	}
	if len(repRejected.Violations) < 3 {
		t.Errorf("expected multiple violations flagged, got %d: %v", len(repRejected.Violations), repRejected.Violations)
	}
}

func TestEndToEndRankingAndQualityPipeline(t *testing.T) {
	intentClassifier := NewIntentClassifier()
	abuseDetector := NewAbuseDetector(5*time.Second, 10, 5)
	reranker := NewReranker(DefaultRerankerConfig())
	safeClassifier := NewSafeSearchClassifier()

	// 1. Process User Query
	rawQuery := "강남역 맛집 추천 gksrmf"
	clientIP := "203.0.113.42"

	// Abuse inspection
	abuseRep := abuseDetector.InspectQuery(clientIP, rawQuery)
	if abuseRep.IsAbusive {
		t.Fatalf("benign query should not be flagged as abusive")
	}

	// Intent Classification
	intent := intentClassifier.Classify(rawQuery, "ko", "KR")
	if intent.Intent != IntentLocal {
		t.Errorf("expected IntentLocal, got %s", intent.Intent)
	}

	// Opt-in correction
	corrected, wasCorrected := intentClassifier.ApplyCorrection("gksrmf", true)
	if !wasCorrected || corrected != "한글" {
		t.Errorf("expected correction '한글', got %s, %v", corrected, wasCorrected)
	}

	// 2. Candidate retrieval and RRF Hybrid Fusion
	lexicalHits := RankingSystem{
		Name:   "lexical_bm25",
		Weight: 1.0,
		Hits: []RankedItem{
			{DocumentID: "doc-gov-gangnam", Title: "강남구청 공식 모범음식점 안내", URL: "https://gangnam.go.kr/food", Score: 14.5},
			{DocumentID: "doc-spam-gangnam", Title: "강남 맛집 무한키워드도배", URL: "http://spam.test/food", Score: 16.0},
			{DocumentID: "doc-blog-gangnam", Title: "강남역 직장인 점심 맛집 모음", URL: "https://blog.test/gangnam", Score: 12.0},
		},
	}
	semanticHits := RankingSystem{
		Name:   "semantic_embedding",
		Weight: 1.0,
		Hits: []RankedItem{
			{DocumentID: "doc-gov-gangnam", Title: "강남구청 공식 모범음식점 안내", URL: "https://gangnam.go.kr/food", Score: 0.92},
			{DocumentID: "doc-blog-gangnam", Title: "강남역 직장인 점심 맛집 모음", URL: "https://blog.test/gangnam", Score: 0.85},
		},
	}
	fused := ReciprocalRankFusion([]RankingSystem{lexicalHits, semanticHits}, DefaultRRFConfig())
	if len(fused) != 3 {
		t.Fatalf("expected 3 fused hits, got %d", len(fused))
	}

	// 3. Second-Pass Quality, Freshness & Spam Reranker
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	rerankCandidates := []DocumentSignals{
		{
			DocumentID:           "doc-gov-gangnam",
			URL:                  "https://gangnam.go.kr/food",
			Title:                "강남구청 공식 모범음식점 안내",
			BaseScore:            fused[0].RRFScore,
			PublishedAt:          now.Add(-5 * 24 * time.Hour), // Fresh
			DomainAuthority:      0.95,                         // High (.go.kr)
			TermFrequencyDensity: 0.04,
		},
		{
			DocumentID:           "doc-spam-gangnam",
			URL:                  "http://spam.test/food",
			Title:                "강남 맛집 무한키워드도배",
			BaseScore:            fused[1].RRFScore,
			PublishedAt:          now.Add(-2 * 24 * time.Hour),
			DomainAuthority:      0.1,
			TermFrequencyDensity: 0.35, // severe keyword stuffing
			HasHiddenText:        true,
			IsSpamCluster:        true,
		},
		{
			DocumentID:           "doc-blog-gangnam",
			URL:                  "https://blog.test/gangnam",
			Title:                "강남역 직장인 점심 맛집 모음",
			BaseScore:            fused[2].RRFScore,
			PublishedAt:          now.Add(-30 * 24 * time.Hour),
			DomainAuthority:      0.5,
			TermFrequencyDensity: 0.05,
		},
	}

	reranked := reranker.Rerank(rerankCandidates, now)

	// doc-gov-gangnam should dominate Rank 1
	if reranked[0].DocumentID != "doc-gov-gangnam" {
		t.Fatalf("expected doc-gov-gangnam at Rank 1, got %s", reranked[0].DocumentID)
	}
	// doc-spam-gangnam should be penalized to Rank 3
	if reranked[2].DocumentID != "doc-spam-gangnam" {
		t.Fatalf("expected doc-spam-gangnam to be demoted to Rank 3, got %s", reranked[2].DocumentID)
	}

	// 4. SafeSearch Filtering
	safeDocs := []SafeSearchDocument{
		{
			DocumentID: reranked[0].DocumentID,
			URL:        reranked[0].URL,
			Title:      reranked[0].Title,
			Safety:     safeClassifier.ClassifyContent(reranked[0].Title, "", reranked[0].URL),
		},
		{
			DocumentID: reranked[1].DocumentID,
			URL:        reranked[1].URL,
			Title:      reranked[1].Title,
			Safety:     safeClassifier.ClassifyContent(reranked[1].Title, "", reranked[1].URL),
		},
	}
	safeResults := safeClassifier.FilterResults(safeDocs, SafeSearchStrict)
	if len(safeResults) != 2 {
		t.Fatalf("expected both clean documents to pass SafeSearch Strict, got %d", len(safeResults))
	}
}

