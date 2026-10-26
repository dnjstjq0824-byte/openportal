package crawler

import (
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		wantCanonical string
		wantHostname  string
		wantPort      int
		wantPath      string
		expectError   bool
	}{
		{
			name:          "standard HTTPS",
			rawURL:        "https://example.com/path/to/page",
			wantCanonical: "https://example.com/path/to/page",
			wantHostname:  "example.com",
			wantPort:      443,
			wantPath:      "/path/to/page",
			expectError:   false,
		},
		{
			name:          "standard HTTP default port stripped",
			rawURL:        "http://example.com:80/index.html",
			wantCanonical: "http://example.com/index.html",
			wantHostname:  "example.com",
			wantPort:      80,
			wantPath:      "/index.html",
			expectError:   false,
		},
		{
			name:          "custom port preserved",
			rawURL:        "https://example.com:8443/api",
			wantCanonical: "https://example.com:8443/api",
			wantHostname:  "example.com",
			wantPort:      8443,
			wantPath:      "/api",
			expectError:   false,
		},
		{
			name:          "uppercase scheme and host normalized",
			rawURL:        "HTTPS://EXAMPLE.COM/Page",
			wantCanonical: "https://example.com/Page",
			wantHostname:  "example.com",
			wantPort:      443,
			wantPath:      "/Page",
			expectError:   false,
		},
		{
			name:          "IDN punycode conversion for Korean domain",
			rawURL:        "https://검색엔진.한국/소개",
			wantCanonical: "https://xn--c79as89axudhxe.xn--3e0b707e/%EC%86%8C%EA%B0%9C",
			wantHostname:  "xn--c79as89axudhxe.xn--3e0b707e",
			wantPort:      443,
			wantPath:      "/%EC%86%8C%EA%B0%9C",
			expectError:   false,
		},
		{
			name:          "path directory traversal and duplicate slashes resolved",
			rawURL:        "https://example.com/a/b/../c//d/./",
			wantCanonical: "https://example.com/a/c/d/",
			wantHostname:  "example.com",
			wantPort:      443,
			wantPath:      "/a/c/d/",
			expectError:   false,
		},
		{
			name:          "tracking parameters stripped and queries sorted",
			rawURL:        "https://example.com/search?b=2&utm_source=google&a=1&fbclid=xyz&utm_campaign=winter",
			wantCanonical: "https://example.com/search?a=1&b=2",
			wantHostname:  "example.com",
			wantPort:      443,
			expectError:   false,
		},
		{
			name:          "fragment removed",
			rawURL:        "https://example.com/doc#section-2",
			wantCanonical: "https://example.com/doc",
			wantHostname:  "example.com",
			wantPort:      443,
			wantPath:      "/doc",
			expectError:   false,
		},
		{
			name:          "empty path becomes slash",
			rawURL:        "https://example.com",
			wantCanonical: "https://example.com/",
			wantHostname:  "example.com",
			wantPort:      443,
			wantPath:      "/",
			expectError:   false,
		},
		{
			name:        "invalid scheme rejected",
			rawURL:      "ftp://example.com/file.txt",
			expectError: true,
		},
		{
			name:        "javascript scheme rejected",
			rawURL:      "javascript:alert(1)",
			expectError: true,
		},
		{
			name:        "file scheme rejected",
			rawURL:      "file:///etc/passwd",
			expectError: true,
		},
		{
			name:        "empty URL rejected",
			rawURL:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cu, err := Canonicalize(tt.rawURL)
			if (err != nil) != tt.expectError {
				t.Fatalf("Canonicalize(%q) error = %v, expectError = %v", tt.rawURL, err, tt.expectError)
			}
			if tt.expectError {
				return
			}
			if cu.Canonical != tt.wantCanonical {
				t.Errorf("Canonical = %q, want %q", cu.Canonical, tt.wantCanonical)
			}
			if cu.Hostname != tt.wantHostname {
				t.Errorf("Hostname = %q, want %q", cu.Hostname, tt.wantHostname)
			}
			if cu.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", cu.Port, tt.wantPort)
			}
			if tt.wantPath != "" && cu.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", cu.Path, tt.wantPath)
			}
			if len(cu.URLHash) != 64 {
				t.Errorf("URLHash length = %d, want 64", len(cu.URLHash))
			}
			// Verify hash is deterministic
			if HashURL(cu.Canonical) != cu.URLHash {
				t.Errorf("HashURL mismatch: %s != %s", HashURL(cu.Canonical), cu.URLHash)
			}
		})
	}
}
