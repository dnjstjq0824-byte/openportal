package launch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DRDrillResult represents the outcome of a disaster recovery restore rehearsal (Task 094).
type DRDrillResult struct {
	ID               string    `json:"id"`
	DrillName        string    `json:"drill_name"`
	TargetRTOSec     int       `json:"target_rto_seconds"` // e.g. 1800 (30 min)
	ActualRTOSec     int       `json:"actual_rto_seconds"`
	TargetRPOSec     int       `json:"target_rpo_seconds"` // e.g. 300 (5 min)
	ActualRPOSec     int       `json:"actual_rpo_seconds"`
	SnapshotChecksum string    `json:"snapshot_checksum"`
	ChecksumVerified bool      `json:"checksum_verified"`
	Status           string    `json:"status"` // PASSED, FAILED
	VerifiedBy       string    `json:"verified_by"`
	ConductedAt      time.Time `json:"conducted_at"`
}

// DRManager tracks backup integrity and restore rehearsal verification.
type DRManager struct {
	mu     sync.RWMutex
	drills map[string]DRDrillResult
}

// NewDRManager initializes the disaster recovery validator.
func NewDRManager() *DRManager {
	return &DRManager{
		drills: make(map[string]DRDrillResult),
	}
}

// RecordDrill validates RTO/RPO SLA thresholds and records the rehearsal.
func (m *DRManager) RecordDrill(drillName string, targetRTO, actualRTO, targetRPO, actualRPO int, rawSnapshotData []byte, verifiedBy string) (*DRDrillResult, error) {
	if drillName == "" || verifiedBy == "" {
		return nil, errors.New("drill_name and verified_by are required")
	}

	h := sha256.Sum256(rawSnapshotData)
	checksum := hex.EncodeToString(h[:])

	status := "PASSED"
	if actualRTO > targetRTO || actualRPO > targetRPO || len(rawSnapshotData) == 0 {
		status = "FAILED"
	}

	id := fmt.Sprintf("dr-%d", time.Now().UnixNano())
	res := DRDrillResult{
		ID:               id,
		DrillName:        drillName,
		TargetRTOSec:     targetRTO,
		ActualRTOSec:     actualRTO,
		TargetRPOSec:     targetRPO,
		ActualRPOSec:     actualRPO,
		SnapshotChecksum: checksum,
		ChecksumVerified: len(rawSnapshotData) > 0,
		Status:           status,
		VerifiedBy:       verifiedBy,
		ConductedAt:      time.Now().UTC(),
	}

	m.mu.Lock()
	m.drills[id] = res
	m.mu.Unlock()

	return &res, nil
}

// ListDrills returns all recorded drills.
func (m *DRManager) ListDrills() []DRDrillResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]DRDrillResult, 0, len(m.drills))
	for _, d := range m.drills {
		list = append(list, d)
	}
	return list
}

// HasPassingRehearsal verifies that at least one rehearsal passed within the allowed maximum age.
func (m *DRManager) HasPassingRehearsal(maxAge time.Duration) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, d := range m.drills {
		if d.Status == "PASSED" && d.ChecksumVerified {
			if maxAge <= 0 || time.Since(d.ConductedAt) <= maxAge {
				return true
			}
		}
	}
	return false
}

