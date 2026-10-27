package launch

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLatencyProfileAndPercentiles(t *testing.T) {
	p := NewLatencyProfile()
	// Record 100 samples from 1ms to 100ms
	for i := 1; i <= 100; i++ {
		p.RecordSample(float64(i), nil)
	}
	// Record 2 failures
	p.RecordSample(0, errors.New("timeout"))
	p.RecordSample(0, errors.New("500 internal server error"))

	res := p.CalculateResult(10 * time.Second)
	if res.TotalRequests != 102 {
		t.Fatalf("expected 102 total requests, got %d", res.TotalRequests)
	}
	if res.Failed != 2 {
		t.Fatalf("expected 2 failures, got %d", res.Failed)
	}
	if res.Successful != 100 {
		t.Fatalf("expected 100 successful, got %d", res.Successful)
	}
	if res.P50Ms < 49.0 || res.P50Ms > 51.0 {
		t.Fatalf("expected P50 around 50ms, got %f", res.P50Ms)
	}
	if res.P95Ms < 94.0 || res.P95Ms > 96.0 {
		t.Fatalf("expected P95 around 95ms, got %f", res.P95Ms)
	}
	if res.P99Ms < 98.0 || res.P99Ms > 100.0 {
		t.Fatalf("expected P99 around 99ms, got %f", res.P99Ms)
	}
	if res.QPS <= 0 {
		t.Fatalf("expected positive QPS, got %f", res.QPS)
	}
}

func TestClusterCapacityModel(t *testing.T) {
	model, err := PlanClusterCapacity(10_000_000, 2048, 1) // 10M documents, 2KB avg size
	if err != nil {
		t.Fatal(err)
	}
	if model.PrimaryShards < 1 {
		t.Fatalf("expected at least 1 primary shard, got %d", model.PrimaryShards)
	}
	if model.TotalStorageGB <= 0 {
		t.Fatalf("expected positive storage GB, got %f", model.TotalStorageGB)
	}
	if model.RecommendedNodeRAM == "" {
		t.Fatal("empty RAM tier recommendation")
	}

	// Invalid parameters
	_, err = PlanClusterCapacity(0, 2048, 1)
	if err == nil {
		t.Fatal("expected error on 0 doc count")
	}
}

func TestCircuitBreakerAndFaultInjection(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	if cb.State() != StateClosed {
		t.Fatalf("expected initially CLOSED, got %s", cb.State())
	}

	failFn := func() error { return errors.New("upstream failure") }
	okFn := func() error { return nil }

	// Trigger 3 failures to trip the circuit breaker
	_ = cb.Execute(failFn)
	_ = cb.Execute(failFn)
	_ = cb.Execute(failFn)

	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN state after 3 failures, got %s", cb.State())
	}

	// Fast-fail
	err := cb.Execute(okFn)
	if err == nil || err.Error() != "circuit breaker is open: request fast-failed" {
		t.Fatalf("expected fast-fail error, got %v", err)
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN state after cooldown, got %s", cb.State())
	}

	// Successful execution closes circuit breaker
	_ = cb.Execute(okFn)
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED state after successful probe, got %s", cb.State())
	}

	// Fault injector test
	fi := NewFaultInjector()
	fi.Configure(true, 5*time.Millisecond, 0.5)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Injected fault test with sample below failRate
	err = fi.Intercept(ctx, 0.2)
	if err == nil {
		t.Fatal("expected injected fault error")
	}
}

func TestDRRehearsalManager(t *testing.T) {
	dr := NewDRManager()

	snapshotBytes := []byte("postgres-full-backup-snapshot-20261027")
	drill, err := dr.RecordDrill("postgres_failover_drill", 1800, 420, 300, 60, snapshotBytes, "secops-operator-1")
	if err != nil {
		t.Fatal(err)
	}

	if drill.Status != "PASSED" {
		t.Fatalf("expected drill PASSED, got %s", drill.Status)
	}
	if !drill.ChecksumVerified {
		t.Fatal("expected verified checksum")
	}

	if !dr.HasPassingRehearsal(1 * time.Hour) {
		t.Fatal("expected passing rehearsal within 1 hour")
	}

	// Record failed drill (RTO target breached)
	failedDrill, err := dr.RecordDrill("slow_restore_drill", 60, 300, 300, 60, snapshotBytes, "secops-operator-2")
	if err != nil {
		t.Fatal(err)
	}
	if failedDrill.Status != "FAILED" {
		t.Fatalf("expected FAILED status when RTO breached, got %s", failedDrill.Status)
	}
}

func TestSLOTrackerAndBudget(t *testing.T) {
	slo := NewSLOTracker()

	// 10,000 requests, 1 error -> 99.99% availability (> 99.9% target, budget 90% remaining)
	avail := slo.EvaluateAvailability(10000, 9999)
	if avail.Status != "HEALTHY" {
		t.Fatalf("expected HEALTHY availability at 99.99%%, got %s", avail.Status)
	}

	// Breach availability -> 99.0%
	breachedAvail := slo.EvaluateAvailability(10000, 9900)
	if breachedAvail.Status != "BREACHED" {
		t.Fatalf("expected BREACHED status, got %s", breachedAvail.Status)
	}

	// Latency evaluations
	latOK := slo.EvaluateLatency(45.0)
	if latOK.Status != "HEALTHY" {
		t.Fatalf("expected HEALTHY latency, got %s", latOK.Status)
	}

	latBreach := slo.EvaluateLatency(250.0)
	if latBreach.Status != "BREACHED" {
		t.Fatalf("expected BREACHED latency, got %s", latBreach.Status)
	}

	hasBreach, alerts := slo.CheckBreaches()
	if !hasBreach || len(alerts) < 2 {
		t.Fatalf("expected breaches reported, got hasBreach=%v, alerts=%v", hasBreach, alerts)
	}
}

func TestComplianceAndAccessibilityAudit(t *testing.T) {
	checklist := AccessibilityChecklist{
		HasKeyboardNavigation: true,
		HasAriaLandmarks:      true,
		HasColorContrast45:    true,
		HasAltTextForImages:   true,
		HasFocusIndicators:    true,
	}
	pass, missing := AuditAccessibility(checklist)
	if !pass || len(missing) > 0 {
		t.Fatalf("expected 100%% accessibility pass, got %v", missing)
	}

	loc := LocalizationAudit{
		SupportedLocales: []string{"ko", "en"},
		DefaultLocale:    "ko",
	}
	if err := loc.ValidateLocale("ko"); err != nil {
		t.Fatal(err)
	}
	if err := loc.ValidateLocale("fr"); err == nil {
		t.Fatal("expected error on unsupported locale fr")
	}

	status := AuditReleaseCompliance(checklist, loc, true, true)
	if !status.AccessibilityPassed || !status.LocalizationPassed || !status.PrivacyPolicyNotice || !status.TermsOfServiceNotice {
		t.Fatalf("expected full compliance pass, got %+v", status)
	}
}

func TestDefectRegisterGating(t *testing.T) {
	reg := NewDefectRegister()

	// Register P3 minor defect
	_, err := reg.RegisterDefect(SeverityP3, "Minor footer alignment on 320px", "web", "Adjust CSS padding")
	if err != nil {
		t.Fatal(err)
	}

	pass, p0, p1, _, p3, blockers := reg.EvaluateReleaseReadiness()
	if !pass || p0 != 0 || p1 != 0 || p3 != 1 || len(blockers) > 0 {
		t.Fatalf("expected pass with 0 P0/P1, got pass=%v, p0=%d, p1=%d, blockers=%v", pass, p0, p1, blockers)
	}

	// Register P0 blocker
	p0Rec, err := reg.RegisterDefect(SeverityP0, "Unchecked database crash on bulk ingest", "indexer", "Add payload limit")
	if err != nil {
		t.Fatal(err)
	}

	pass, p0, p1, _, _, blockers = reg.EvaluateReleaseReadiness()
	if pass || p0 != 1 || len(blockers) != 1 {
		t.Fatalf("expected blocked release with 1 P0, got pass=%v, p0=%d, blockers=%v", pass, p0, blockers)
	}

	// Resolve the P0 blocker
	if err := reg.ResolveDefect(p0Rec.ID); err != nil {
		t.Fatal(err)
	}

	pass, p0, p1, _, _, blockers = reg.EvaluateReleaseReadiness()
	if !pass || p0 != 0 || p1 != 0 {
		t.Fatalf("expected pass after P0 resolution, got pass=%v, p0=%d", pass, p0)
	}
}

func TestMasterReleaseGate(t *testing.T) {
	// Case 1: All downstream audits incomplete -> Overall: CONDITIONAL_CANARY, Public: false
	rep := EvaluateMasterReleaseGate(false, false, false, false, true, true, false, 0, 0)
	if rep.OverallDecision != "CONDITIONAL_CANARY" {
		t.Fatalf("expected CONDITIONAL_CANARY, got %s", rep.OverallDecision)
	}
	if rep.PublicLaunchReady {
		t.Fatal("public launch should not be ready without external audits")
	}
	if rep.PhasesBlocked < 4 {
		t.Fatalf("expected at least 4 blocked phases, got %d", rep.PhasesBlocked)
	}

	// Case 2: P0 defect present -> Overall: NO-GO, Canary: false
	repWithP0 := EvaluateMasterReleaseGate(false, false, false, false, true, true, false, 1, 0)
	if repWithP0.OverallDecision != "NO-GO" || repWithP0.CanaryReady {
		t.Fatalf("expected NO-GO when P0 exists, got %s (canary=%v)", repWithP0.OverallDecision, repWithP0.CanaryReady)
	}

	// Case 3: All external audits and contracts signed, soak passed, 0 P0/P1 -> Overall: GO, Public: true
	repFullGo := EvaluateMasterReleaseGate(true, true, true, true, true, true, true, 0, 0)
	if repFullGo.OverallDecision != "GO" || !repFullGo.PublicLaunchReady {
		t.Fatalf("expected GO when all audits passed, got %s", repFullGo.OverallDecision)
	}
}
