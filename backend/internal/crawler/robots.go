package crawler

import (
	"bufio"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultCrawlerUserAgent is the primary user-agent token for OpenPortal Search crawler.
const DefaultCrawlerUserAgent = "OpenPortalBot"

// RuleType denotes allow or disallow directive.
type RuleType int

const (
	RuleAllow RuleType = iota
	RuleDisallow
)

// PathRule represents a single parsed Allow or Disallow rule in RFC 9309.
type PathRule struct {
	Type        RuleType
	Pattern     string
	Regex       *regexp.Regexp
	PatternLen  int
}

// UserAgentGroup represents a set of rules applying to one or more user-agent identifiers.
type UserAgentGroup struct {
	Agents     []string
	Rules      []PathRule
	CrawlDelay time.Duration
}

// RobotsPolicy represents the parsed RFC 9309 robots.txt policy for a site.
type RobotsPolicy struct {
	StatusCode int
	Groups     []UserAgentGroup
	Sitemaps   []string
	RawBody    string
	FetchedAt  time.Time
	ExpiresAt  time.Time
}

// CompilePattern converts an RFC 9309 robots.txt path pattern to a Go regexp.
// Handles '*' (matches 0 or more characters) and '$' (anchors at end of path).
func CompilePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	var sb strings.Builder
	sb.WriteString("^")
	i := 0
	for i < len(pattern) {
		ch := pattern[i]
		switch ch {
		case '*':
			sb.WriteString(".*")
		case '$':
			if i == len(pattern)-1 {
				sb.WriteString("$")
			} else {
				sb.WriteString(regexp.QuoteMeta("$"))
			}
		default:
			sb.WriteString(regexp.QuoteMeta(string(ch)))
		}
		i++
	}
	// If pattern does not end with '$', it is a prefix match
	if !strings.HasSuffix(pattern, "$") {
		// RFC 9309 prefix match
	}
	return regexp.Compile(sb.String())
}

// ParseRobotsTXT parses robots.txt content according to RFC 9309.
func ParseRobotsTXT(body string, statusCode int) *RobotsPolicy {
	now := time.Now().UTC()
	policy := &RobotsPolicy{
		StatusCode: statusCode,
		RawBody:    body,
		FetchedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}

	// RFC 9309 §2.3.1: Handling of HTTP status codes
	// 4xx: robots.txt is considered nonexistent -> full allow
	if statusCode >= 400 && statusCode < 500 {
		return policy // empty groups means allow-all
	}
	// 5xx: server error -> fail-closed: full disallow
	if statusCode >= 500 && statusCode < 600 {
		// synthesize a disallow-all group
		policy.Groups = []UserAgentGroup{
			{
				Agents: []string{"*"},
				Rules: []PathRule{
					{Type: RuleDisallow, Pattern: "/", PatternLen: 1, Regex: regexp.MustCompile("^/")},
				},
			},
		}
		return policy
	}

	scanner := bufio.NewScanner(strings.NewReader(body))
	var currentGroup *UserAgentGroup
	inGroupMembers := false

	for scanner.Scan() {
		line := scanner.Text()
		// Remove comments (#)
		if idx := strings.IndexByte(line, '#'); idx != -1 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent":
			agent := strings.ToLower(val)
			if inGroupMembers || currentGroup == nil {
				// Starting a new group
				if currentGroup != nil && len(currentGroup.Agents) > 0 {
					policy.Groups = append(policy.Groups, *currentGroup)
				}
				currentGroup = &UserAgentGroup{
					Agents: []string{agent},
				}
				inGroupMembers = false
			} else {
				// Additional user-agent for the same group
				currentGroup.Agents = append(currentGroup.Agents, agent)
			}

		case "disallow":
			if currentGroup == nil {
				continue
			}
			inGroupMembers = true
			if val == "" {
				// An empty Disallow means "allow everything" (RFC 9309 §2.2.1)
				continue
			}
			re, err := CompilePattern(val)
			if err == nil {
				currentGroup.Rules = append(currentGroup.Rules, PathRule{
					Type:       RuleDisallow,
					Pattern:    val,
					Regex:      re,
					PatternLen: len(val),
				})
			}

		case "allow":
			if currentGroup == nil {
				continue
			}
			inGroupMembers = true
			if val == "" {
				continue
			}
			re, err := CompilePattern(val)
			if err == nil {
				currentGroup.Rules = append(currentGroup.Rules, PathRule{
					Type:       RuleAllow,
					Pattern:    val,
					Regex:      re,
					PatternLen: len(val),
				})
			}

		case "crawl-delay":
			if currentGroup == nil {
				continue
			}
			inGroupMembers = true
			if secs, err := strconv.ParseFloat(val, 64); err == nil && secs >= 0 {
				currentGroup.CrawlDelay = time.Duration(secs * float64(time.Second))
			}

		case "sitemap":
			if val != "" {
				policy.Sitemaps = append(policy.Sitemaps, val)
			}
		}
	}

	if currentGroup != nil && len(currentGroup.Agents) > 0 {
		policy.Groups = append(policy.Groups, *currentGroup)
	}

	return policy
}

// IsAllowed evaluates whether the target URL path is allowed for crawling under RFC 9309 §2.2.2.
// Precedence rules:
// 1. Matches most specific User-Agent group (e.g. "openportalbot"), falling back to wildcard "*".
// 2. Evaluates all matching Allow and Disallow rules.
// 3. The rule with the longest matching pattern wins.
// 4. In case of equal length between Allow and Disallow, ALLOW wins.
// 5. If no rules match, ALLOW is the default.
func (p *RobotsPolicy) IsAllowed(targetURLPath string, userAgent string) bool {
	if p == nil {
		return true
	}
	// Fail-closed on 5xx
	if p.StatusCode >= 500 && p.StatusCode < 600 {
		return false
	}
	// Allow-all on 4xx
	if p.StatusCode >= 400 && p.StatusCode < 500 {
		return true
	}

	ua := strings.ToLower(userAgent)
	if ua == "" {
		ua = strings.ToLower(DefaultCrawlerUserAgent)
	}

	// 1. Find matching group: exact agent first, then wildcard
	var matchedGroup *UserAgentGroup
	var wildcardGroup *UserAgentGroup

	for i := range p.Groups {
		grp := &p.Groups[i]
		for _, a := range grp.Agents {
			if a == "*" {
				wildcardGroup = grp
			} else if strings.Contains(ua, a) || a == ua {
				matchedGroup = grp
				break
			}
		}
		if matchedGroup != nil {
			break
		}
	}

	targetGroup := matchedGroup
	if targetGroup == nil {
		targetGroup = wildcardGroup
	}
	if targetGroup == nil || len(targetGroup.Rules) == 0 {
		return true
	}

	// Clean path for evaluation
	evalPath := targetURLPath
	if evalPath == "" {
		evalPath = "/"
	}
	if !strings.HasPrefix(evalPath, "/") {
		evalPath = "/" + evalPath
	}
	// Strip query and fragment if passed
	if idx := strings.IndexAny(evalPath, "?#"); idx != -1 {
		evalPath = evalPath[:idx]
	}

	// RFC 9309 §2.2.2: Find longest matching rule
	var bestMatch *PathRule
	bestLen := -1

	for i := range targetGroup.Rules {
		rule := &targetGroup.Rules[i]
		if rule.Regex == nil {
			continue
		}
		if rule.Regex.MatchString(evalPath) {
			if rule.PatternLen > bestLen {
				bestLen = rule.PatternLen
				bestMatch = rule
			} else if rule.PatternLen == bestLen {
				// Tie-breaking: RFC 9309 §2.2.2: If patterns have equal length, Allow wins
				if rule.Type == RuleAllow {
					bestMatch = rule
				}
			}
		}
	}

	if bestMatch == nil {
		return true
	}
	return bestMatch.Type == RuleAllow
}

// GetCrawlDelay returns the configured crawl delay for the given user-agent, or 0 if none.
func (p *RobotsPolicy) GetCrawlDelay(userAgent string) time.Duration {
	if p == nil {
		return 0
	}
	ua := strings.ToLower(userAgent)
	if ua == "" {
		ua = strings.ToLower(DefaultCrawlerUserAgent)
	}
	var wildcardDelay time.Duration

	for _, grp := range p.Groups {
		for _, a := range grp.Agents {
			if a == "*" {
				wildcardDelay = grp.CrawlDelay
			} else if strings.Contains(ua, a) || a == ua {
				return grp.CrawlDelay
			}
		}
	}
	return wildcardDelay
}

// RobotsCache manages caching of fetched robots.txt policies by host URL.
type RobotsCache struct {
	mu      sync.RWMutex
	entries map[string]*RobotsPolicy
}

func NewRobotsCache() *RobotsCache {
	return &RobotsCache{
		entries: make(map[string]*RobotsPolicy),
	}
}

func (c *RobotsCache) Get(hostKey string) (*RobotsPolicy, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[hostKey]
	if !ok || time.Now().UTC().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, true
}

func (c *RobotsCache) Set(hostKey string, policy *RobotsPolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[hostKey] = policy
}

// RobotsURL constructs the standard robots.txt URL from a base URL.
func RobotsURL(baseRawURL string) (string, error) {
	u, err := url.Parse(baseRawURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host), nil
}
