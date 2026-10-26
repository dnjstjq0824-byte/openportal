package crawler

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetcherContentTypeValidation(t *testing.T) {
	// Mock server serving unsupported binary content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0x00, 0x01, 0x02, 0x03})
	}))
	defer server.Close()

	fetcher := &SafeFetcher{client: server.Client()}
	_, err := fetcher.Fetch(context.Background(), server.URL, DefaultFetchOptions())
	if err == nil || !strings.Contains(err.Error(), "unsupported content-type") {
		t.Errorf("expected ErrUnsupportedContentType, got: %v", err)
	}
}

func TestFetcherBodySizeLimit(t *testing.T) {
	// Mock server serving 3 MiB of text (exceeds 2 MiB limit)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("A"), 3*1024*1024))
	}))
	defer server.Close()

	fetcher := &SafeFetcher{client: server.Client()}
	opts := DefaultFetchOptions()
	opts.MaxBodyBytes = 1 * 1024 * 1024 // 1 MiB limit
	_, err := fetcher.Fetch(context.Background(), server.URL, opts)
	if err != ErrBodyTooLarge {
		t.Errorf("expected ErrBodyTooLarge, got: %v", err)
	}
}

func TestFetcherGzipDecompression(t *testing.T) {
	rawHTML := "<html><body><h1>OpenPortal Crawl Test</h1></body></html>"
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte(rawHTML))
	zw.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	fetcher := &SafeFetcher{client: server.Client()}
	res, err := fetcher.Fetch(context.Background(), server.URL, DefaultFetchOptions())
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if string(res.DecompressedBody) != rawHTML {
		t.Errorf("decompressed body mismatch: %s", string(res.DecompressedBody))
	}
}

func TestFetcherDecompressionBombGuard(t *testing.T) {
	// Synthesize a decompression bomb: 10KB compressed expanding to > 500KB (ratio > 50:1)
	hugeData := bytes.Repeat([]byte("0"), 600*1024)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(hugeData)
	zw.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	fetcher := &SafeFetcher{client: server.Client()}
	opts := DefaultFetchOptions()
	opts.MaxDecompressedBytes = 200 * 1024 // 200 KB max limit
	_, err := fetcher.Fetch(context.Background(), server.URL, opts)
	if err != ErrDecompressionBomb {
		t.Errorf("expected ErrDecompressionBomb, got: %v", err)
	}
}

func TestFetcherConditional304NotModified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"test-etag"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"test-etag"`)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>initial</html>"))
	}))
	defer server.Close()

	fetcher := &SafeFetcher{client: server.Client()}
	opts := DefaultFetchOptions()
	opts.ETag = `"test-etag"`

	res, err := fetcher.Fetch(context.Background(), server.URL, opts)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if !res.NotModified {
		t.Errorf("expected NotModified=true on 304 response")
	}
	if res.StatusCode != 304 {
		t.Errorf("expected status 304, got %d", res.StatusCode)
	}
}
