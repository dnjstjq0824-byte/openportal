package ai

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RAGRequest encapsulates input for AI overview synthesis.
type RAGRequest struct {
	Query      string          `json:"query"`
	Passages   []DocumentChunk `json:"passages"`
	SafeSearch string          `json:"safe_search"`
}

// RAGResponse holds the synthesized overview or safe fallback metadata.
type RAGResponse struct {
	Query             string           `json:"query"`
	Answer            string           `json:"answer,omitempty"`
	Disclaimer        string           `json:"disclaimer,omitempty"`
	Citations         []CitationAnchor `json:"citations,omitempty"`
	Grounding         GroundingReport  `json:"grounding"`
	FallbackTriggered bool             `json:"fallback_triggered"`
	FallbackReason    string           `json:"fallback_reason,omitempty"`
	ExecutionMs       int64            `json:"execution_ms"`
}

// SynthesizeAnswer runs the end-to-end RAG pipeline with strict grounding checks and safe fallback (Task 064, 068).
func SynthesizeAnswer(ctx context.Context, req RAGRequest, budgetLimits ModelBudgetLimits) (*RAGResponse, error) {
	start := time.Now()

	// 1. Guardrail Inspection (Task 061, 066, 067)
	decision := EvaluateInputQuery(req.Query)
	if !decision.Allowed {
		return &RAGResponse{
			Query:             req.Query,
			FallbackTriggered: true,
			FallbackReason:    decision.Reason,
			ExecutionMs:       time.Since(start).Milliseconds(),
		}, nil
	}

	// 2. Candidate passage verification (Must have at least 1 passage)
	if len(req.Passages) == 0 {
		return &RAGResponse{
			Query:             req.Query,
			FallbackTriggered: true,
			FallbackReason:    "no candidate passages available for grounding; falling back to organic web search",
			ExecutionMs:       time.Since(start).Milliseconds(),
		}, nil
	}

	// 3. Build Citation Anchors from retrieved passages
	var citations []CitationAnchor
	var passageTexts []string

	for idx, p := range req.Passages {
		if idx >= 5 { // Maximum 5 source citations
			break
		}
		c := CitationAnchor{
			Index:       idx + 1,
			DocumentID:  p.DocumentID,
			Title:       fmt.Sprintf("Source %d: %s", idx+1, p.HeadingContext),
			URL:         p.SourceURL,
			Publisher:   "OpenPortal Corpus",
			PublishedAt: time.Now(),
			Snippet:     p.Content,
		}
		citations = append(citations, c)
		passageTexts = append(passageTexts, fmt.Sprintf("[%d] %s", c.Index, p.Content))
	}

	// 4. Synthesize structured answer referencing citations
	// In production, this invokes LLM with temperature 0.0 and JSON mode.
	// In deterministic mode, constructs verified grounded summary directly from citations:
	var sb strings.Builder
	for _, c := range citations {
		// Extract first key sentence of each passage and append citation tag
		sentences := strings.Split(c.Snippet, ".")
		if len(sentences) > 0 && strings.TrimSpace(sentences[0]) != "" {
			sb.WriteString(strings.TrimSpace(sentences[0]))
			sb.WriteString(fmt.Sprintf(" [%d]. ", c.Index))
		}
	}
	syntheticAnswer := strings.TrimSpace(sb.String())

	// 5. Verify Grounding & Faithfulness (Task 065)
	grounding := VerifyGrounding(syntheticAnswer, citations)
	if !grounding.IsFaithful {
		return &RAGResponse{
			Query:             req.Query,
			FallbackTriggered: true,
			FallbackReason:    fmt.Sprintf("answer failed grounding threshold (score: %.2f < 0.80); hallucination guard triggered", grounding.OverallFaithScore),
			ExecutionMs:       time.Since(start).Milliseconds(),
		}, nil
	}

	// 6. Enforce budget limits
	estTokens := len(strings.Fields(syntheticAnswer)) * 2
	if err := budgetLimits.ValidateBudget(estTokens, estTokens); err != nil {
		return &RAGResponse{
			Query:             req.Query,
			FallbackTriggered: true,
			FallbackReason:    "model token budget exceeded; falling back to organic web search",
			ExecutionMs:       time.Since(start).Milliseconds(),
		}, nil
	}

	return &RAGResponse{
		Query:             req.Query,
		Answer:            syntheticAnswer,
		Disclaimer:        decision.Disclaimer,
		Citations:         citations,
		Grounding:         grounding,
		FallbackTriggered: false,
		ExecutionMs:       time.Since(start).Milliseconds(),
	}, nil
}
