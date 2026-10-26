package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

var (
	ErrInvalidScheme   = errors.New("invalid scheme: only http and https are allowed")
	ErrEmptyHost       = errors.New("host cannot be empty")
	ErrURLTooLong      = errors.New("url exceeds maximum length of 2048 characters")
	ErrInvalidURLChars = errors.New("url contains invalid characters")
)

// trackingParams contains common analytics and tracking query parameters to be stripped during canonicalization.
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"gclid":        true,
	"fbclid":       true,
	"msclkid":      true,
	"zanpid":       true,
	"mc_eid":       true,
	"_ga":          true,
	"_gl":          true,
}

// CanonicalURL represents a canonicalized, normalized web URL ready for crawling and indexing.
type CanonicalURL struct {
	Original    string `json:"original"`
	Canonical   string `json:"canonical"`
	Scheme      string `json:"scheme"`
	Hostname    string `json:"hostname"`
	Port        int    `json:"port"`
	Path        string `json:"path"`
	Query       string `json:"query"`
	URLHash     string `json:"url_hash"` // 64-char hex SHA-256
	HostKey     string `json:"host_key"` // scheme://hostname:port
}

// Canonicalize parses, cleans, normalizes, and hashes a URL according to Task 022 specifications.
// - Scheme: strictly http or https, lowercase.
// - Hostname: lowercase, IDN punycode converted (RFC 5891).
// - Port: default ports (80 for http, 443 for https) stripped.
// - Path: directory traversal resolved, consecutive slashes collapsed, empty path becomes "/".
// - Query: tracking parameters stripped, query keys and values sorted alphabetically.
// - Fragment: completely removed.
// - Hash: deterministic SHA-256 of the canonical URL string.
func Canonicalize(rawURL string) (*CanonicalURL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("url cannot be empty")
	}
	if len(rawURL) > 2048 {
		return nil, ErrURLTooLong
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	// 1. Validate and normalize scheme
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrInvalidScheme
	}

	// 2. Validate and normalize host (including IDN / punycode)
	rawHost := u.Host
	if rawHost == "" {
		return nil, ErrEmptyHost
	}

	hostPart := rawHost
	portPart := ""
	if strings.Contains(rawHost, ":") {
		h, p, err := splitHostPort(rawHost)
		if err != nil {
			return nil, fmt.Errorf("invalid host:port: %w", err)
		}
		hostPart = h
		portPart = p
	}

	if hostPart == "" {
		return nil, ErrEmptyHost
	}

	// Convert IDN hostname to ASCII punycode (e.g. 한글.한국 -> xn--...)
	asciiHost, err := idna.Lookup.ToASCII(strings.ToLower(hostPart))
	if err != nil {
		return nil, fmt.Errorf("failed to convert IDN to punycode: %w", err)
	}
	asciiHost = strings.TrimSuffix(asciiHost, ".") // Remove trailing FQDN dot

	// Resolve effective port
	port := 80
	if scheme == "https" {
		port = 443
	}
	if portPart != "" {
		parsedPort, err := strconv.Atoi(portPart)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return nil, fmt.Errorf("invalid port number: %s", portPart)
		}
		port = parsedPort
	}

	// 3. Normalize path
	cleanedPath := u.Path
	if cleanedPath == "" {
		cleanedPath = "/"
	} else {
		hasTrailingSlash := strings.HasSuffix(cleanedPath, "/")
		cleanedPath = path.Clean(cleanedPath)
		if !strings.HasPrefix(cleanedPath, "/") {
			cleanedPath = "/" + cleanedPath
		}
		if hasTrailingSlash && cleanedPath != "/" && !strings.HasSuffix(cleanedPath, "/") {
			cleanedPath += "/"
		}
	}
	escapedPath := (&url.URL{Path: cleanedPath}).EscapedPath()
	if escapedPath == "" {
		escapedPath = "/"
	}

	// 4. Normalize query parameters
	var cleanQuery string
	if u.RawQuery != "" {
		queryParams, err := url.ParseQuery(u.RawQuery)
		if err == nil && len(queryParams) > 0 {
			filtered := url.Values{}
			for key, vals := range queryParams {
				lowerKey := strings.ToLower(key)
				if trackingParams[lowerKey] {
					continue
				}
				sort.Strings(vals)
				for _, v := range vals {
					filtered.Add(key, v)
				}
			}
			if len(filtered) > 0 {
				cleanQuery = filtered.Encode()
			}
		}
	}

	// 5. Build canonical string (fragment always omitted)
	var canonicalHost string
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		canonicalHost = asciiHost
	} else {
		canonicalHost = fmt.Sprintf("%s:%d", asciiHost, port)
	}

	canonicalStr := fmt.Sprintf("%s://%s%s", scheme, canonicalHost, escapedPath)
	if cleanQuery != "" {
		canonicalStr += "?" + cleanQuery
	}

	// 6. Generate deterministic SHA-256 hash
	hashBytes := sha256.Sum256([]byte(canonicalStr))
	urlHash := hex.EncodeToString(hashBytes[:])

	hostKey := fmt.Sprintf("%s://%s:%d", scheme, asciiHost, port)

	return &CanonicalURL{
		Original:  rawURL,
		Canonical: canonicalStr,
		Scheme:    scheme,
		Hostname:  asciiHost,
		Port:      port,
		Path:      escapedPath,
		Query:     cleanQuery,
		URLHash:   urlHash,
		HostKey:   hostKey,
	}, nil
}

// splitHostPort splits a network host and port, handling IPv6 brackets.
func splitHostPort(host string) (string, string, error) {
	if strings.HasPrefix(host, "[") {
		idx := strings.LastIndex(host, "]")
		if idx == -1 {
			return "", "", errors.New("missing ']' in IPv6 address")
		}
		h := host[1:idx]
		rest := host[idx+1:]
		if strings.HasPrefix(rest, ":") {
			return h, rest[1:], nil
		}
		return h, "", nil
	}
	idx := strings.LastIndex(host, ":")
	if idx == -1 {
		return host, "", nil
	}
	return host[:idx], host[idx+1:], nil
}

// HashURL is a convenience function that computes the deterministic SHA-256 hash of a normalized URL string.
func HashURL(canonicalURL string) string {
	h := sha256.Sum256([]byte(canonicalURL))
	return hex.EncodeToString(h[:])
}

