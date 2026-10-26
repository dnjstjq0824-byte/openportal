package ranking

import (
	"math"
	"testing"
)

func TestBlindJudgingWorkflow(t *testing.T) {
	wf := NewBlindJudgingWorkflow()

	query := "오픈포털 검색엔진 아키텍처"
	docID := "doc-arch-001"

	wf.RegisterItem(query, docID, "오픈포털 아키텍처 명세서", "https://docs.openportal.org/arch", "분산 크롤링 및 인덱싱 아키텍처 가이드")

	// Judge 1 submits Vital (3)
	it1, err := wf.SubmitBlindJudgment(JudgmentSubmission{
		QueryText:  query,
		DocumentID: docID,
		JudgeID:    "judge-alpha",
		Grade:      GradeVital,
	})
	if err != nil {
		t.Fatalf("Judge 1 submit failed: %v", err)
	}
	if it1.FinalGrade != nil {
		t.Fatalf("expected nil final grade with only 1 submission")
	}

	// Duplicate judge submission rejected
	_, dupErr := wf.SubmitBlindJudgment(JudgmentSubmission{
		QueryText:  query,
		DocumentID: docID,
		JudgeID:    "judge-alpha",
		Grade:      GradeRelevant,
	})
	if dupErr == nil {
		t.Fatalf("expected duplicate judge error")
	}

	// Judge 2 submits Relevant (2) -> diff = 1 -> resolved without arbitration
	it2, err := wf.SubmitBlindJudgment(JudgmentSubmission{
		QueryText:  query,
		DocumentID: docID,
		JudgeID:    "judge-beta",
		Grade:      GradeRelevant,
	})
	if err != nil {
		t.Fatalf("Judge 2 submit failed: %v", err)
	}
	if it2.NeedsArbitration {
		t.Fatalf("diff of 1 should not require arbitration")
	}
	if it2.FinalGrade == nil || *it2.FinalGrade != GradeRelevant {
		t.Fatalf("expected conservative resolved grade Relevant(2), got %v", it2.FinalGrade)
	}

	// Test severe disagreement (Vital 3 vs Irrelevant 0)
	docDisagree := "doc-disagree-002"
	wf.RegisterItem(query, docDisagree, "모호한 블로그 글", "https://blog.test/post", "내용 일부")

	wf.SubmitBlindJudgment(JudgmentSubmission{QueryText: query, DocumentID: docDisagree, JudgeID: "judge-alpha", Grade: GradeVital})
	itDis, _ := wf.SubmitBlindJudgment(JudgmentSubmission{QueryText: query, DocumentID: docDisagree, JudgeID: "judge-beta", Grade: GradeIrrelevant})

	if !itDis.NeedsArbitration {
		t.Fatalf("diff of 3 must require arbitration")
	}

	// Senior Arbiter resolves to Marginal (1)
	resolved, err := wf.Arbitrate(query, docDisagree, "chief_evaluator", GradeMarginal)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if resolved.NeedsArbitration {
		t.Fatalf("expected arbitration resolved")
	}
	if *resolved.FinalGrade != GradeMarginal {
		t.Fatalf("expected final grade Marginal(1), got %v", *resolved.FinalGrade)
	}
}

func TestCalculateCohensKappa(t *testing.T) {
	// High agreement dataset
	pairsHigh := [][2]JudgmentGrade{
		{GradeVital, GradeVital},
		{GradeRelevant, GradeRelevant},
		{GradeMarginal, GradeMarginal},
		{GradeIrrelevant, GradeIrrelevant},
		{GradeVital, GradeVital},
		{GradeRelevant, GradeRelevant},
		{GradeRelevant, GradeMarginal},
		{GradeIrrelevant, GradeIrrelevant},
	}
	kappaHigh := CalculateCohensKappa(pairsHigh)
	if kappaHigh < 0.70 {
		t.Errorf("expected high agreement kappa > 0.70, got %f", kappaHigh)
	}

	// Complete agreement
	pairsPerfect := [][2]JudgmentGrade{
		{GradeVital, GradeVital},
		{GradeRelevant, GradeRelevant},
		{GradeMarginal, GradeMarginal},
	}
	kappaPerfect := CalculateCohensKappa(pairsPerfect)
	if math.Abs(kappaPerfect-1.0) > 1e-4 {
		t.Errorf("expected perfect kappa 1.0, got %f", kappaPerfect)
	}
}

