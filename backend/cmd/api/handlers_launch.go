package main

import (
	"net/http"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/launch"
)

func (a *app) handleLaunchGateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	_, p0, p1, _, _, _ := a.ensureDefectRegister().EvaluateReleaseReadiness()
	hasBreach, _ := a.ensureSLOTracker().CheckBreaches()
	sloHealthy := !hasBreach
	// A-3: report the measured rehearsal state. Never simulate a passing drill.
	drPassed := a.ensureDRManager().HasPassingRehearsal(30 * 24 * time.Hour)

	report := launch.EvaluateMasterReleaseGate(
		false, // Phases 0-4: no external audit evidence on file (A-3)
		false, // Phase 5 contracts pending
		false, // Phase 6 LLM SLA pending
		false, // Phase 7 financial audit pending
		false, // Phase 8 security audit pending
		drPassed,
		sloHealthy,
		false, // Canary soak test pending
		p0,
		p1,
	)
	writeJSON(w, 200, report)
}

func (a *app) handleLaunchBenchmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	res := a.ensureBenchProfiler().CalculateResult(60 * time.Second)
	writeJSON(w, 200, res)
}

func (a *app) handleLaunchDRDrill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		DrillName    string `json:"drill_name"`
		TargetRTO    int    `json:"target_rto"`
		ActualRTO    int    `json:"actual_rto"`
		TargetRPO    int    `json:"target_rpo"`
		ActualRPO    int    `json:"actual_rpo"`
		SnapshotData string `json:"snapshot_data"`
		VerifiedBy   string `json:"verified_by"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	drill, err := a.ensureDRManager().RecordDrill(
		body.DrillName,
		body.TargetRTO,
		body.ActualRTO,
		body.TargetRPO,
		body.ActualRPO,
		[]byte(body.SnapshotData),
		body.VerifiedBy,
	)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, drill)
}

func (a *app) handleLaunchDefects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pass, p0, p1, p2, p3, blockers := a.ensureDefectRegister().EvaluateReleaseReadiness()
		defects := a.ensureDefectRegister().ListDefects()
		writeJSON(w, 200, map[string]any{
			"pass":     pass,
			"p0_count": p0,
			"p1_count": p1,
			"p2_count": p2,
			"p3_count": p3,
			"blockers": blockers,
			"defects":  defects,
		})
		return
	}
	if r.Method == http.MethodPost {
		var body struct {
			Severity   string `json:"severity"`
			Title      string `json:"title"`
			Domain     string `json:"domain"`
			Mitigation string `json:"mitigation"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
			return
		}
		rec, err := a.ensureDefectRegister().RegisterDefect(launch.DefectSeverity(body.Severity), body.Title, body.Domain, body.Mitigation)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, rec)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
