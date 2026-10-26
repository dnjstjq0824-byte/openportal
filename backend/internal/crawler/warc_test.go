package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCreateWARCResponseRecord(t *testing.T) {
	targetURI := "https://example.com/docs/intro"
	urlHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	body := []byte("<html><body><h1>Hello OpenPortal</h1></body></html>")
	header := make(http.Header)
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Server", "TestServer/1.0")

	capturedAt := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	record := CreateWARCResponseRecord(targetURI, urlHash, 200, header, body, capturedAt)

	if record == nil {
		t.Fatalf("expected non-nil WARC record")
	}

	// Verify object key format
	expectedKey := "raw/2026/10/26/" + urlHash + ".warc"
	if record.ObjectKey != expectedKey {
		t.Errorf("ObjectKey = %s, want %s", record.ObjectKey, expectedKey)
	}

	// Verify payload SHA-256 digest
	expectedHash := sha256.Sum256(body)
	expectedHex := hex.EncodeToString(expectedHash[:])
	if record.ChecksumSHA != expectedHex {
		t.Errorf("ChecksumSHA = %s, want %s", record.ChecksumSHA, expectedHex)
	}
	if record.PayloadDigest != "sha256:"+expectedHex {
		t.Errorf("PayloadDigest = %s, want sha256:%s", record.PayloadDigest, expectedHex)
	}

	// Verify WARC headers parsed
	warcHeaders, err := ParseWARCRecordHeader(record.RawData)
	if err != nil {
		t.Fatalf("ParseWARCRecordHeader failed: %v", err)
	}

	if warcHeaders["warc-type"] != "response" {
		t.Errorf("warc-type = %s, want response", warcHeaders["warc-type"])
	}
	if warcHeaders["warc-target-uri"] != targetURI {
		t.Errorf("warc-target-uri = %s, want %s", warcHeaders["warc-target-uri"], targetURI)
	}
	if warcHeaders["warc-date"] != "2026-10-26T12:00:00Z" {
		t.Errorf("warc-date = %s, want 2026-10-26T12:00:00Z", warcHeaders["warc-date"])
	}

	// Verify full payload contains HTTP status line and body
	rawStr := string(record.RawData)
	if !strings.Contains(rawStr, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("raw WARC record missing HTTP/1.1 200 OK")
	}
	if !strings.Contains(rawStr, string(body)) {
		t.Errorf("raw WARC record missing response body")
	}
}

