package ranking

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

type SafeSearchLevel string

const (
	SafeSearchStrict   SafeSearchLevel = "strict"
	SafeSearchModerate SafeSearchLevel = "moderate"
	SafeSearchOff      SafeSearchLevel = "off"
)

type ReportCategory string

const (
	ReportAdult      ReportCategory = "adult"
	ReportGambling   ReportCategory = "gambling"
	ReportMalware    ReportCategory = "malware"
	ReportCopyright  ReportCategory = "copyright"
	ReportDefamation ReportCategory = "defamation"
	ReportSpam       ReportCategory = "spam"
)

type CaseStatus string

const (
	CasePending   CaseStatus = "pending"
	CaseActioned  CaseStatus = "actioned"
	CaseDismissed CaseStatus = "dismissed"
)

type AppealStatus string

const (
	AppealPending  AppealStatus = "pending"
	AppealApproved AppealStatus = "approved"
	AppealRejected AppealStatus = "rejected"
)

type ContentSafetyClassification struct {
	IsAdult       bool    `json:"is_adult"`
	IsGambling    bool    `json:"is_gambling"`
	IsMalware     bool    `json:"is_malware"`
	IsViolentHate bool    `json:"is_violent_hate"`
	AdultScore    float64 `json:"adult_score"`    // 0.0 ~ 1.0
	GamblingScore float64 `json:"gambling_score"` // 0.0 ~ 1.0
	MalwareScore  float64 `json:"malware_score"`  // 0.0 ~ 1.0
}

type SafeSearchDocument struct {
	DocumentID  string                      `json:"document_id"`
	URL         string                      `json:"url"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Safety      ContentSafetyClassification `json:"safety"`
}

type ModerationCase struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`
	Reporter   string         `json:"reporter"`
	Category   ReportCategory `json:"category"`
	Details    string         `json:"details"`
	Status     CaseStatus     `json:"status"`
	ActionTaken string        `json:"action_taken,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type AppealRecord struct {
	ID          string       `json:"id"`
	CaseID      string       `json:"case_id"`
	Appellant   string       `json:"appellant"`
	Reason      string       `json:"reason"`
	Status      AppealStatus `json:"status"`
	Reviewer    string       `json:"reviewer,omitempty"`
	ReviewNotes string       `json:"review_notes,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	ResolvedAt  *time.Time   `json:"resolved_at,omitempty"`
}

type SafeSearchClassifier struct {
	adultKeywords    []string
	gamblingKeywords []string
	malwareKeywords  []string
}

func NewSafeSearchClassifier() *SafeSearchClassifier {
	return &SafeSearchClassifier{
		adultKeywords: []string{
			"성인인증", "야동", "음란물", "에로", "섹스", "포르노", "adult", "porn", "xxx", "nsfw", "erotic",
		},
		gamblingKeywords: []string{
			"카지노", "바카라", "토토사이트", "불법도박", "슬롯머신", "환전보장", "casino", "baccarat", "gambling",
		},
		malwareKeywords: []string{
			"크랙다운", "랜섬웨어", "악성코드", "해킹툴", "트로이목마", "keygen", "warez", "trojan", "malware",
		},
	}
}

// ClassifyContent inspects text and URLs for safety categories.
func (c *SafeSearchClassifier) ClassifyContent(title, description, url string) ContentSafetyClassification {
	full := strings.ToLower(title + " " + description + " " + url)

	var safety ContentSafetyClassification

	adultHits := 0
	for _, kw := range c.adultKeywords {
		if strings.Contains(full, kw) {
			adultHits++
		}
	}
	if adultHits > 0 {
		safety.IsAdult = true
		safety.AdultScore = 0.5 + float64(adultHits)*0.25
		if safety.AdultScore > 1.0 {
			safety.AdultScore = 1.0
		}
	}

	gamblingHits := 0
	for _, kw := range c.gamblingKeywords {
		if strings.Contains(full, kw) {
			gamblingHits++
		}
	}
	if gamblingHits > 0 {
		safety.IsGambling = true
		safety.GamblingScore = 0.6 + float64(gamblingHits)*0.2
		if safety.GamblingScore > 1.0 {
			safety.GamblingScore = 1.0
		}
	}

	malwareHits := 0
	for _, kw := range c.malwareKeywords {
		if strings.Contains(full, kw) {
			malwareHits++
		}
	}
	if malwareHits > 0 {
		safety.IsMalware = true
		safety.MalwareScore = 0.7 + float64(malwareHits)*0.2
		if safety.MalwareScore > 1.0 {
			safety.MalwareScore = 1.0
		}
	}

	return safety
}

// FilterResults filters documents based on the chosen SafeSearch policy.
func (c *SafeSearchClassifier) FilterResults(docs []SafeSearchDocument, level SafeSearchLevel) []SafeSearchDocument {
	if level == "" {
		level = SafeSearchModerate
	}

	filtered := make([]SafeSearchDocument, 0, len(docs))
	for _, doc := range docs {
		switch level {
		case SafeSearchStrict:
			// Strict drops all adult, gambling, or malware content
			if doc.Safety.IsAdult || doc.Safety.IsGambling || doc.Safety.IsMalware {
				continue
			}
		case SafeSearchModerate:
			// Moderate blocks high-confidence adult (score >= 0.7), gambling, and malware
			if (doc.Safety.IsAdult && doc.Safety.AdultScore >= 0.7) || doc.Safety.IsGambling || doc.Safety.IsMalware {
				continue
			}
		case SafeSearchOff:
			// Off still blocks severe malware to protect users
			if doc.Safety.IsMalware && doc.Safety.MalwareScore >= 0.9 {
				continue
			}
		}
		filtered = append(filtered, doc)
	}
	return filtered
}

// ModerationWorkflowManager handles complaints, takedowns, and owner appeals.
type ModerationWorkflowManager struct {
	mu      sync.RWMutex
	cases   map[string]*ModerationCase
	appeals map[string]*AppealRecord
}

func NewModerationWorkflowManager() *ModerationWorkflowManager {
	return &ModerationWorkflowManager{
		cases:   make(map[string]*ModerationCase),
		appeals: make(map[string]*AppealRecord),
	}
}

func genID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

// SubmitReport creates a moderation case from user/webmaster report.
func (m *ModerationWorkflowManager) SubmitReport(docID, reporter string, cat ReportCategory, details string) (*ModerationCase, error) {
	if strings.TrimSpace(docID) == "" {
		return nil, errors.New("document_id required")
	}
	if strings.TrimSpace(reporter) == "" {
		return nil, errors.New("reporter required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	c := &ModerationCase{
		ID:         genID("case"),
		DocumentID: docID,
		Reporter:   reporter,
		Category:   cat,
		Details:    details,
		Status:     CasePending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.cases[c.ID] = c
	return c, nil
}

// ReviewCase updates status and action taken on a reported case.
func (m *ModerationWorkflowManager) ReviewCase(caseID, action string, dismiss bool) (*ModerationCase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.cases[caseID]
	if !ok {
		return nil, errors.New("case not found")
	}
	if dismiss {
		c.Status = CaseDismissed
		c.ActionTaken = "dismissed: " + action
	} else {
		c.Status = CaseActioned
		c.ActionTaken = action
	}
	c.UpdatedAt = time.Now()
	return c, nil
}

// SubmitAppeal allows content owners to appeal moderation actions.
func (m *ModerationWorkflowManager) SubmitAppeal(caseID, appellant, reason string) (*AppealRecord, error) {
	if strings.TrimSpace(caseID) == "" || strings.TrimSpace(reason) == "" {
		return nil, errors.New("case_id and reason are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.cases[caseID]; !ok {
		return nil, errors.New("associated case not found")
	}

	appeal := &AppealRecord{
		ID:        genID("apl"),
		CaseID:    caseID,
		Appellant: appellant,
		Reason:    reason,
		Status:    AppealPending,
		CreatedAt: time.Now(),
	}
	m.appeals[appeal.ID] = appeal
	return appeal, nil
}

// ResolveAppeal records decision of independent reviewer.
func (m *ModerationWorkflowManager) ResolveAppeal(appealID, reviewer string, approved bool, notes string) (*AppealRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	appeal, ok := m.appeals[appealID]
	if !ok {
		return nil, errors.New("appeal not found")
	}

	now := time.Now()
	appeal.Reviewer = reviewer
	appeal.ReviewNotes = notes
	appeal.ResolvedAt = &now
	if approved {
		appeal.Status = AppealApproved
		if c, cOk := m.cases[appeal.CaseID]; cOk {
			c.Status = CaseDismissed
			c.ActionTaken = "restored via approved appeal: " + notes
			c.UpdatedAt = now
		}
	} else {
		appeal.Status = AppealRejected
	}
	return appeal, nil
}

