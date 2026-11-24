package ranking

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func sampleDef(id string) ExperimentDef {
	return ExperimentDef{
		ID: "exp-" + id, Name: "Experiment " + id, Layer: "ranking", Salt: "s1",
		SplitPercent: 50, Status: ExpStatusRunning,
		ControlProfile: "bm25", TreatmentProfile: "bm25-rrf",
		MaxErrorRateThreshold: 0.05, MaxP99LatencyThreshold: 500.0, MinSampleSize: 20,
	}
}

func TestMemoryStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	def := sampleDef("mem")
	if err := store.SaveExperiment(ctx, def); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadExperiment(ctx, def.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != def {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
	paused := sampleDef("paused")
	paused.Status = ExpStatusPaused
	if err := store.SaveExperiment(ctx, paused); err != nil {
		t.Fatal(err)
	}
	running, err := store.LoadRunning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 1 || running[0].ID != def.ID {
		t.Fatalf("running = %v", running)
	}
	if _, err := store.LoadExperiment(ctx, "missing"); err != ErrExperimentNotFound {
		t.Fatalf("missing error = %v", err)
	}
}

func TestValidateExperimentDef(t *testing.T) {
	good := sampleDef("ok")
	if err := ValidateExperimentDef(good); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.ID = ""
	if err := ValidateExperimentDef(bad); err == nil {
		t.Fatal("expected ID error")
	}
	bad = good
	bad.SplitPercent = 101
	if err := ValidateExperimentDef(bad); err == nil {
		t.Fatal("expected split error")
	}
	bad = good
	bad.Status = "bogus"
	if err := ValidateExperimentDef(bad); err == nil {
		t.Fatal("expected status error")
	}
	if err := NewMemoryStore().SaveExperiment(context.Background(), bad); err == nil {
		t.Fatal("expected store to reject invalid def")
	}
}

func TestPlatformPersistAndRestoreWithMemoryStore(t *testing.T) {
	ctx := context.Background()
	platform := NewExperimentPlatform()
	platform.AttachStore(NewMemoryStore())
	def := sampleDef("persist")
	if err := platform.RegisterExperiment(def); err != nil {
		t.Fatal(err)
	}
	if err := platform.PersistExperiment(ctx, def.ID); err != nil {
		t.Fatal(err)
	}
	fresh := NewExperimentPlatform()
	// Share the same store to simulate a restart against one database.
	fresh.AttachStore(storeOf(t, platform))
	if err := fresh.RestoreRunning(ctx); err != nil {
		t.Fatal(err)
	}
	variant, profile, err := fresh.AssignDeterministic(def.ID, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if variant != VariantControl && variant != VariantTreatment {
		t.Fatalf("variant = %q", variant)
	}
	if profile == "" {
		t.Fatal("empty profile after restore")
	}
}

func storeOf(t *testing.T, p *ExperimentPlatform) ExperimentStore {
	t.Helper()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.store == nil {
		t.Fatal("no store attached")
	}
	return p.store
}

// liveStore opens a migrated database or skips the caller. Setup: apply
// migrations 001..019 (node scripts/migrate.mjs) then run with
// TEST_DATABASE_URL set. Skipped otherwise (unit coverage stays in memory).
func liveStore(t *testing.T) (*PostgresStore, *sql.DB, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL unset; live PostgresStore test skipped")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	var hasCols int
	err = db.QueryRowContext(ctx, `select count(*) from information_schema.columns
    where table_schema='query' and table_name='experiments'
    and column_name in ('layer','salt','split_percent','control_profile','treatment_profile',
    'max_error_rate_threshold','max_p99_latency_threshold','min_sample_size',
    'rollback_reason','rolled_back_at')`).Scan(&hasCols)
	if err != nil || hasCols != 10 {
		t.Skipf("migration 019 not applied (columns=%d, err=%v)", hasCols, err)
	}
	return NewPostgresStore(db), db, ctx
}

// TestPostgresStoreLive exercises PostgresStore against a real database.
func TestPostgresStoreLive(t *testing.T) {
	store, db, ctx := liveStore(t)
	name := "live-exp"
	_, _ = db.ExecContext(ctx, "delete from query.experiments where name = $1", name)
	def := sampleDef("live")
	def.ID = name
	if err := store.SaveExperiment(ctx, def); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadExperiment(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SplitPercent != 50 || loaded.ControlProfile != "bm25" || loaded.Status != ExpStatusRunning {
		t.Fatalf("loaded = %+v", loaded)
	}
	running, err := store.LoadRunning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, exp := range running {
		if exp.ID == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("running set missing %q", name)
	}
	// The 006 seed row backfilled by 019 must load with its own values.
	seeded, err := store.LoadExperiment(ctx, "ranking_bm25_vs_rrf")
	if err != nil {
		t.Fatal(err)
	}
	if seeded.Layer != "ranking" || seeded.SplitPercent != 50 || seeded.ControlProfile != "bm25_baseline_v1" {
		t.Fatalf("seeded row = %+v", seeded)
	}
	def.Status = ExpStatusPaused
	if err := store.SaveExperiment(ctx, def); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.LoadExperiment(ctx, name)
	if err != nil || loaded.Status != ExpStatusPaused {
		t.Fatalf("upsert = %+v, err = %v", loaded, err)
	}
	if _, err := store.LoadExperiment(ctx, "does-not-exist"); err != ErrExperimentNotFound {
		t.Fatalf("missing error = %v", err)
	}
	if _, err := db.ExecContext(ctx, "delete from query.experiments where name = $1", name); err != nil {
		t.Fatal(err)
	}
}

// TestPlatformRestartRestoreLive simulates a restart: definitions persisted by
// one platform instance are restored by a fresh instance from the database,
// including the layer lock.
func TestPlatformRestartRestoreLive(t *testing.T) {
	store, db, ctx := liveStore(t)
	name := "live-restart"
	if _, err := db.ExecContext(ctx, "delete from query.experiments where name = $1", name); err != nil {
		t.Fatal(err)
	}
	def := sampleDef("restart")
	def.ID = name
	before := NewExperimentPlatform()
	before.AttachStore(store)
	if err := before.RegisterExperiment(def); err != nil {
		t.Fatal(err)
	}
	if err := before.PersistExperiment(ctx, name); err != nil {
		t.Fatal(err)
	}
	// Fresh instance, same database: restart restores the running definition.
	after := NewExperimentPlatform()
	after.AttachStore(store)
	if err := after.RestoreRunning(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := after.AssignDeterministic(name, "user-7"); err != nil {
		t.Fatalf("assignment after restore: %v", err)
	}
	// The restored layer lock still excludes a second running experiment.
	clash := sampleDef("clash")
	clash.Layer = def.Layer
	clash.Status = ExpStatusRunning
	if err := after.RegisterExperiment(clash); err == nil {
		t.Fatal("expected layer mutual-exclusion after restore")
	}
	if _, err := db.ExecContext(ctx, "delete from query.experiments where name = $1", name); err != nil {
		t.Fatal(err)
	}
}

// TestPlatformMultiInstanceLive simulates two instances sharing one store: an
// experiment persisted by instance A becomes visible to instance B on restore.
func TestPlatformMultiInstanceLive(t *testing.T) {
	store, db, ctx := liveStore(t)
	name := "live-multi"
	if _, err := db.ExecContext(ctx, "delete from query.experiments where name = $1", name); err != nil {
		t.Fatal(err)
	}
	def := sampleDef("multi")
	def.ID = name
	def.Layer = "ranking-multi"
	instanceA := NewExperimentPlatform()
	instanceA.AttachStore(store)
	if err := instanceA.RegisterExperiment(def); err != nil {
		t.Fatal(err)
	}
	if err := instanceA.PersistExperiment(ctx, name); err != nil {
		t.Fatal(err)
	}
	instanceB := NewExperimentPlatform()
	instanceB.AttachStore(store)
	if err := instanceB.RestoreRunning(ctx); err != nil {
		t.Fatal(err)
	}
	variant, _, err := instanceB.AssignDeterministic(name, "user-9")
	if err != nil {
		t.Fatalf("instance B assignment: %v", err)
	}
	same, _, err := instanceA.AssignDeterministic(name, "user-9")
	if err != nil || same != variant {
		t.Fatalf("instances disagree: A=%q B=%q err=%v", same, variant, err)
	}
	if _, err := db.ExecContext(ctx, "delete from query.experiments where name = $1", name); err != nil {
		t.Fatal(err)
	}
}
