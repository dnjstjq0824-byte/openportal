package launch

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// DefectSeverity categorizes defect severity (Task 098).
type DefectSeverity string

const (
	SeverityP0 DefectSeverity = "P0" // Blocker / Data Loss / Security Breach / Crash
	SeverityP1 DefectSeverity = "P1" // Critical / Major Feature Broken / No Workaround
	SeverityP2 DefectSeverity = "P2" // Major / Impaired Functionality with Workaround
	SeverityP3 DefectSeverity = "P3" // Minor / Cosmetic / Minor Inconvenience
)

// DefectRecord represents a single tracked issue.
type DefectRecord struct {
	ID          string         `json:"id"`
	Severity    DefectSeverity `json:"severity"`
	Title       string         `json:"title"`
	Domain      string         `json:"domain"`
	Status      string         `json:"status"` // "OPEN", "MITIGATED", "RESOLVED"
	Mitigation  string         `json:"mitigation,omitempty"`
	ReportedAt  time.Time      `json:"reported_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
}

// DefectRegister manages defect tracking across all functional subsystems.
type DefectRegister struct {
	mu      sync.RWMutex
	defects map[string]DefectRecord
}

// NewDefectRegister initializes the register.
func NewDefectRegister() *DefectRegister {
	return &DefectRegister{
		defects: make(map[string]DefectRecord),
	}
}

// RegisterDefect logs a new issue.
func (r *DefectRegister) RegisterDefect(severity DefectSeverity, title, domain, mitigation string) (*DefectRecord, error) {
	if title == "" || domain == "" {
		return nil, errors.New("title and domain are required")
	}

	id := fmt.Sprintf("def-%d", time.Now().UnixNano())
	rec := DefectRecord{
		ID:         id,
		Severity:   severity,
		Title:      title,
		Domain:     domain,
		Status:     "OPEN",
		Mitigation: mitigation,
		ReportedAt: time.Now().UTC(),
	}

	r.mu.Lock()
	r.defects[id] = rec
	r.mu.Unlock()

	return &rec, nil
}

// ResolveDefect marks a defect as resolved.
func (r *DefectRegister) ResolveDefect(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.defects[id]
	if !ok {
		return errors.New("defect not found: " + id)
	}

	now := time.Now().UTC()
	rec.Status = "RESOLVED"
	rec.ResolvedAt = &now
	r.defects[id] = rec
	return nil
}

// EvaluateReleaseReadiness asserts the zero P0 / zero P1 defect gate.
func (r *DefectRegister) EvaluateReleaseReadiness() (pass bool, p0Count int, p1Count int, p2Count int, p3Count int, blockers []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, d := range r.defects {
		if d.Status == "RESOLVED" {
			continue
		}
		switch d.Severity {
		case SeverityP0:
			p0Count++
			blockers = append(blockers, fmt.Sprintf("[P0-BLOCKER] %s: %s", d.Domain, d.Title))
		case SeverityP1:
			p1Count++
			blockers = append(blockers, fmt.Sprintf("[P1-BLOCKER] %s: %s", d.Domain, d.Title))
		case SeverityP2:
			p2Count++
		case SeverityP3:
			p3Count++
		}
	}

	pass = (p0Count == 0 && p1Count == 0)
	return pass, p0Count, p1Count, p2Count, p3Count, blockers
}

// ListDefects returns all recorded defects.
func (r *DefectRegister) ListDefects() []DefectRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]DefectRecord, 0, len(r.defects))
	for _, d := range r.defects {
		list = append(list, d)
	}
	return list
}

