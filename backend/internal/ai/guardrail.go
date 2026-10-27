package ai

import (
	"errors"
	"fmt"
	"strings"
)

// YMYLCategory represents sensitive "Your Money or Your Life" topics requiring disclaimers.
type YMYLCategory string

const (
	YMYLNone       YMYLCategory = "none"
	YMYLMedical    YMYLCategory = "medical"
	YMYLFinancial  YMYLCategory = "financial"
	YMYLLegal      YMYLCategory = "legal"
	YMYLEmergency  YMYLCategory = "emergency"
)

// GuardrailDecision encapsulates the outcome of input and safety evaluation.
type GuardrailDecision struct {
	Allowed       bool         `json:"allowed"`
	Reason        string       `json:"reason"`
	YMYL          YMYLCategory `json:"ymyl_category"`
	Disclaimer    string       `json:"disclaimer,omitempty"`
	RefusalNotice string       `json:"refusal_notice,omitempty"`
}

var promptInjectionPatterns = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"system prompt",
	"you are now in developer mode",
	"dan mode",
	"jailbreak",
	"bypass security",
	"reveal your prompt",
	"what are your instructions",
	"<script",
	"javascript:",
	"eval(",
}

var medicalKeywords = []string{
	"치료법", "약 복용", "진단", "증상 치료", "암 치료", "응급처치", "의약품", "부작용",
	"symptom diagnosis", "dosage", "cure disease", "prescription",
}

var financialKeywords = []string{
	"주식 매수 추천", "가상화폐 급등", "선물 투자", "목돈 굴리기", "수익률 보장",
	"stock pick", "crypto investment", "guaranteed returns",
}

var legalKeywords = []string{
	"고소장 작성", "형사처벌 기준", "합의금 계산", "소송 승소",
	"legal lawsuit advice", "criminal liability",
}

// EvaluateInputQuery tests user input queries against prompt injection and sensitive domain policies.
func EvaluateInputQuery(q string) GuardrailDecision {
	clean := strings.TrimSpace(q)
	if clean == "" {
		return GuardrailDecision{Allowed: false, Reason: "empty input query"}
	}

	low := strings.ToLower(clean)

	// 1. Prompt Injection Detection (Task 066)
	for _, pattern := range promptInjectionPatterns {
		if strings.Contains(low, pattern) {
			return GuardrailDecision{
				Allowed:       false,
				Reason:        "prompt injection or security bypass attempt detected",
				RefusalNotice: "보안 정책에 따라 시스템 지침을 변경하거나 우회하려는 질의는 처리할 수 없습니다.",
			}
		}
	}

	// 2. Sensitive Domain (YMYL) Inspection (Task 067)
	for _, kw := range medicalKeywords {
		if strings.Contains(low, kw) {
			return GuardrailDecision{
				Allowed:    true,
				YMYL:       YMYLMedical,
				Disclaimer: "의학적 조언이나 진단이 아닙니다. 정확한 진단과 치료는 반드시 전문 의료진과 상담하십시오.",
			}
		}
	}

	for _, kw := range financialKeywords {
		if strings.Contains(low, kw) {
			return GuardrailDecision{
				Allowed:    true,
				YMYL:       YMYLFinancial,
				Disclaimer: "투자 권유나 재정적 조언이 아닙니다. 모든 투자의 책임은 본인에게 있으며 전문가와 상담하세요.",
			}
		}
	}

	for _, kw := range legalKeywords {
		if strings.Contains(low, kw) {
			return GuardrailDecision{
				Allowed:    true,
				YMYL:       YMYLLegal,
				Disclaimer: "법률적 상담이나 대리가 아닙니다. 구체적인 법률 분쟁은 변호사 등 법률 전문가와 상담하십시오.",
			}
		}
	}

	return GuardrailDecision{
		Allowed: true,
		YMYL:    YMYLNone,
	}
}

// ModelBudgetLimits sets hard guardrails for token consumption and API costs (Task 061).
type ModelBudgetLimits struct {
	MaxPromptTokens     int     `json:"max_prompt_tokens"`
	MaxCompletionTokens int     `json:"max_completion_tokens"`
	MaxCostPerQueryUSD  float64 `json:"max_cost_per_query_usd"`
}

// DefaultBudgetLimits returns standard cost and token caps.
func DefaultBudgetLimits() ModelBudgetLimits {
	return ModelBudgetLimits{
		MaxPromptTokens:     4096,
		MaxCompletionTokens: 1024,
		MaxCostPerQueryUSD:  0.010000, // 1 cent per query ceiling
	}
}

// ValidateBudget checks if estimated tokens remain within policy bounds.
func (b ModelBudgetLimits) ValidateBudget(promptTokens, completionTokens int) error {
	if promptTokens > b.MaxPromptTokens {
		return fmt.Errorf("prompt token count %d exceeds maximum allowed limit %d", promptTokens, b.MaxPromptTokens)
	}
	if completionTokens > b.MaxCompletionTokens {
		return fmt.Errorf("completion token count %d exceeds maximum allowed limit %d", completionTokens, b.MaxCompletionTokens)
	}
	// Approximate calculation at $0.002 / 1K tokens
	estCost := float64(promptTokens+completionTokens) * 0.000002
	if estCost > b.MaxCostPerQueryUSD {
		return errors.New("estimated query cost exceeds maximum cost limit")
	}
	return nil
}
