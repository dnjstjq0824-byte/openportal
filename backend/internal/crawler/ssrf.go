package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrSSRFBlockedIP        = errors.New("ssrf: target ip address is in a forbidden private or reserved range")
	ErrSSRFBlockedHost      = errors.New("ssrf: target hostname is forbidden (internal/metadata)")
	ErrSSRFRedirectLimit    = errors.New("ssrf: maximum redirect limit exceeded")
	ErrSSRFProtocolDowngrade = errors.New("ssrf: protocol downgrade from https to http is forbidden")
	ErrSSRFNoValidIP        = errors.New("ssrf: no valid public ip resolved for host")
)

// Blocked CIDR ranges for SSRF prevention.
var blockedCIDRs = []string{
	// IPv4 Loopback
	"127.0.0.0/8",
	// IPv4 Private networks (RFC 1918)
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	// Carrier Grade NAT (RFC 6598)
	"100.64.0.0/10",
	// IPv4 Link-local (RFC 3927) & Cloud Metadata
	"169.254.0.0/16",
	// Multicast & Broadcast
	"224.0.0.0/4",
	"255.255.255.255/32",
	// IETF Documentation & Reserved (RFC 5737 / RFC 2544)
	"192.0.2.0/24",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"198.18.0.0/15",
	"240.0.0.0/4",
	"0.0.0.0/8",
	// IPv6 Loopback & Unspecified
	"::1/128",
	"::/128",
	// IPv6 Unique Local Address (RFC 4193)
	"fc00::/7",
	// IPv6 Link-local Unicast (RFC 4291)
	"fe80::/10",
	// IPv6 Multicast
	"ff00::/8",
	// IPv6 Documentation (RFC 3849)
	"2001:db8::/32",
}

var parsedBlockedNets []*net.IPNet

func init() {
	for _, cidr := range blockedCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedBlockedNets = append(parsedBlockedNets, ipNet)
		}
	}
}

// Blocked internal or metadata hostnames.
var blockedHostnames = map[string]bool{
	"localhost":                true,
	"metadata.google.internal": true,
	"instance-data":            true,
	"169.254.169.254":          true,
	"127.0.0.1":                true,
	"::1":                      true,
}

// IsBlockedIP checks whether an IP address belongs to loopback, private, link-local, multicast, or reserved ranges.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, blockedNet := range parsedBlockedNets {
		if blockedNet.Contains(ip) {
			return true
		}
	}
	return false
}

// IsBlockedHost checks whether a hostname is explicitly prohibited.
func IsBlockedHost(host string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))
	if blockedHostnames[lower] {
		return true
	}
	if strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".local") || strings.HasSuffix(lower, ".internal") {
		return true
	}
	return false
}

// SSRFGuard provides safe network dialing and HTTP client configuration to prevent SSRF and DNS rebinding.
type SSRFGuard struct {
	DNSResolver  *net.Resolver
	DialTimeout  time.Duration
	MaxRedirects int
}

func NewSSRFGuard() *SSRFGuard {
	return &SSRFGuard{
		DNSResolver:  net.DefaultResolver,
		DialTimeout:  5 * time.Second,
		MaxRedirects: 5,
	}
}

// ValidateTarget resolves all IPs for a target URL and verifies they are safe public addresses.
// Returns the first validated IP for IP pinning to eliminate DNS rebinding.
func (g *SSRFGuard) ValidateTarget(ctx context.Context, targetURL string) (net.IP, int, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, 0, err
	}
	host := u.Hostname()
	if IsBlockedHost(host) {
		return nil, 0, fmt.Errorf("%w: %s", ErrSSRFBlockedHost, host)
	}

	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		p, err := net.LookupPort("tcp", u.Port())
		if err != nil {
			return nil, 0, err
		}
		port = p
	}

	// If host is already an IP literal
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return nil, 0, fmt.Errorf("%w: %s", ErrSSRFBlockedIP, ip.String())
		}
		return ip, port, nil
	}

	// Resolve all IP addresses via DNS
	ips, err := g.DNSResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, 0, fmt.Errorf("dns resolution failed for %s: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, 0, ErrSSRFNoValidIP
	}

	// Ensure ALL resolved IPs are safe. If a malicious DNS returns both a public IP and private IP, reject!
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return nil, 0, fmt.Errorf("%w: %s resolved to forbidden ip %s", ErrSSRFBlockedIP, host, ip.String())
		}
	}

	return ips[0], port, nil
}

// SafeTransport returns an http.Transport configured with IP pinning and SSRF defense.
func (g *SSRFGuard) SafeTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if IsBlockedHost(host) {
				return nil, fmt.Errorf("%w: %s", ErrSSRFBlockedHost, host)
			}

			// Resolve host and validate IPs
			ips, err := g.DNSResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			var pinnedIP net.IP
			for _, ip := range ips {
				if IsBlockedIP(ip) {
					return nil, fmt.Errorf("%w: %s resolved to %s", ErrSSRFBlockedIP, host, ip.String())
				}
				if pinnedIP == nil {
					pinnedIP = ip
				}
			}
			if pinnedIP == nil {
				return nil, ErrSSRFNoValidIP
			}

			// Pin connection directly to the verified IP
			pinnedAddr := net.JoinHostPort(pinnedIP.String(), portStr)
			dialer := &net.Dialer{
				Timeout: g.DialTimeout,
			}
			return dialer.DialContext(ctx, network, pinnedAddr)
		},
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		DisableKeepAlives:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// SafeClient returns an http.Client equipped with SSRF safe transport and redirect verification.
func (g *SSRFGuard) SafeClient() *http.Client {
	return &http.Client{
		Transport: g.SafeTransport(),
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= g.MaxRedirects {
				return ErrSSRFRedirectLimit
			}

			// Check for protocol downgrade (HTTPS -> HTTP)
			lastReq := via[len(via)-1]
			if lastReq.URL.Scheme == "https" && req.URL.Scheme == "http" {
				return ErrSSRFProtocolDowngrade
			}

			// Validate redirect target hostname and IP
			targetHost := req.URL.Hostname()
			if IsBlockedHost(targetHost) {
				return fmt.Errorf("%w: %s in redirect", ErrSSRFBlockedHost, targetHost)
			}

			// Inspect redirect target IP immediately
			ips, err := g.DNSResolver.LookupIP(req.Context(), "ip", targetHost)
			if err != nil {
				return err
			}
			for _, ip := range ips {
				if IsBlockedIP(ip) {
					return fmt.Errorf("%w: redirect target %s resolves to %s", ErrSSRFBlockedIP, targetHost, ip.String())
				}
			}

			return nil
		},
	}
}

