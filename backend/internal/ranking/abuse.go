package ranking

import (
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

type AbuseType string

const (
	AbuseNone              AbuseType = "none"
	AbuseBurstRate         AbuseType = "burst_rate_limit"
	AbuseManipulation      AbuseType = "manipulation_stuffing"
	AbusePolicyBlacklist   AbuseType = "policy_blacklist"
	AbusePIILeak           AbuseType = "pii_leakage"
	AbuseEntropyAnomaly    AbuseType = "entropy_anomaly"
)

type AbuseReport struct {
	IsAbusive bool      `json:"is_abusive"`
	Type      AbuseType `json:"type"`
	Reason    string    `json:"reason"`
	Score     float64   `json:"score"` // 0.0 (benign) to 1.0 (malicious)
}

type clientRecord struct {
	timestamps []time.Time
}

type AbuseDetector struct {
	mu            sync.RWMutex
	window        time.Duration
	maxBurstIP    int
	maxBurstQuery int
	ipHistory     map[string]*clientRecord
	queryHistory  map[string]*clientRecord
	purgedTerms   map[string]string // term -> reason
	blacklisted   map[string]string // normalized term -> category
}

var (
	// PII Patterns: Korean Resident Registration Number (RRN), Credit Card, Phone Number
	rrnPattern        = regexp.MustCompile(`\b\d{6}-[1-4]\d{6}\b`)
	creditCardPattern = regexp.MustCompile(`\b(?:\d{4}-){3}\d{4}\b|\b\d{16}\b`)
	phonePattern      = regexp.MustCompile(`\b01[0-9]-\d{3,4}-\d{4}\b`)

	// Default policy violations
	defaultBlacklist = map[string]string{
		"불법도박":   "illegal_gambling",
		"마약판매":   "illegal_substance",
		"해킹툴다운":  "malware_distribution",
		"성매매알선":  "commercial_sex",
		"주민등록번호": "pii_query",
		"카드번호유출": "pii_query",
		"자살방법":   "self_harm",
	}
)

func NewAbuseDetector(window time.Duration, maxBurstIP, maxBurstQuery int) *AbuseDetector {
	if window <= 0 {
		window = 10 * time.Second
	}
	if maxBurstIP <= 0 {
		maxBurstIP = 25
	}
	if maxBurstQuery <= 0 {
		maxBurstQuery = 10
	}

	ad := &AbuseDetector{
		window:        window,
		maxBurstIP:    maxBurstIP,
		maxBurstQuery: maxBurstQuery,
		ipHistory:     make(map[string]*clientRecord),
		queryHistory:  make(map[string]*clientRecord),
		purgedTerms:   make(map[string]string),
		blacklisted:   make(map[string]string),
	}

	for k, v := range defaultBlacklist {
		ad.blacklisted[strings.ToLower(k)] = v
	}

	return ad
}

// CalculateShannonEntropy measures informational entropy of the query string.
func CalculateShannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0.0
	}
	counts := make(map[rune]float64)
	total := float64(0)
	for _, r := range s {
		counts[r]++
		total++
	}
	var entropy float64
	for _, count := range counts {
		p := count / total
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// InspectQuery evaluates a query for abuse, policy violations, or PII leakage.
func (ad *AbuseDetector) InspectQuery(clientIP, query string) AbuseReport {
	q := strings.TrimSpace(query)
	norm := strings.ToLower(q)

	// 1. PII Check
	if rrnPattern.MatchString(q) {
		return AbuseReport{
			IsAbusive: true,
			Type:      AbusePIILeak,
			Reason:    "query contains resident registration number (RRN)",
			Score:     1.0,
		}
	}
	if creditCardPattern.MatchString(q) {
		return AbuseReport{
			IsAbusive: true,
			Type:      AbusePIILeak,
			Reason:    "query contains payment card number pattern",
			Score:     1.0,
		}
	}
	if phonePattern.MatchString(q) {
		return AbuseReport{
			IsAbusive: true,
			Type:      AbusePIILeak,
			Reason:    "query contains mobile phone number pattern",
			Score:     0.9,
		}
	}

	// 2. Blacklist / Purged check
	ad.mu.RLock()
	if reason, ok := ad.purgedTerms[norm]; ok {
		ad.mu.RUnlock()
		return AbuseReport{
			IsAbusive: true,
			Type:      AbusePolicyBlacklist,
			Reason:    "query has been purged: " + reason,
			Score:     1.0,
		}
	}
	for blocked, cat := range ad.blacklisted {
		if strings.Contains(norm, blocked) {
			ad.mu.RUnlock()
			return AbuseReport{
				IsAbusive: true,
				Type:      AbusePolicyBlacklist,
				Reason:    "query violates policy: " + cat,
				Score:     1.0,
			}
		}
	}
	ad.mu.RUnlock()

	// 3. Synthetic Entropy / Repetition check (e.g. "aaaaaaaaaaaa" or repeated patterns)
	if len(q) >= 12 {
		entropy := CalculateShannonEntropy(q)
		if entropy < 1.2 {
			return AbuseReport{
				IsAbusive: true,
				Type:      AbuseEntropyAnomaly,
				Reason:    "query exhibits abnormally low character entropy (repetition spam)",
				Score:     0.85,
			}
		}
	}

	// 4. Burst Rate Limit Check
	if clientIP != "" {
		ad.mu.Lock()
		defer ad.mu.Unlock()

		now := time.Now()
		cutoff := now.Add(-ad.window)

		// Check IP burst
		rec, exists := ad.ipHistory[clientIP]
		if !exists {
			rec = &clientRecord{}
			ad.ipHistory[clientIP] = rec
		}
		// prune old timestamps
		validIP := make([]time.Time, 0, len(rec.timestamps)+1)
		for _, t := range rec.timestamps {
			if t.After(cutoff) {
				validIP = append(validIP, t)
			}
		}
		validIP = append(validIP, now)
		rec.timestamps = validIP
		if len(validIP) > ad.maxBurstIP {
			return AbuseReport{
				IsAbusive: true,
				Type:      AbuseBurstRate,
				Reason:    "client IP request rate exceeded threshold",
				Score:     0.95,
			}
		}

		// Check Query burst (manipulation / keyword stuffing attack)
		qKey := clientIP + "::" + norm
		qRec, qExists := ad.queryHistory[qKey]
		if !qExists {
			qRec = &clientRecord{}
			ad.queryHistory[qKey] = qRec
		}
		validQ := make([]time.Time, 0, len(qRec.timestamps)+1)
		for _, t := range qRec.timestamps {
			if t.After(cutoff) {
				validQ = append(validQ, t)
			}
		}
		validQ = append(validQ, now)
		qRec.timestamps = validQ
		if len(validQ) > ad.maxBurstQuery {
			return AbuseReport{
				IsAbusive: true,
				Type:      AbuseManipulation,
				Reason:    "identical query repetition burst detected (manipulation attempt)",
				Score:     0.90,
			}
		}
	}

	return AbuseReport{
		IsAbusive: false,
		Type:      AbuseNone,
		Score:     0.0,
	}
}

// Purge immediately blocks a term from suggestions and searches.
func (ad *AbuseDetector) Purge(term, reason string) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	norm := strings.ToLower(strings.TrimSpace(term))
	if norm != "" {
		ad.purgedTerms[norm] = reason
	}
}

// FilterSuggestions cleans a list of suggestions by removing any blocked or blacklisted terms.
func (ad *AbuseDetector) FilterSuggestions(suggestions []string) []string {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	res := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		norm := strings.ToLower(strings.TrimSpace(s))
		if norm == "" {
			continue
		}
		if _, ok := ad.purgedTerms[norm]; ok {
			continue
		}
		blocked := false
		for b := range ad.blacklisted {
			if strings.Contains(norm, b) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		// PII in suggestion
		if rrnPattern.MatchString(s) || creditCardPattern.MatchString(s) || phonePattern.MatchString(s) {
			continue
		}
		res = append(res, s)
	}
	return res
}

