package crawler

import (
	"testing"
	"time"
)

func TestRobotsRFC9309Compliance(t *testing.T) {
	robotsTxt := `
# robots.txt for testing RFC 9309 compliance

User-agent: Googlebot
Disallow: /

User-agent: OpenPortalBot
Disallow: /private/
Allow: /private/public-preview/
Disallow: /admin
Allow: /admin/login
Disallow: /*.pdf$
Crawl-delay: 2.5
Sitemap: https://example.com/sitemap.xml
Sitemap: https://example.com/sitemap2.xml

User-agent: *
Disallow: /secret/
Disallow: /temp/
Allow: /temp/view
Disallow: /exact$
`

	policy := ParseRobotsTXT(robotsTxt, 200)

	// 1. Sitemaps extraction
	if len(policy.Sitemaps) != 2 {
		t.Fatalf("expected 2 sitemaps, got %d", len(policy.Sitemaps))
	}
	if policy.Sitemaps[0] != "https://example.com/sitemap.xml" {
		t.Errorf("sitemap 0 mismatch: %s", policy.Sitemaps[0])
	}

	// 2. Specific User-Agent precedence: OpenPortalBot should match its own block, not * or Googlebot
	if !policy.IsAllowed("/private/public-preview/doc", "OpenPortalBot") {
		t.Errorf("expected /private/public-preview/doc to be allowed by OpenPortalBot Allow rule")
	}
	if policy.IsAllowed("/private/hidden", "OpenPortalBot") {
		t.Errorf("expected /private/hidden to be disallowed by OpenPortalBot Disallow rule")
	}

	// 3. Wildcard path matching (* and $)
	if policy.IsAllowed("/docs/manual.pdf", "OpenPortalBot") {
		t.Errorf("expected /docs/manual.pdf to be disallowed by /*.pdf$")
	}
	if !policy.IsAllowed("/docs/manual.pdf.html", "OpenPortalBot") {
		t.Errorf("expected /docs/manual.pdf.html to be allowed (not ending with .pdf)")
	}

	// 4. Fallback to wildcard User-Agent (*) for other bots
	if policy.IsAllowed("/secret/data", "OtherBot") {
		t.Errorf("expected /secret/data to be disallowed for OtherBot by * block")
	}
	if !policy.IsAllowed("/temp/view", "OtherBot") {
		t.Errorf("expected /temp/view to be allowed for OtherBot (Allow longer than Disallow)")
	}
	if policy.IsAllowed("/temp/other", "OtherBot") {
		t.Errorf("expected /temp/other to be disallowed for OtherBot")
	}

	// 5. Crawl delay parsing
	delay := policy.GetCrawlDelay("OpenPortalBot")
	if delay != 2500*time.Millisecond {
		t.Errorf("expected crawl delay of 2.5s, got %v", delay)
	}

	// 6. Tie-breaking rule (RFC 9309 §2.2.2): Equal length between Allow and Disallow -> Allow wins
	tieRobots := `
User-agent: *
Disallow: /item
Allow: /item
`
	tiePolicy := ParseRobotsTXT(tieRobots, 200)
	if !tiePolicy.IsAllowed("/item", "OpenPortalBot") {
		t.Errorf("RFC 9309 §2.2.2 tie-break: Allow must win when pattern lengths are equal")
	}

	// 7. Status code 404 (Not Found) -> Full ALLOW
	policy404 := ParseRobotsTXT("Disallow: /", 404)
	if !policy404.IsAllowed("/any/path", "OpenPortalBot") {
		t.Errorf("RFC 9309 §2.3.1: 4xx must result in full allow")
	}

	// 8. Status code 503 (Service Unavailable) -> Fail-closed Full DISALLOW
	policy503 := ParseRobotsTXT("", 503)
	if policy503.IsAllowed("/any/path", "OpenPortalBot") {
		t.Errorf("RFC 9309 §2.3.1: 5xx must result in fail-closed full disallow")
	}
}

func TestRobotsCache(t *testing.T) {
	cache := NewRobotsCache()
	policy := &RobotsPolicy{
		StatusCode: 200,
		ExpiresAt:  time.Now().UTC().Add(10 * time.Minute),
	}

	cache.Set("https://example.com:443", policy)
	got, found := cache.Get("https://example.com:443")
	if !found || got.StatusCode != 200 {
		t.Fatalf("expected to find cached policy")
	}

	// Expired policy should return found=false
	expiredPolicy := &RobotsPolicy{
		StatusCode: 200,
		ExpiresAt:  time.Now().UTC().Add(-1 * time.Minute),
	}
	cache.Set("https://expired.com:443", expiredPolicy)
	_, foundExpired := cache.Get("https://expired.com:443")
	if foundExpired {
		t.Errorf("expected expired policy to be considered a cache miss")
	}
}

