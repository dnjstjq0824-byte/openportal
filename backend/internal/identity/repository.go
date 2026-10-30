package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrDatabaseOp      = errors.New("database operation failed")
)

// DBExecutor generalizes *sql.DB and *sql.Tx.
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// SQLRepository implements persistence for user preferences, sessions, and recovery codes
// backed by PostgreSQL with Row Level Security (RLS) tenant isolation.
type SQLRepository struct {
	db DBExecutor
}

func NewSQLRepository(db DBExecutor) *SQLRepository {
	return &SQLRepository{db: db}
}

// SetTenantContext binds the current session's account ID to PostgreSQL session settings
// so that RLS policies (e.g. preferences_self_policy) enforce tenant isolation.
func (r *SQLRepository) SetTenantContext(ctx context.Context, accountID string) error {
	_, err := r.db.ExecContext(ctx, "SELECT set_config('app.current_account_id', $1, false)", accountID)
	if err != nil {
		return fmt.Errorf("%w: failed to set tenant RLS context: %v", ErrDatabaseOp, err)
	}
	return nil
}

// GetPreferences fetches an account's preferences from the identity.preferences table.
func (r *SQLRepository) GetPreferences(ctx context.Context, accountID string) (*Preferences, error) {
	if accountID == "" {
		return nil, ErrInvalidInput
	}

	query := `
		SELECT safe_search, theme, language, personalized_search,
		       recent_searches_opt_in, search_correction_opt_in, updated_at
		FROM identity.preferences
		WHERE account_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, accountID)

	var p Preferences
	p.AccountID = accountID
	err := row.Scan(
		&p.SafeSearch,
		&p.Theme,
		&p.Language,
		&p.PersonalizedSearch,
		&p.RecentSearchesOptIn,
		&p.SearchCorrectionOptIn,
		&p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseOp, err)
	}
	return &p, nil
}

// SavePreferences inserts or updates preferences for an account.
func (r *SQLRepository) SavePreferences(ctx context.Context, p *Preferences) error {
	if p == nil || p.AccountID == "" {
		return ErrInvalidInput
	}
	if p.SafeSearch == "" {
		p.SafeSearch = "moderate"
	}
	if p.Theme == "" {
		p.Theme = "system"
	}
	if p.Language == "" {
		p.Language = "ko"
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now()
	}

	query := `
		INSERT INTO identity.preferences (
			account_id, safe_search, theme, language,
			personalized_search, recent_searches_opt_in, search_correction_opt_in, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (account_id) DO UPDATE SET
			safe_search = EXCLUDED.safe_search,
			theme = EXCLUDED.theme,
			language = EXCLUDED.language,
			personalized_search = EXCLUDED.personalized_search,
			recent_searches_opt_in = EXCLUDED.recent_searches_opt_in,
			search_correction_opt_in = EXCLUDED.search_correction_opt_in,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		p.AccountID,
		p.SafeSearch,
		p.Theme,
		p.Language,
		p.PersonalizedSearch,
		p.RecentSearchesOptIn,
		p.SearchCorrectionOptIn,
		p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("%w: failed to save preferences: %v", ErrDatabaseOp, err)
	}
	return nil
}

// SaveRecentSearch records an opt-in query history entry.
func (r *SQLRepository) SaveRecentSearch(ctx context.Context, accountID, queryText, locale string) error {
	if accountID == "" || queryText == "" {
		return ErrInvalidInput
	}
	if locale == "" {
		locale = "ko-KR"
	}

	sqlStr := `
		INSERT INTO identity.recent_searches (account_id, query_normalized, locale, searched_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (account_id, query_normalized)
		DO UPDATE SET searched_at = now()
	`
	_, err := r.db.ExecContext(ctx, sqlStr, accountID, queryText, locale)
	if err != nil {
		return fmt.Errorf("%w: failed to record recent search: %v", ErrDatabaseOp, err)
	}
	return nil
}

// GetRecentSearches retrieves recent search queries for an account up to limit.
func (r *SQLRepository) GetRecentSearches(ctx context.Context, accountID string, limit int) ([]string, error) {
	if accountID == "" {
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	query := `
		SELECT query_normalized
		FROM identity.recent_searches
		WHERE account_id = $1
		ORDER BY searched_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query recent searches: %v", ErrDatabaseOp, err)
	}
	defer rows.Close()

	var queries []string
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}
	return queries, nil
}

// ClearRecentSearches purges query history for an account.
func (r *SQLRepository) ClearRecentSearches(ctx context.Context, accountID string) error {
	if accountID == "" {
		return ErrInvalidInput
	}
	_, err := r.db.ExecContext(ctx, "DELETE FROM identity.recent_searches WHERE account_id = $1", accountID)
	if err != nil {
		return fmt.Errorf("%w: failed to clear recent searches: %v", ErrDatabaseOp, err)
	}
	return nil
}
