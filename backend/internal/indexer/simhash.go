package indexer

import (
	"fmt"
	"hash/fnv"
	"math/bits"
	"strings"
	"unicode"
)

// SimHash computes a 64-bit locality sensitive hash of text using word/character n-grams.
func SimHash(text string) uint64 {
	tokens := tokenizeForSimHash(text)
	if len(tokens) == 0 {
		return 0
	}

	var v [64]int

	// Count token weights (frequency-based)
	freqs := make(map[string]int)
	for _, tok := range tokens {
		freqs[tok]++
	}

	for tok, weight := range freqs {
		h := hashToken(tok)
		for i := 0; i < 64; i++ {
			bit := (h >> i) & 1
			if bit == 1 {
				v[i] += weight
			} else {
				v[i] -= weight
			}
		}
	}

	var fingerprint uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			fingerprint |= (1 << i)
		}
	}

	return fingerprint
}

// HammingDistance calculates the bitwise difference between two 64-bit SimHashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// IsNearDuplicate returns true if the Hamming distance between two SimHashes is <= threshold.
// Standard threshold for near-duplicate documents is 3 bits out of 64.
func IsNearDuplicate(a, b uint64, threshold int) bool {
	if threshold <= 0 {
		threshold = 3
	}
	return HammingDistance(a, b) <= threshold
}

// FormatSimHash formats a 64-bit SimHash as a 16-character hexadecimal string.
func FormatSimHash(h uint64) string {
	return fmt.Sprintf("%016x", h)
}

// ParseSimHash parses a 16-character hex string into a uint64 SimHash.
func ParseSimHash(s string) (uint64, error) {
	var val uint64
	_, err := fmt.Sscanf(s, "%016x", &val)
	return val, err
}

func hashToken(token string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(token))
	return h.Sum64()
}

func tokenizeForSimHash(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	if len(words) == 0 {
		return nil
	}

	var tokens []string
	tokens = append(tokens, words...)

	// Character 3-grams across words for robust subword/morphological similarity
	for _, word := range words {
		runes := []rune(word)
		if len(runes) >= 3 {
			for i := 0; i <= len(runes)-3; i++ {
				tokens = append(tokens, string(runes[i:i+3]))
			}
		}
	}

	// Word bi-grams for phrase context
	for i := 0; i < len(words)-1; i++ {
		tokens = append(tokens, words[i]+"_"+words[i+1])
	}

	return tokens
}

