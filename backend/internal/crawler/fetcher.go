package crawler

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

var (
	ErrUnsupportedContentType = errors.New("fetcher: unsupported content-type")
	ErrBodyTooLarge           = errors.New("fetcher: response body exceeds maximum allowed size")
	ErrDecompressionBomb      = errors.New("fetcher: suspected decompression bomb (exceeded decompressed size or ratio limit)")
)

// Allowed MIME types for web crawling.
var allowedContentTypes = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
	"text/plain":            true,
	"application/xml":       true,
	"text/xml":              true,
	"application/rss+xml":   true,
	"application/atom+xml":  true,
}

// FetchOptions controls limits and headers for a web fetch request.
type FetchOptions struct {
	RejectRedirects      bool          // Bounded callers must recheck policy before following a new URL.
	MaxBodyBytes         int64         // e.g. 2 MiB (2,097,152 bytes)
	MaxDecompressedBytes int64         // e.g. 5 MiB (5,242,880 bytes)
	Timeout              time.Duration // e.g. 10s
	UserAgent            string
	ETag                 string
	LastModified         string
}

func DefaultFetchOptions() FetchOptions {
	return FetchOptions{
		MaxBodyBytes:         2 * 1024 * 1024, // 2 MiB
		MaxDecompressedBytes: 5 * 1024 * 1024, // 5 MiB
		Timeout:              10 * time.Second,
		UserAgent:            "OpenPortalBot/1.0 (+https://openportal.org/bot.html)",
	}
}

// FetchResult represents the structured result of a completed fetch attempt.
type FetchResult struct {
	StatusCode       int
	Header           http.Header
	ContentType      string
	RawBody          []byte
	DecompressedBody []byte
	TransferredBytes int64
	Elapsed          time.Duration
	NotModified      bool
}

// SafeFetcher executes protected HTTP requests with SSRF guards, size limits, and decompression bomb protection.
type SafeFetcher struct {
	client *http.Client
}

func NewSafeFetcher(guard *SSRFGuard) *SafeFetcher {
	if guard == nil {
		guard = NewSSRFGuard()
	}
	return &SafeFetcher{
		client: guard.SafeClient(),
	}
}

// NewSafeFetcherWithClient constructs a SafeFetcher using a custom HTTP client (e.g. for testing).
func NewSafeFetcherWithClient(client *http.Client) *SafeFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &SafeFetcher{client: client}
}

// Fetch retrieves the URL with full security guards applied.
func (f *SafeFetcher) Fetch(ctx context.Context, targetURL string, opts FetchOptions) (*FetchResult, error) {
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 2 * 1024 * 1024
	}
	if opts.MaxDecompressedBytes <= 0 {
		opts.MaxDecompressedBytes = 5 * 1024 * 1024
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultFetchOptions().UserAgent
	}

	fetchCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch request: %w", err)
	}

	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.1")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	if opts.ETag != "" {
		req.Header.Set("If-None-Match", opts.ETag)
	}
	if opts.LastModified != "" {
		req.Header.Set("If-Modified-Since", opts.LastModified)
	}

	start := time.Now()
	client := *f.client
	if opts.RejectRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch request failed: %w", err)
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		return &FetchResult{
			StatusCode:       resp.StatusCode,
			Header:           resp.Header.Clone(),
			ContentType:      resp.Header.Get("Content-Type"),
			NotModified:      true,
			TransferredBytes: 0,
			Elapsed:          elapsed,
		}, nil
	}

	// Verify Content-Type before reading full body
	rawContentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(rawContentType)
	if err != nil || mediaType == "" {
		mediaType = "text/html" // Default fallback
	}
	mediaType = strings.ToLower(mediaType)
	if !allowedContentTypes[mediaType] && resp.StatusCode == http.StatusOK {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedContentType, rawContentType)
	}

	// Read raw body with strict limit reader (+1 byte to detect overflow)
	limitReader := io.LimitReader(resp.Body, opts.MaxBodyBytes+1)
	rawBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(rawBytes)) > opts.MaxBodyBytes {
		return nil, ErrBodyTooLarge
	}

	// Handle decompression with bomb guard
	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	var decompressed []byte
	switch encoding {
	case "gzip":
		decompressed, err = decompressGzip(rawBytes, opts.MaxDecompressedBytes)
		if err != nil {
			return nil, err
		}
	case "deflate":
		decompressed, err = decompressDeflate(rawBytes, opts.MaxDecompressedBytes)
		if err != nil {
			return nil, err
		}
	default:
		decompressed = rawBytes
	}

	return &FetchResult{
		StatusCode:       resp.StatusCode,
		Header:           resp.Header.Clone(),
		ContentType:      mediaType,
		RawBody:          rawBytes,
		DecompressedBody: decompressed,
		TransferredBytes: int64(len(rawBytes)),
		Elapsed:          elapsed,
		NotModified:      false,
	}, nil
}

// decompressGzip decompresses gzip payload while enforcing maximum decompressed bytes and ratio limits.
func decompressGzip(compressed []byte, maxBytes int64) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("gzip reader error: %w", err)
	}
	defer reader.Close()

	return readWithBombGuard(reader, int64(len(compressed)), maxBytes)
}

// decompressDeflate decompresses deflate payload while enforcing maximum decompressed bytes and ratio limits.
func decompressDeflate(compressed []byte, maxBytes int64) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(compressed))
	defer reader.Close()

	return readWithBombGuard(reader, int64(len(compressed)), maxBytes)
}

// readWithBombGuard reads from a decompressor with a maximum byte limit and compression ratio limit (max 25x).
func readWithBombGuard(r io.Reader, compressedLen int64, maxBytes int64) ([]byte, error) {
	limitReader := io.LimitReader(r, maxBytes+1)
	decompressed, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}
	if int64(len(decompressed)) > maxBytes {
		return nil, ErrDecompressionBomb
	}
	// Check expansion ratio: if compressed is non-trivial (>1KB) and ratio exceeds 25:1, flag bomb
	if compressedLen > 1024 && int64(len(decompressed))/compressedLen > 25 {
		return nil, ErrDecompressionBomb
	}
	return decompressed, nil
}

