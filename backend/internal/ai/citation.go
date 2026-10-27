package ai

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CitationAnchor represents a verified link to an indexed source document.
type CitationAnchor struct {
	Index       int       `json:"index"`
	DocumentID  string    `json:"document_id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Publisher   string    `json:"publisher"`
	PublishedAt time.Time `json:"published_at"`
	Snippet     string    `json:"snippet"`
}

// GroundedSentence represents a single sentence in the AI synthesis with associated citations.
type GroundedSentence struct {
	Sentence      string           `json:"sentence"`
	CitationIDs   []int            `json:"citation_ids"`
	GroundingRatio float64         `json:"grounding_ratio"` // Lexical overlap ratio with source passages
	IsGrounded    bool             `json:"is_grounded"`
}

// GroundingReport contains the audit evaluation of AI response faithfulness.
type GroundingReport struct {
	TotalSentences    int                `json:"total_sentences"`
	GroundedSentences int                `json:"grounded_sentences"`
	OverallFaithScore float64            `json:"overall_faith_score"` // 0.0 to 1.0
	IsFaithful        bool               `json:"is_faithful"`         // True if OverallFaithScore >= 0.80
	Sentences         []GroundedSentence `json:"sentences"`
	Citations         []CitationAnchor   `json:"citations"`
}

var citationRegex = regexp.MustCompile(`\[(\d+)\]`)

// ExtractCitationIndices extracts all numeric indices referenced like "[1]", "[2]" from text.
func ExtractCitationIndices(text string) []int {
	matches := citationRegex.FindAllStringSubmatch(text, -1)
	seen := make(map[int]bool)
	var indices []int

	for _, m := range matches {
		if len(m) > 1 {
			if idx, err := strconv.Atoi(m[1]); err == nil && !seen[idx] {
				indices = append(indices, idx)
				seen[idx] = true
			}
		}
	}
	return indices
}

// ComputeLexicalOverlap calculates the fraction of words in the claim that appear in the reference passage.
func ComputeLexicalOverlap(claim, passage string) float64 {
	claimWords := strings.Fields(strings.ToLower(claim))
	if len(claimWords) == 0 {
		return 0.0
	}

	passageLow := strings.ToLower(passage)
	matched := 0
	for _, w := range claimWords {
		// Clean punctuation
		cleaned := strings.Trim(w, `.,!?"'()[]{}<>-~`)
		if len(cleaned) < 2 {
			continue
		}
		if strings.Contains(passageLow, cleaned) {
			matched++
		}
	}

	return float64(matched) / float64(len(claimWords))
}

// VerifyGrounding evaluates whether each sentence in the AI synthesis is grounded in the retrieved citations.
func VerifyGrounding(text string, citations []CitationAnchor) GroundingReport {
	// Split by sentence terminators
	rawSentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n'
	})

	citationMap := make(map[int]CitationAnchor)
	for _, c := range citations {
		citationMap[c.Index] = c
	}

	var sentences []GroundedSentence
	groundedCount := 0

	for _, raw := range rawSentences {
		s := strings.TrimSpace(raw)
		if len(s) < 5 {
			continue
		}

		citedIndices := ExtractCitationIndices(s)
		maxOverlap := 0.0

		for _, idx := range citedIndices {
			if anchor, ok := citationMap[idx]; ok {
				overlap := ComputeLexicalOverlap(s, anchor.Snippet)
				if overlap > maxOverlap {
					maxOverlap = overlap
				}
			}
		}

		// A sentence is considered grounded if it has at least 0.50 lexical overlap with its cited source
		isGrounded := len(citedIndices) > 0 && maxOverlap >= 0.50
		if isGrounded {
			groundedCount++
		}

		sentences = append(sentences, GroundedSentence{
			Sentence:       s,
			CitationIDs:    citedIndices,
			GroundingRatio: maxOverlap,
			IsGrounded:     isGrounded,
		})
	}

	faithScore := 0.0
	if len(sentences) > 0 {
		faithScore = float64(groundedCount) / float64(len(sentences))
	}

	return GroundingReport{
		TotalSentences:    len(sentences),
		GroundedSentences: groundedCount,
		OverallFaithScore: faithScore,
		IsFaithful:        faithScore >= 0.80, // Minimum 80% threshold required
		Sentences:         sentences,
		Citations:         citations,
	}
}

// FormatCitationFootnote creates human-readable markdown citations.
func FormatCitationFootnote(c CitationAnchor) string {
	dateStr := c.PublishedAt.Format("2006-01-02")
	return fmt.Sprintf("[%d] [%s](%s) — %s (%s)", c.Index, c.Title, c.URL, c.Publisher, dateStr)
}
