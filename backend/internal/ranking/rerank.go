package ranking

import (
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

type DocumentSignals struct {
	DocumentID           string    `json:"document_id"`
	URL                  string    `json:"url"`
	Title                string    `json:"title"`
	Description          string    `json:"description"`
	BaseScore            float64   `json:"base_score"`
	PublishedAt          time.Time `json:"published_at"`
	DomainAuthority      float64   `json:"domain_authority"` // 0.0 ~ 1.0
	BrokenLinksRatio     float64   `json:"broken_links_ratio"` // 0.0 ~ 1.0
	TermFrequencyDensity float64   `json:"term_frequency_density"` // > 0.15 indicates keyword stuffing
	HasHiddenText        bool      `json:"has_hidden_text"`
	IsSpamCluster        bool      `json:"is_spam_cluster"`
}

type RerankedHit struct {
	DocumentID   string             `json:"document_id"`
	URL          string             `json:"url"`
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	OriginalRank int                `json:"original_rank"`
	FinalRank    int                `json:"final_rank"`
	BaseScore    float64            `json:"base_score"`
	FinalScore   float64            `json:"final_score"`
	Signals      map[string]float64 `json:"signals"`
}

type RerankerConfig struct {
	FreshnessHalfLifeDays float64
	MaxFreshnessBoost     float64
	HTTPSBoost            float64
	HTTPPenalty           float64
	KeywordStuffingLimit  float64
	MaxBrokenLinkPenalty  float64
}

func DefaultRerankerConfig() RerankerConfig {
	return RerankerConfig{
		FreshnessHalfLifeDays: 60.0,
		MaxFreshnessBoost:     0.25,
		HTTPSBoost:            0.15,
		HTTPPenalty:           0.20,
		KeywordStuffingLimit:  0.15,
		MaxBrokenLinkPenalty:  0.40,
	}
}

type Reranker struct {
	cfg RerankerConfig
}

func NewReranker(cfg RerankerConfig) *Reranker {
	return &Reranker{cfg: cfg}
}

// ComputeFreshnessBoost calculates an exponential decay boost based on publication recency.
func (r *Reranker) ComputeFreshnessBoost(publishedAt time.Time, now time.Time) float64 {
	if publishedAt.IsZero() || publishedAt.After(now) {
		return 0.0
	}
	days := now.Sub(publishedAt).Hours() / 24.0
	if days < 0 {
		days = 0
	}
	decayLambda := math.Ln2 / r.cfg.FreshnessHalfLifeDays
	decay := math.Exp(-decayLambda * days)
	return decay * r.cfg.MaxFreshnessBoost
}

// Rerank evaluates site quality, spam signals, and freshness to produce the final ranking.
func (r *Reranker) Rerank(candidates []DocumentSignals, now time.Time) []RerankedHit {
	if now.IsZero() {
		now = time.Now()
	}

	results := make([]RerankedHit, len(candidates))

	for i, doc := range candidates {
		signals := make(map[string]float64)

		score := doc.BaseScore
		if score <= 0 {
			score = 0.01
		}

		// 1. Protocol boost/penalty
		isHTTPS := strings.HasPrefix(strings.ToLower(doc.URL), "https://")
		if isHTTPS {
			score *= (1.0 + r.cfg.HTTPSBoost)
			signals["https_boost"] = r.cfg.HTTPSBoost
		} else {
			score *= (1.0 - r.cfg.HTTPPenalty)
			signals["http_penalty"] = -r.cfg.HTTPPenalty
		}

		// 2. Domain Authority boost (.go.kr / authoritative domains)
		parsedURL, err := url.Parse(doc.URL)
		authBoost := doc.DomainAuthority * 0.20
		if err == nil && parsedURL != nil {
			host := strings.ToLower(parsedURL.Hostname())
			if strings.HasSuffix(host, ".go.kr") || strings.HasSuffix(host, ".ac.kr") {
				authBoost += 0.10
			}
		}
		score *= (1.0 + authBoost)
		signals["domain_authority_boost"] = authBoost

		// 3. Freshness boost
		freshnessBoost := r.ComputeFreshnessBoost(doc.PublishedAt, now)
		score *= (1.0 + freshnessBoost)
		signals["freshness_boost"] = freshnessBoost

		// 4. Broken links penalty
		brokenPenalty := doc.BrokenLinksRatio * r.cfg.MaxBrokenLinkPenalty
		if brokenPenalty > 0 {
			score *= (1.0 - brokenPenalty)
			signals["broken_links_penalty"] = -brokenPenalty
		}

		// 5. Spam / Manipulation defense
		var spamPenalty float64
		if doc.IsSpamCluster {
			spamPenalty += 0.90
		}
		if doc.HasHiddenText {
			spamPenalty += 0.80
		}
		if doc.TermFrequencyDensity > r.cfg.KeywordStuffingLimit {
			excess := doc.TermFrequencyDensity - r.cfg.KeywordStuffingLimit
			penalty := math.Min(excess*4.0, 0.70)
			spamPenalty += penalty
		}
		if spamPenalty > 0.98 {
			spamPenalty = 0.98
		}
		score *= (1.0 - spamPenalty)
		signals["spam_penalty"] = -spamPenalty

		results[i] = RerankedHit{
			DocumentID:   doc.DocumentID,
			URL:          doc.URL,
			Title:        doc.Title,
			Description:  doc.Description,
			OriginalRank: i + 1,
			BaseScore:    doc.BaseScore,
			FinalScore:   score,
			Signals:      signals,
		}
	}

	// Sort descending by FinalScore with stable tie-breaking on OriginalRank
	sort.SliceStable(results, func(i, j int) bool {
		if math.Abs(results[i].FinalScore-results[j].FinalScore) > 1e-6 {
			return results[i].FinalScore > results[j].FinalScore
		}
		return results[i].OriginalRank < results[j].OriginalRank
	})

	for idx := range results {
		results[idx].FinalRank = idx + 1
	}

	return results
}

