// Experiment definition persistence (049, G020 follow-up).
//
// ExperimentPlatform stays the in-memory source of truth for assignment, but
// definitions survive restarts via ExperimentStore: PostgresStore for real
// deployments (query.experiments as extended by migration 019) and
// MemoryStore for unit tests and nil-DB fallback. The platform experiment ID
// maps to the unique query.experiments.name column.
package ranking

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

// ErrExperimentNotFound is returned when no stored definition matches.
var ErrExperimentNotFound = errors.New("experiment not found")

// ExperimentStore persists experiment definitions across restarts.
type ExperimentStore interface {
	SaveExperiment(ctx context.Context, exp ExperimentDef) error
	LoadExperiment(ctx context.Context, name string) (ExperimentDef, error)
	LoadRunning(ctx context.Context) ([]ExperimentDef, error)
}

// ValidateExperimentDef enforces the storage contract shared by all stores.
func ValidateExperimentDef(exp ExperimentDef) error {
	if len(exp.ID) < 1 || len(exp.ID) > 128 {
		return errors.New("experiment ID must be 1..128 characters")
	}
	if strings.TrimSpace(exp.Layer) == "" {
		return errors.New("experiment layer is required")
	}
	if exp.SplitPercent < 0 || exp.SplitPercent > 100 {
		return errors.New("split_percent must be 0..100")
	}
	switch exp.Status {
	case ExpStatusDraft, ExpStatusRunning, ExpStatusPaused, ExpStatusRolledBack, ExpStatusConcluded:
	default:
		return errors.New("unknown experiment status")
	}
	return nil
}

// MemoryStore is a mutex-guarded in-memory ExperimentStore for unit tests and
// nil-DB fallback. Contents do not survive restarts by design.
type MemoryStore struct {
	mu   sync.RWMutex
	defs map[string]ExperimentDef
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{defs: make(map[string]ExperimentDef)}
}

// SaveExperiment inserts or replaces a definition by ID.
func (s *MemoryStore) SaveExperiment(_ context.Context, exp ExperimentDef) error {
	if err := ValidateExperimentDef(exp); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defs[exp.ID] = exp
	return nil
}

// LoadExperiment returns the definition for an ID or ErrExperimentNotFound.
func (s *MemoryStore) LoadExperiment(_ context.Context, name string) (ExperimentDef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.defs[name]
	if !ok {
		return ExperimentDef{}, ErrExperimentNotFound
	}
	return exp, nil
}

// LoadRunning returns all definitions with status running, ordered by ID.
func (s *MemoryStore) LoadRunning(_ context.Context) ([]ExperimentDef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ExperimentDef
	for _, exp := range s.defs {
		if exp.Status == ExpStatusRunning {
			out = append(out, exp)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID > out[j].ID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out, nil
}

// PostgresStore persists experiment definitions in query.experiments.
// It requires migration 019 (049 definition columns).
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore returns a PostgresStore over db (must be non-nil).
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

// SaveExperiment upserts a definition keyed by name (platform experiment ID).
func (s *PostgresStore) SaveExperiment(ctx context.Context, exp ExperimentDef) error {
	if err := ValidateExperimentDef(exp); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO query.experiments
  (name, status, layer, salt, split_percent, control_profile, treatment_profile,
   max_error_rate_threshold, max_p99_latency_threshold, min_sample_size,
   rollback_reason, rolled_back_at, started_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now())
ON CONFLICT (name) DO UPDATE SET
  status = EXCLUDED.status, layer = EXCLUDED.layer, salt = EXCLUDED.salt,
  split_percent = EXCLUDED.split_percent, control_profile = EXCLUDED.control_profile,
  treatment_profile = EXCLUDED.treatment_profile,
  max_error_rate_threshold = EXCLUDED.max_error_rate_threshold,
  max_p99_latency_threshold = EXCLUDED.max_p99_latency_threshold,
  min_sample_size = EXCLUDED.min_sample_size, rollback_reason = EXCLUDED.rollback_reason,
  rolled_back_at = EXCLUDED.rolled_back_at`,
		exp.ID, string(exp.Status), nullString(exp.Layer), nullString(exp.Salt), exp.SplitPercent,
		nullString(exp.ControlProfile), nullString(exp.TreatmentProfile),
		exp.MaxErrorRateThreshold, exp.MaxP99LatencyThreshold, exp.MinSampleSize,
		nullString(exp.RollbackReason), nullTime(exp.RolledBackAt))
	return err
}

func scanExperimentDef(row interface {
	Scan(dest ...any) error
}) (ExperimentDef, error) {
	var exp ExperimentDef
	var status string
	var layer, salt, control, treatment, rollbackReason sql.NullString
	var split sql.NullInt32
	var maxErr, maxP99 sql.NullFloat64
	var minSample sql.NullInt32
	var rolledBackAt sql.NullTime
	err := row.Scan(&exp.ID, &status, &layer, &salt, &split,
		&control, &treatment, &maxErr, &maxP99,
		&minSample, &rollbackReason, &rolledBackAt)
	if err != nil {
		return ExperimentDef{}, err
	}
	exp.Status = ExperimentStatus(status)
	exp.Layer = layer.String
	exp.Salt = salt.String
	exp.SplitPercent = int(split.Int32)
	exp.ControlProfile = control.String
	exp.TreatmentProfile = treatment.String
	exp.MaxErrorRateThreshold = maxErr.Float64
	exp.MaxP99LatencyThreshold = maxP99.Float64
	exp.MinSampleSize = int(minSample.Int32)
	exp.RollbackReason = rollbackReason.String
	if rolledBackAt.Valid {
		at := rolledBackAt.Time
		exp.RolledBackAt = &at
	}
	return exp, nil
}

const experimentColumns = `name, status, layer, salt, split_percent, control_profile,
  treatment_profile, max_error_rate_threshold, max_p99_latency_threshold,
  min_sample_size, rollback_reason, rolled_back_at`

// LoadExperiment returns the definition for a name or ErrExperimentNotFound.
func (s *PostgresStore) LoadExperiment(ctx context.Context, name string) (ExperimentDef, error) {
	exp, err := scanExperimentDef(s.db.QueryRowContext(ctx,
		"SELECT "+experimentColumns+" FROM query.experiments WHERE name = $1", name))
	if errors.Is(err, sql.ErrNoRows) {
		return ExperimentDef{}, ErrExperimentNotFound
	}
	return exp, err
}

// LoadRunning returns all running definitions ordered by name.
func (s *PostgresStore) LoadRunning(ctx context.Context) ([]ExperimentDef, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+experimentColumns+" FROM query.experiments WHERE status = 'running' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExperimentDef
	for rows.Next() {
		exp, err := scanExperimentDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, exp)
	}
	return out, rows.Err()
}

// AttachStore sets the persistence backend used by PersistExperiment and
// RestoreRunning. A nil store disables persistence (memory-only platform).
func (p *ExperimentPlatform) AttachStore(store ExperimentStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.store = store
}

// PersistExperiment saves one registered definition to the attached store.
// It is a no-op without a store; callers surface the error (never silent).
func (p *ExperimentPlatform) PersistExperiment(ctx context.Context, expID string) error {
	p.mu.RLock()
	exp, exists := p.experiments[expID]
	store := p.store
	p.mu.RUnlock()
	if !exists {
		return errors.New("experiment not found")
	}
	if store == nil {
		return nil
	}
	return store.SaveExperiment(ctx, *exp)
}

// RestoreRunning loads running definitions from the attached store into
// memory (boot path). Invalid stored rows are skipped with a warning (never
// fail boot). Layer conflicts resolve by keeping the stored order;
// conflicting extras stay paused in memory and must be re-registered.
func (p *ExperimentPlatform) RestoreRunning(ctx context.Context) error {
	p.mu.RLock()
	store := p.store
	p.mu.RUnlock()
	if store == nil {
		return nil
	}
	defs, err := store.LoadRunning(ctx)
	if err != nil {
		return err
	}
	for _, def := range defs {
		if err := ValidateExperimentDef(def); err != nil {
			log.Printf("skipping invalid stored experiment %q: %v", def.ID, err)
			continue
		}
		if err := p.RegisterExperiment(def); err != nil {
			def.Status = ExpStatusPaused
			if regErr := p.RegisterExperiment(def); regErr != nil {
				return regErr
			}
		}
	}
	return nil
}
