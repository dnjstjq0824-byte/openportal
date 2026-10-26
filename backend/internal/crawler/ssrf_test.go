package crawler

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
)

func TestSSRFBlockedIPs(t *testing.T) {
	blockedCases := []string{
		"127.0.0.1",
		"127.0.1.1",
		"10.0.0.1",
		"10.254.1.1",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.1.1",
		"192.168.100.50",
		"169.254.169.254", // Cloud metadata
		"169.254.1.1",     // Link local
		"100.64.0.1",      // Carrier grade NAT
		"224.0.0.1",       // Multicast
		"240.0.0.1",       // Reserved
		"192.0.2.1",       // Documentation
		"198.51.100.1",    // Documentation
		"203.0.113.1",     // Documentation
		"::1",             // IPv6 Loopback
		"fe80::1",         // IPv6 Link-local
		"fc00::1",         // IPv6 ULA
	}

	for _, ipStr := range blockedCases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse test IP: %s", ipStr)
		}
		if !IsBlockedIP(ip) {
			t.Errorf("IsBlockedIP(%q) = false, want true (should be blocked)", ipStr)
		}
	}

	allowedCases := []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
		"142.250.190.46",
		"2606:4700:4700::1111",
	}

	for _, ipStr := range allowedCases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse test IP: %s", ipStr)
		}
		if IsBlockedIP(ip) {
			t.Errorf("IsBlockedIP(%q) = true, want false (public IP should be allowed)", ipStr)
		}
	}
}

func TestSSRFBlockedHostnames(t *testing.T) {
	blockedHosts := []string{
		"localhost",
		"metadata.google.internal",
		"instance-data",
		"foo.localhost",
		"app.internal",
		"router.local",
	}

	for _, host := range blockedHosts {
		if !IsBlockedHost(host) {
			t.Errorf("IsBlockedHost(%q) = false, want true", host)
		}
	}

	allowedHosts := []string{
		"example.com",
		"openportal.org",
		"wikipedia.org",
	}

	for _, host := range allowedHosts {
		if IsBlockedHost(host) {
			t.Errorf("IsBlockedHost(%q) = true, want false", host)
		}
	}
}

func TestSSRFGuardValidateTarget(t *testing.T) {
	guard := NewSSRFGuard()
	ctx := context.Background()

	// 1. Literal private IP in URL
	_, _, err := guard.ValidateTarget(ctx, "http://127.0.0.1:8080/secret")
	if err == nil {
		t.Errorf("expected error for 127.0.0.1, got nil")
	}

	// 2. Metadata IP in URL
	_, _, err = guard.ValidateTarget(ctx, "http://169.254.169.254/computeMetadata/v1/")
	if err == nil {
		t.Errorf("expected error for cloud metadata IP, got nil")
	}

	// 3. Metadata Hostname in URL
	_, _, err = guard.ValidateTarget(ctx, "http://metadata.google.internal/computeMetadata")
	if err == nil {
		t.Errorf("expected error for metadata hostname, got nil")
	}

	// 4. Literal public IP in URL
	ip, port, err := guard.ValidateTarget(ctx, "https://93.184.216.34:443/doc")
	if err != nil {
		t.Fatalf("unexpected error for public IP: %v", err)
	}
	if ip.String() != "93.184.216.34" || port != 443 {
		t.Errorf("got IP=%v, port=%v, want 93.184.216.34:443", ip, port)
	}
}

func TestSSRFClientRedirectProtection(t *testing.T) {
	guard := NewSSRFGuard()
	client := guard.SafeClient()

	// 1. Protocol downgrade (HTTPS -> HTTP) must be rejected
	req1, _ := http.NewRequest("GET", "https://example.com/login", nil)
	via := []*http.Request{req1}
	downgradeReq, _ := http.NewRequest("GET", "http://example.com/insecure", nil)
	err := client.CheckRedirect(downgradeReq, via)
	if err != ErrSSRFProtocolDowngrade {
		t.Errorf("expected ErrSSRFProtocolDowngrade, got %v", err)
	}

	// 2. Max redirects exceeded
	maxVia := make([]*http.Request, 5)
	for i := range maxVia {
		u, _ := url.Parse("https://example.com/step")
		maxVia[i] = &http.Request{URL: u}
	}
	nextReq, _ := http.NewRequest("GET", "https://example.com/overflow", nil)
	err = client.CheckRedirect(nextReq, maxVia)
	if err != ErrSSRFRedirectLimit {
		t.Errorf("expected ErrSSRFRedirectLimit, got %v", err)
	}

	// 3. Redirect targeting blocked hostname (e.g. metadata)
	normalReq, _ := http.NewRequest("GET", "https://example.com/redirect", nil)
	viaNormal := []*http.Request{normalReq}
	metadataReq, _ := http.NewRequest("GET", "https://metadata.google.internal/secret", nil)
	err = client.CheckRedirect(metadataReq, viaNormal)
	if err == nil {
		t.Errorf("expected redirect to metadata host to be blocked")
	}
}

