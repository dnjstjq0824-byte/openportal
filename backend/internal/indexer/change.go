package indexer

import (
	"crypto/sha256"
	"encoding/hex"
)

// DocumentVersion represents a stored version of a document.
type DocumentVersion struct {
	Version     int
	DocumentID  string
	ContentHash string
	SimHash     uint64
	WordCount   int
}

// ChangeDecision indicates whether a freshly extracted document is new, modified, or unchanged.
type ChangeDecision int

const (
	DecisionUnchanged ChangeDecision = iota
	DecisionNew
	DecisionModified
)

// DetectChange compares a newly extracted document with the previously stored version.
// If both ContentHash (SHA-256) and SimHash are unchanged, indexing is skipped.
func DetectChange(prev *DocumentVersion, newBody string, newSimHash uint64) (ChangeDecision, string) {
	h := sha256.Sum256([]byte(newBody))
	newHash := hex.EncodeToString(h[:])

	if prev == nil {
		return DecisionNew, newHash
	}

	if prev.ContentHash == newHash && prev.SimHash == newSimHash {
		return DecisionUnchanged, newHash
	}

	return DecisionModified, newHash
}
