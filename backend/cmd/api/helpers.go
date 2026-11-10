package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
)

var validTraceID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

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

// requestIsHTTPS reports whether the incoming request used HTTPS, including
// proxies that set X-Forwarded-Proto.

// requestIsHTTPS reports whether the incoming request used HTTPS, including
// proxies that set X-Forwarded-Proto.
func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func resolveHitPublishedAt(h search.Hit, fallback time.Time) time.Time {
	for _, raw := range []string{h.PublishedAt, h.LastCrawledAt} {
		if raw == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed
		}
	}
	return fallback
}

func resolveHitDomainAuthority(h search.Hit) float64 {
	if h.DomainAuthority > 0 {
		return h.DomainAuthority
	}
	u, err := url.Parse(h.URL)
	if err != nil || u.Host == "" {
		return 0.3
	}
	score := 0.4
	if u.Scheme == "https" {
		score += 0.2
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	if len(host) <= 20 {
		score += 0.1
	}
	if strings.Contains(host, ".go.kr") || strings.Contains(host, ".ac.kr") || strings.Contains(host, ".or.kr") || strings.HasSuffix(host, ".org") {
		score += 0.1
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func resolveHitTermDensity(h search.Hit) float64 {
	if h.TermFrequencyDensity > 0 {
		return h.TermFrequencyDensity
	}
	words := strings.Fields(strings.ToLower(h.Title + " " + h.Description))
	if len(words) < 5 {
		return 0.05
	}
	freq := make(map[string]int, len(words))
	maxCount := 0
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()[]{}")
		if len(w) < 2 {
			continue
		}
		freq[w]++
		if freq[w] > maxCount {
			maxCount = freq[w]
		}
	}
	if maxCount == 0 {
		return 0.05
	}
	return float64(maxCount) / float64(len(words))
}

func clientIP(r *http.Request) string {

	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

// aiOverviewEnabled reports whether the operational AI Overview block may be
// served. Default false: presenting extractive snippets as AI output without a
// configured model pipeline is misleading and stays disabled (A-2).

// aiOverviewEnabled reports whether the operational AI Overview block may be
// served. Default false: presenting extractive snippets as AI output without a
// configured model pipeline is misleading and stays disabled (A-2).
func aiOverviewEnabled() bool {
	return os.Getenv("AI_OVERVIEW_ENABLED") == "true"
}

// hybridSearchEnabled reports whether /search fuses the keyword and recall
// retrieval paths (C-6, default on). Set HYBRID_SEARCH_ENABLED=false to fall
// back to the single keyword path.
func hybridSearchEnabled() bool {
	return os.Getenv("HYBRID_SEARCH_ENABLED") != "false"
}

func decodeJSONBody(r *http.Request, dst any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
