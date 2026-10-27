package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// DocumentChunk represents a semantic passage extracted from an indexed document.
type DocumentChunk struct {
	ChunkID        string `json:"chunk_id"`
	DocumentID     string `json:"document_id"`
	SourceURL      string `json:"source_url"`
	ChunkIndex     int    `json:"chunk_index"`
	HeadingContext string `json:"heading_context"`
	Content        string `json:"content"`
	WordCount      int    `json:"word_count"`
	Hash           string `json:"hash"`
}

// EmbeddingSpec defines the configuration of the embedding model.
type EmbeddingSpec struct {
	ModelName string `json:"model_name"`
	Dimension int    `json:"dimension"`
	Version   string `json:"version"`
}

// DefaultEmbeddingSpec returns standard enterprise embedding configurations.
func DefaultEmbeddingSpec() EmbeddingSpec {
	return EmbeddingSpec{
		ModelName: "text-embedding-3-small",
		Dimension: 1536,
		Version:   "v1.2026",
	}
}

// ChunkDocument splits text into overlapping word windows.
func ChunkDocument(docID, sourceURL, heading, text string, maxWords, overlapWords int) []DocumentChunk {
	if maxWords <= 0 {
		maxWords = 100
	}
	if overlapWords < 0 || overlapWords >= maxWords {
		overlapWords = maxWords / 4
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []DocumentChunk
	step := maxWords - overlapWords
	chunkIdx := 0

	for i := 0; i < len(words); i += step {
		end := i + maxWords
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		content := strings.Join(chunkWords, " ")

		h := sha256.Sum256([]byte(content))
		chunkHash := hex.EncodeToString(h[:16])
		chunkID := fmt.Sprintf("%s-c%03d", docID, chunkIdx)

		chunks = append(chunks, DocumentChunk{
			ChunkID:        chunkID,
			DocumentID:     docID,
			SourceURL:      sourceURL,
			ChunkIndex:     chunkIdx,
			HeadingContext: heading,
			Content:        content,
			WordCount:      len(chunkWords),
			Hash:           chunkHash,
		})

		chunkIdx++
		if end == len(words) {
			break
		}
	}

	return chunks
}
