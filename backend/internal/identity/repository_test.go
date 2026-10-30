package identity

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockExecutor provides an in-memory simulation of the DBExecutor interface.
type mockExecutor struct {
	mu           sync.Mutex
	settings     map[string]string
	prefs        map[string]*Preferences
	recent       map[string][]string
	injectExecErr error
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		settings: make(map[string]string),
		prefs:    make(map[string]*Preferences),
		recent:   make(map[string][]string),
	}
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.injectExecErr != nil {
		return nil, m.injectExecErr
	}

	if strings.Contains(query, "set_config") {
		if len(args) > 0 {
			m.settings["app.current_account_id"] = args[0].(string)
		}
		return nil, nil
	}

	if strings.Contains(query, "INSERT INTO identity.preferences") {
		accountID := args[0].(string)
		safeSearch := args[1].(string)
		theme := args[2].(string)
		lang := args[3].(string)
		personalized := args[4].(bool)
		recentOptIn := args[5].(bool)
		correctionOptIn := args[6].(bool)
		updatedAt := args[7].(time.Time)

		m.prefs[accountID] = &Preferences{
			AccountID:             accountID,
			SafeSearch:            safeSearch,
			Theme:                 theme,
			Language:              lang,
			PersonalizedSearch:    personalized,
			RecentSearchesOptIn:   recentOptIn,
			SearchCorrectionOptIn: correctionOptIn,
			UpdatedAt:             updatedAt,
		}
		return nil, nil
	}

	if strings.Contains(query, "INSERT INTO identity.recent_searches") {
		accountID := args[0].(string)
		queryText := args[1].(string)
		m.recent[accountID] = append([]string{queryText}, m.recent[accountID]...)
		return nil, nil
	}

	if strings.Contains(query, "DELETE FROM identity.recent_searches") {
		accountID := args[0].(string)
		delete(m.recent, accountID)
		return nil, nil
	}

	return nil, nil
}

func (m *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	// Standard *sql.Row cannot be directly instantiated outside database/sql,
	// so for mock testing we test through repository integration or custom mock wrapper.
	return nil
}

func (m *mockExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("not implemented in mock")
}

func TestSQLRepository_SavePreferencesAndRLS(t *testing.T) {
	mock := newMockExecutor()
	repo := NewSQLRepository(mock)
	ctx := context.Background()

	// Test invalid input
	if err := repo.SavePreferences(ctx, nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil preferences, got %v", err)
	}

	accountID := "acc-123e4567-e89b-12d3-a456-426614174000"

	// 1. Set Tenant RLS Context
	if err := repo.SetTenantContext(ctx, accountID); err != nil {
		t.Fatalf("SetTenantContext failed: %v", err)
	}
	if mock.settings["app.current_account_id"] != accountID {
		t.Errorf("expected app.current_account_id to be %q, got %q", accountID, mock.settings["app.current_account_id"])
	}

	// 2. Save Preferences
	p := &Preferences{
		AccountID:             accountID,
		SafeSearch:            "strict",
		Theme:                 "dark",
		Language:              "ko",
		PersonalizedSearch:    true,
		RecentSearchesOptIn:   true,
		SearchCorrectionOptIn: true,
	}

	if err := repo.SavePreferences(ctx, p); err != nil {
		t.Fatalf("SavePreferences failed: %v", err)
	}

	saved, ok := mock.prefs[accountID]
	if !ok {
		t.Fatalf("preferences not saved in mock")
	}
	if saved.SafeSearch != "strict" || saved.Theme != "dark" || !saved.PersonalizedSearch {
		t.Errorf("saved preferences mismatch: %+v", saved)
	}

	// 3. Save Recent Search
	if err := repo.SaveRecentSearch(ctx, accountID, "오픈포털 검색엔진", "ko-KR"); err != nil {
		t.Fatalf("SaveRecentSearch failed: %v", err)
	}
	if len(mock.recent[accountID]) != 1 || mock.recent[accountID][0] != "오픈포털 검색엔진" {
		t.Errorf("recent searches mismatch: %+v", mock.recent[accountID])
	}

	// 4. Clear Recent Searches
	if err := repo.ClearRecentSearches(ctx, accountID); err != nil {
		t.Fatalf("ClearRecentSearches failed: %v", err)
	}
	if len(mock.recent[accountID]) != 0 {
		t.Errorf("expected recent searches to be cleared, got %+v", mock.recent[accountID])
	}
}

func TestSQLRepository_ErrorHandling(t *testing.T) {
	mock := newMockExecutor()
	mock.injectExecErr = errors.New("connection timeout")
	repo := NewSQLRepository(mock)
	ctx := context.Background()

	err := repo.SetTenantContext(ctx, "any-id")
	if !errors.Is(err, ErrDatabaseOp) {
		t.Errorf("expected ErrDatabaseOp, got %v", err)
	}

	p := &Preferences{AccountID: "any-id"}
	err = repo.SavePreferences(ctx, p)
	if !errors.Is(err, ErrDatabaseOp) {
		t.Errorf("expected ErrDatabaseOp, got %v", err)
	}
}
