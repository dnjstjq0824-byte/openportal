package ranking

import (
	"errors"
	"math"
	"sync"
	"time"
)

type JudgmentGrade int

const (
	GradeIrrelevant JudgmentGrade = 0 // 0: 완전히 관련 없음, 스팸, 링크 깨짐
	GradeMarginal   JudgmentGrade = 1 // 1: 부차적 관련성, 불완전하거나 부분적인 정보
	GradeRelevant   JudgmentGrade = 2 // 2: 유용하고 직접적인 답변/정보 제공
	GradeVital      JudgmentGrade = 3 // 3: 핵심 대표 문서, 공식 사이트, 최적의 일치
)

func (g JudgmentGrade) String() string {
	switch g {
	case GradeVital:
		return "Vital(3)"
	case GradeRelevant:
		return "Relevant(2)"
	case GradeMarginal:
		return "Marginal(1)"
	default:
		return "Irrelevant(0)"
	}
}

type JudgmentSubmission struct {
	QueryText  string        `json:"query_text"`
	DocumentID string        `json:"document_id"`
	JudgeID    string        `json:"judge_id"`
	Grade      JudgmentGrade `json:"grade"`
	Notes      string        `json:"notes,omitempty"`
	JudgedAt   time.Time     `json:"judged_at"`
}

type EvaluationItem struct {
	QueryText    string           `json:"query_text"`
	DocumentID   string           `json:"document_id"`
	Title        string           `json:"title"`
	URL          string           `json:"url"`
	Snippets     string           `json:"snippets"`
	Submissions  []JudgmentSubmission `json:"submissions"`
	FinalGrade   *JudgmentGrade   `json:"final_grade,omitempty"`
	NeedsArbitration bool         `json:"needs_arbitration"`
}

type BlindJudgingWorkflow struct {
	mu    sync.RWMutex
	items map[string]*EvaluationItem // key: query::doc_id
}

func NewBlindJudgingWorkflow() *BlindJudgingWorkflow {
	return &BlindJudgingWorkflow{
		items: make(map[string]*EvaluationItem),
	}
}

func itemKey(query, docID string) string {
	return query + "::" + docID
}

// RegisterItem registers a blind evaluation item without disclosing ranking source.
func (w *BlindJudgingWorkflow) RegisterItem(query, docID, title, url, snippet string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := itemKey(query, docID)
	if _, ok := w.items[key]; !ok {
		w.items[key] = &EvaluationItem{
			QueryText:  query,
			DocumentID: docID,
			Title:      title,
			URL:        url,
			Snippets:   snippet,
		}
	}
}

// SubmitBlindJudgment records an independent evaluation from a certified judge.
func (w *BlindJudgingWorkflow) SubmitBlindJudgment(sub JudgmentSubmission) (*EvaluationItem, error) {
	if sub.Grade < 0 || sub.Grade > 3 {
		return nil, errors.New("grade must be between 0 (Irrelevant) and 3 (Vital)")
	}
	if sub.JudgeID == "" {
		return nil, errors.New("judge_id required")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	key := itemKey(sub.QueryText, sub.DocumentID)
	item, ok := w.items[key]
	if !ok {
		item = &EvaluationItem{
			QueryText:  sub.QueryText,
			DocumentID: sub.DocumentID,
		}
		w.items[key] = item
	}

	// Ensure judge hasn't already submitted for this item
	for _, s := range item.Submissions {
		if s.JudgeID == sub.JudgeID {
			return nil, errors.New("judge already submitted for this item")
		}
	}

	if sub.JudgedAt.IsZero() {
		sub.JudgedAt = time.Now()
	}
	item.Submissions = append(item.Submissions, sub)

	// When 2 or more judges have submitted, check agreement
	if len(item.Submissions) == 2 {
		g1 := item.Submissions[0].Grade
		g2 := item.Submissions[1].Grade
		diff := int(math.Abs(float64(g1 - g2)))
		if diff == 0 {
			finalG := g1
			item.FinalGrade = &finalG
			item.NeedsArbitration = false
		} else if diff == 1 {
			// Accept conservative minimum or round down average
			finalG := JudgmentGrade(int(math.Min(float64(g1), float64(g2))))
			item.FinalGrade = &finalG
			item.NeedsArbitration = false
		} else {
			// Severe disagreement (diff >= 2) requires arbitration
			item.NeedsArbitration = true
		}
	}

	return item, nil
}

// Arbitrate resolves severe disagreement by senior adjudicator.
func (w *BlindJudgingWorkflow) Arbitrate(query, docID, arbiterID string, grade JudgmentGrade) (*EvaluationItem, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := itemKey(query, docID)
	item, ok := w.items[key]
	if !ok {
		return nil, errors.New("item not found")
	}
	item.FinalGrade = &grade
	item.NeedsArbitration = false
	item.Submissions = append(item.Submissions, JudgmentSubmission{
		QueryText:  query,
		DocumentID: docID,
		JudgeID:    "arbiter:" + arbiterID,
		Grade:      grade,
		JudgedAt:   time.Now(),
	})
	return item, nil
}

// CalculateCohensKappa computes Cohen's Kappa inter-annotator agreement between two judges.
// Pairs: slice of [gradeJudgeA, gradeJudgeB]
func CalculateCohensKappa(pairs [][2]JudgmentGrade) float64 {
	n := len(pairs)
	if n == 0 {
		return 1.0
	}

	// 4 categories: 0, 1, 2, 3
	matrix := [4][4]int{}
	observedAgree := 0

	for _, p := range pairs {
		gA, gB := int(p[0]), int(p[1])
		if gA >= 0 && gA < 4 && gB >= 0 && gB < 4 {
			matrix[gA][gB]++
			if gA == gB {
				observedAgree++
			}
		}
	}

	po := float64(observedAgree) / float64(n)

	// Chance agreement Pe
	pe := 0.0
	for k := 0; k < 4; k++ {
		rowSum := 0
		colSum := 0
		for c := 0; c < 4; c++ {
			rowSum += matrix[k][c]
			colSum += matrix[c][k]
		}
		pA := float64(rowSum) / float64(n)
		pB := float64(colSum) / float64(n)
		pe += pA * pB
	}

	if math.Abs(1.0-pe) < 1e-9 {
		return 1.0
	}

	return (po - pe) / (1.0 - pe)
}

