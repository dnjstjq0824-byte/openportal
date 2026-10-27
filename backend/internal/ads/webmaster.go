package ads

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// VerificationMethod represents the mechanism used to prove site ownership.
type VerificationMethod string

const (
	VerifyDNSTXT     VerificationMethod = "dns_txt"
	VerifyHTMLMeta   VerificationMethod = "html_meta"
	VerifyFileUpload VerificationMethod = "file_upload"
)

// DomainVerificationRecord holds the ownership verification state of a site.
type DomainVerificationRecord struct {
	AccountID          string             `json:"account_id"`
	Domain             string             `json:"domain"`
	Method             VerificationMethod `json:"method"`
	ExpectedToken      string             `json:"expected_token"`
	IsVerified         bool               `json:"is_verified"`
	VerifiedAt         *time.Time         `json:"verified_at,omitempty"`
}

// SitemapRecord tracks submitted XML sitemaps.
type SitemapRecord struct {
	SitemapURL  string    `json:"sitemap_url"`
	Domain      string    `json:"domain"`
	SubmittedAt time.Time `json:"submitted_at"`
	Status      string    `json:"status"` // pending, processed, error
	URLCount    int       `json:"url_count"`
}

// URLInspectionResult contains search index status for a requested URL.
type URLInspectionResult struct {
	URL          string    `json:"url"`
	Domain       string    `json:"domain"`
	IsIndexed    bool      `json:"is_indexed"`
	RobotsAllowed bool     `json:"robots_allowed"`
	CanonicalURL string    `json:"canonical_url"`
	LastCrawledAt time.Time `json:"last_crawled_at"`
}

// WebmasterManager manages domain verification, sitemaps, and URL inspection.
type WebmasterManager struct {
	mu            sync.RWMutex
	verifications map[string]DomainVerificationRecord // key: accountID + ":" + domain
	sitemaps      map[string][]SitemapRecord          // key: domain
}

// NewWebmasterManager creates a new instance.
func NewWebmasterManager() *WebmasterManager {
	return &WebmasterManager{
		verifications: make(map[string]DomainVerificationRecord),
		sitemaps:      make(map[string][]SitemapRecord),
	}
}

// GenerateVerificationRecord creates a pending domain verification record with a unique token.
func (m *WebmasterManager) GenerateVerificationRecord(accountID, domain string, method VerificationMethod) (*DomainVerificationRecord, error) {
	if accountID == "" || strings.TrimSpace(domain) == "" {
		return nil, errors.New("account_id and domain are required")
	}

	cleanDomain := strings.ToLower(strings.TrimSpace(domain))
	token := fmt.Sprintf("openportal-site-verification=%s-%d", cleanDomain, time.Now().UnixNano())

	key := accountID + ":" + cleanDomain
	rec := DomainVerificationRecord{
		AccountID:     accountID,
		Domain:        cleanDomain,
		Method:        method,
		ExpectedToken: token,
		IsVerified:    false,
	}

	m.mu.Lock()
	m.verifications[key] = rec
	m.mu.Unlock()

	return &rec, nil
}

// ConfirmVerification checks presented proof against the expected verification token.
func (m *WebmasterManager) ConfirmVerification(accountID, domain, presentedToken string) (bool, error) {
	cleanDomain := strings.ToLower(strings.TrimSpace(domain))
	key := accountID + ":" + cleanDomain

	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.verifications[key]
	if !ok {
		return false, fmt.Errorf("no verification record found for %s", cleanDomain)
	}

	if !strings.EqualFold(strings.TrimSpace(presentedToken), strings.TrimSpace(rec.ExpectedToken)) {
		return false, errors.New("verification token mismatch")
	}

	now := time.Now()
	rec.IsVerified = true
	rec.VerifiedAt = &now
	m.verifications[key] = rec
	return true, nil
}

// SubmitSitemap registers a sitemap for a verified domain.
func (m *WebmasterManager) SubmitSitemap(accountID, rawURL string) (*SitemapRecord, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("invalid sitemap URL")
	}

	domain := strings.ToLower(parsed.Hostname())
	key := accountID + ":" + domain

	m.mu.RLock()
	rec, verified := m.verifications[key]
	m.mu.RUnlock()

	if !verified || !rec.IsVerified {
		return nil, fmt.Errorf("domain %s is not verified for account %s; verify ownership first", domain, accountID)
	}

	entry := SitemapRecord{
		SitemapURL:  rawURL,
		Domain:      domain,
		SubmittedAt: time.Now(),
		Status:      "processed",
		URLCount:    10, // Simulated discovered URLs
	}

	m.mu.Lock()
	m.sitemaps[domain] = append(m.sitemaps[domain], entry)
	m.mu.Unlock()

	return &entry, nil
}

