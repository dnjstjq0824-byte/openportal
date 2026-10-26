package crawler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// WARCRecord represents an ISO 28500 compliant WARC/1.0 response record.
type WARCRecord struct {
	RecordID      string
	Date          time.Time
	TargetURI     string
	ContentType   string
	PayloadDigest string // sha256:<hex>
	BlockDigest   string // sha256:<hex>
	ObjectKey     string // Unique storage key (e.g. raw/YYYY/MM/DD/<hash>.warc)
	RawData       []byte
	PayloadBytes  int64
	ChecksumSHA   string
}

// GenerateObjectKey produces a deterministic, partitioned object storage key for a crawled URL.
func GenerateObjectKey(capturedAt time.Time, urlHash string) string {
	utc := capturedAt.UTC()
	return fmt.Sprintf("raw/%04d/%02d/%02d/%s.warc", utc.Year(), utc.Month(), utc.Day(), urlHash)
}

// CreateWARCResponseRecord serializes an HTTP response and its payload into a standard WARC/1.0 response block.
func CreateWARCResponseRecord(targetURI string, urlHash string, statusCode int, header http.Header, body []byte, capturedAt time.Time) *WARCRecord {
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	} else {
		capturedAt = capturedAt.UTC()
	}

	// 1. Calculate body payload digest
	payloadHash := sha256.Sum256(body)
	payloadDigestHex := hex.EncodeToString(payloadHash[:])
	payloadDigest := fmt.Sprintf("sha256:%s", payloadDigestHex)

	// 2. Build the HTTP response block
	var httpBlock bytes.Buffer
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "Unknown"
	}
	fmt.Fprintf(&httpBlock, "HTTP/1.1 %d %s\r\n", statusCode, statusText)

	// Sort headers deterministically
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range header[k] {
			fmt.Fprintf(&httpBlock, "%s: %s\r\n", k, v)
		}
	}
	httpBlock.WriteString("\r\n")
	httpBlock.Write(body)

	httpBlockBytes := httpBlock.Bytes()

	// 3. Calculate block digest
	blockHash := sha256.Sum256(httpBlockBytes)
	blockDigestHex := hex.EncodeToString(blockHash[:])
	blockDigest := fmt.Sprintf("sha256:%s", blockDigestHex)

	// 4. Build WARC header
	recordID := fmt.Sprintf("<urn:uuid:%s>", urlHash[:32])
	objectKey := GenerateObjectKey(capturedAt, urlHash)

	var warcHeader bytes.Buffer
	warcHeader.WriteString("WARC/1.0\r\n")
	warcHeader.WriteString("WARC-Type: response\r\n")
	fmt.Fprintf(&warcHeader, "WARC-Record-ID: %s\r\n", recordID)
	fmt.Fprintf(&warcHeader, "WARC-Date: %s\r\n", capturedAt.Format(time.RFC3339))
	fmt.Fprintf(&warcHeader, "WARC-Target-URI: %s\r\n", targetURI)
	fmt.Fprintf(&warcHeader, "WARC-Payload-Digest: %s\r\n", payloadDigest)
	fmt.Fprintf(&warcHeader, "WARC-Block-Digest: %s\r\n", blockDigest)
	warcHeader.WriteString("Content-Type: application/http; msgtype=response\r\n")
	fmt.Fprintf(&warcHeader, "Content-Length: %d\r\n", len(httpBlockBytes))
	warcHeader.WriteString("\r\n")

	// Combine WARC header and HTTP block followed by standard two CRLFs
	var fullRecord bytes.Buffer
	fullRecord.Write(warcHeader.Bytes())
	fullRecord.Write(httpBlockBytes)
	fullRecord.WriteString("\r\n\r\n")

	return &WARCRecord{
		RecordID:      recordID,
		Date:          capturedAt,
		TargetURI:     targetURI,
		ContentType:   "application/warc",
		PayloadDigest: payloadDigest,
		BlockDigest:   blockDigest,
		ObjectKey:     objectKey,
		RawData:       fullRecord.Bytes(),
		PayloadBytes:  int64(len(body)),
		ChecksumSHA:   payloadDigestHex,
	}
}

// ParseWARCRecordHeader extracts basic metadata from a serialized WARC record buffer.
func ParseWARCRecordHeader(data []byte) (map[string]string, error) {
	headers := make(map[string]string)
	lines := strings.Split(string(data), "\r\n")
	if len(lines) == 0 || lines[0] != "WARC/1.0" {
		return nil, fmt.Errorf("invalid WARC header: missing WARC/1.0 magic")
	}

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			break // End of WARC header block
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	return headers, nil
}

