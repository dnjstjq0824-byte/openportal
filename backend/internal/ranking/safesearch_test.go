package ranking

import (
	"testing"
)

func TestSafeSearchClassificationAndFiltering(t *testing.T) {
	classifier := NewSafeSearchClassifier()

	docs := []SafeSearchDocument{
		{
			DocumentID:  "doc-clean",
			URL:         "https://example.com/tech-news",
			Title:       "최신 기술 동향 및 개발 블로그",
			Description: "오픈소스 검색엔진 및 아키텍처 가이드",
			Safety:      classifier.ClassifyContent("최신 기술 동향", "가이드", "https://example.com"),
		},
		{
			DocumentID:  "doc-adult-mild",
			URL:         "https://health.example.com/education",
			Title:       "청소년 성교육 가이드라인",
			Description: "성인인증 제도의 역사와 성교육 필요성",
			Safety:      classifier.ClassifyContent("청소년 성교육", "성인인증", "https://health.example.com"),
		},
		{
			DocumentID:  "doc-adult-explicit",
			URL:         "https://adult-xxx.test/porn",
			Title:       "음란물 포르노 성인 야동 사이트",
			Description: "성인인증 필수 섹스 영상",
			Safety:      classifier.ClassifyContent("음란물 포르노 야동", "섹스 영상", "https://adult-xxx.test"),
		},
		{
			DocumentID:  "doc-gambling",
			URL:         "https://casino-bet.test/toto",
			Title:       "불법도박 바카라 카지노 토토사이트",
			Description: "환전보장 슬롯머신 배팅",
			Safety:      classifier.ClassifyContent("불법도박 바카라 카지노 토토사이트", "환전보장 슬롯머신", "https://casino-bet.test"),
		},
		{
			DocumentID:  "doc-malware",
			URL:         "https://warez.test/crack.exe",
			Title:       "상용 소프트웨어 크랙다운 및 키젠",
			Description: "랜섬웨어 트로이목마 해킹툴 다운로드",
			Safety:      classifier.ClassifyContent("크랙다운 키젠", "랜섬웨어 해킹툴", "https://warez.test/crack.exe"),
		},
	}

	// 1. Strict Mode
	strictResults := classifier.FilterResults(docs, SafeSearchStrict)
	for _, d := range strictResults {
		if d.DocumentID != "doc-clean" {
			t.Errorf("Strict mode leaked unsafe document: %s", d.DocumentID)
		}
	}
	if len(strictResults) != 1 {
		t.Errorf("Strict mode expected 1 clean document, got %d", len(strictResults))
	}

	// 2. Moderate Mode
	moderateResults := classifier.FilterResults(docs, SafeSearchModerate)
	// doc-clean is kept. doc-adult-mild (educational, 1 hit, score 0.75 or so) vs explicit
	// Let's verify explicit adult, gambling, and malware are excluded
	for _, d := range moderateResults {
		if d.DocumentID == "doc-adult-explicit" || d.DocumentID == "doc-gambling" || d.DocumentID == "doc-malware" {
			t.Errorf("Moderate mode should filter explicit adult, gambling, malware, got: %s", d.DocumentID)
		}
	}

	// 3. Off Mode
	offResults := classifier.FilterResults(docs, SafeSearchOff)
	// Off mode should allow adult and gambling, but still filter severe malware
	foundAdult := false
	foundGambling := false
	foundMalware := false
	for _, d := range offResults {
		if d.DocumentID == "doc-adult-explicit" {
			foundAdult = true
		}
		if d.DocumentID == "doc-gambling" {
			foundGambling = true
		}
		if d.DocumentID == "doc-malware" {
			foundMalware = true
		}
	}
	if !foundAdult || !foundGambling {
		t.Errorf("Off mode should allow adult (%v) and gambling (%v)", foundAdult, foundGambling)
	}
	if foundMalware {
		t.Errorf("Off mode should still filter dangerous malware")
	}
}

func TestModerationCaseAndAppealWorkflow(t *testing.T) {
	mgr := NewModerationWorkflowManager()

	// 1. Submit report
	c, err := mrgSubmit(mgr, "doc-123", "user@example.test", ReportMalware, "악성코드가 포함된 URL입니다")
	if err != nil {
		t.Fatalf("SubmitReport failed: %v", err)
	}
	if c.Status != CasePending {
		t.Fatalf("expected pending status, got %s", c.Status)
	}

	// 2. Review and action
	reviewed, err := mgr.ReviewCase(c.ID, "takedown: flagged for malware download", false)
	if err != nil {
		t.Fatalf("ReviewCase failed: %v", err)
	}
	if reviewed.Status != CaseActioned {
		t.Fatalf("expected actioned status, got %s", reviewed.Status)
	}

	// 3. Content owner submits appeal
	appeal, err := mgr.SubmitAppeal(c.ID, "webmaster@example.test", "오탐입니다. 합법적인 오픈소스 보안 연구 도구입니다.")
	if err != nil {
		t.Fatalf("SubmitAppeal failed: %v", err)
	}
	if appeal.Status != AppealPending {
		t.Fatalf("expected appeal pending, got %s", appeal.Status)
	}

	// 4. Independent review approves appeal
	resolved, err := mgr.ResolveAppeal(appeal.ID, "compliance_officer_1", true, "보안 연구 사이트로 확인되어 차단 해제")
	if err != nil {
		t.Fatalf("ResolveAppeal failed: %v", err)
	}
	if resolved.Status != AppealApproved {
		t.Fatalf("expected appeal approved, got %s", resolved.Status)
	}
	if resolved.Reviewer != "compliance_officer_1" {
		t.Fatalf("expected reviewer recorded, got %s", resolved.Reviewer)
	}

	// Case status should now be dismissed / restored
	mgr.mu.RLock()
	caseFinal := mgr.cases[c.ID]
	mgr.mu.RUnlock()
	if caseFinal.Status != CaseDismissed {
		t.Fatalf("expected case dismissed after appeal approval, got %s", caseFinal.Status)
	}
}

func mrgSubmit(m *ModerationWorkflowManager, docID, reporter string, cat ReportCategory, details string) (*ModerationCase, error) {
	return m.SubmitReport(docID, reporter, cat, details)
}

