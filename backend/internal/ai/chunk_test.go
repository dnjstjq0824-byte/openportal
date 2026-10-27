package ai

import (
	"strings"
	"testing"
)

func TestChunkDocument(t *testing.T) {
	// 25 words text
	text := "One two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty twenty-one twenty-two twenty-three twenty-four twenty-five"
	chunks := ChunkDocument("doc-1", "https://openportal.org", "Architecture", text, 10, 2)

	if len(chunks) == 0 {
		t.Fatalf("expected chunks, got 0")
	}

	// First chunk should have 10 words
	if chunks[0].WordCount != 10 {
		t.Errorf("expected 10 words in first chunk, got %d", chunks[0].WordCount)
	}

	// Verify overlap: end of chunk 0 should overlap with beginning of chunk 1
	c0Words := strings.Fields(chunks[0].Content)
	c1Words := strings.Fields(chunks[1].Content)

	if c0Words[8] != c1Words[0] || c0Words[9] != c1Words[1] {
		t.Errorf("overlap mismatch between chunk 0 and 1: c0 tail=%v, c1 head=%v", c0Words[8:], c1Words[:2])
	}
}
