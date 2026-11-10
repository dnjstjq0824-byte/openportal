package crawler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WARCStore persists serialized WARC records.
type WARCStore interface {
	Write(record *WARCRecord) error
}

// FileWARCStore appends each record to a file under dir, addressed by the
// record's partitioned ObjectKey (C-2). Keys are validated so a malicious
// TargetURI can never escape dir.
type FileWARCStore struct {
	dir string
}

// NewFileWARCStore creates a local-directory WARC archive.
func NewFileWARCStore(dir string) (*FileWARCStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("warc directory is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &FileWARCStore{dir: abs}, nil
}

// Write stores one record. Records sharing an ObjectKey are appended, keeping
// the file a valid multi-record WARC segment.
func (s *FileWARCStore) Write(record *WARCRecord) error {
	if record == nil || len(record.RawData) == 0 {
		return fmt.Errorf("warc record is empty")
	}
	// Reject traversal on the RAW key: Clean would normalize ".." away before
	// any check on the cleaned form could see it.
	if strings.Contains(record.ObjectKey, "..") {
		return fmt.Errorf("warc object key escapes archive: %q", record.ObjectKey)
	}
	key := filepath.Clean("/" + record.ObjectKey)
	path := filepath.Join(s.dir, key)
	if !strings.HasPrefix(path, s.dir+string(os.PathSeparator)) {
		return fmt.Errorf("warc object key escapes archive: %q", record.ObjectKey)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(record.RawData)
	return err
}
