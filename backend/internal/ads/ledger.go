package ads

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// LedgerEntryType represents debit or credit in the double-entry accounting model.
type LedgerEntryType string

const (
	EntryCredit LedgerEntryType = "credit" // Balance increase (e.g. deposit, refund)
	EntryDebit  LedgerEntryType = "debit"  // Balance decrease (e.g. ad click spend, chargeback)
)

// LedgerTransaction models an accounting entry.
type LedgerTransaction struct {
	ID            string          `json:"id"`
	AccountID     string          `json:"account_id"`
	Type          LedgerEntryType `json:"entry_type"`
	Amount        float64         `json:"amount"`
	Currency      string          `json:"currency"`
	ReferenceType string          `json:"reference_type"` // "deposit", "campaign_charge", "refund", "chargeback"
	ReferenceID   string          `json:"reference_id"`
	BalanceAfter  float64         `json:"balance_after"`
	Timestamp     time.Time       `json:"timestamp"`
}

// PartnerAPIKey models partner credentials and scopes.
type PartnerAPIKey struct {
	KeyID       string    `json:"key_id"`
	PartnerID   string    `json:"partner_id"`
	Scopes      []string  `json:"scopes"` // "ads:read", "ads:write", "reports:read"
	DailyQuota  int       `json:"daily_quota"`
	UsedToday   int       `json:"used_today"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// AdBillingLedger manages balances and double-entry consistency (Tasks 078, 079).
type AdBillingLedger struct {
	mu           sync.RWMutex
	balances     map[string]float64
	transactions map[string][]LedgerTransaction
	partnerKeys  map[string]PartnerAPIKey
}

// NewAdBillingLedger creates a ledger instance.
func NewAdBillingLedger() *AdBillingLedger {
	return &AdBillingLedger{
		balances:     make(map[string]float64),
		transactions: make(map[string][]LedgerTransaction),
		partnerKeys:  make(map[string]PartnerAPIKey),
	}
}

// Deposit credits an advertiser account.
func (l *AdBillingLedger) Deposit(accountID string, amount float64, currency, refID string) (*LedgerTransaction, error) {
	if amount <= 0 {
		return nil, errors.New("deposit amount must be greater than zero")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cur := l.balances[accountID]
	next := cur + amount
	l.balances[accountID] = next

	tx := LedgerTransaction{
		ID:            fmt.Sprintf("tx-dep-%d", time.Now().UnixNano()),
		AccountID:     accountID,
		Type:          EntryCredit,
		Amount:        amount,
		Currency:      currency,
		ReferenceType: "deposit",
		ReferenceID:   refID,
		BalanceAfter:  next,
		Timestamp:     time.Now(),
	}

	l.transactions[accountID] = append(l.transactions[accountID], tx)
	return &tx, nil
}

// ChargeClick debits an advertiser account for a validated ad click.
func (l *AdBillingLedger) ChargeClick(accountID string, amount float64, currency, clickID string) (*LedgerTransaction, error) {
	if amount <= 0 {
		return nil, errors.New("charge amount must be greater than zero")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cur := l.balances[accountID]
	if cur < amount {
		return nil, errors.New("insufficient ad budget balance")
	}

	next := cur - amount
	l.balances[accountID] = next

	tx := LedgerTransaction{
		ID:            fmt.Sprintf("tx-chg-%d", time.Now().UnixNano()),
		AccountID:     accountID,
		Type:          EntryDebit,
		Amount:        amount,
		Currency:      currency,
		ReferenceType: "campaign_charge",
		ReferenceID:   clickID,
		BalanceAfter:  next,
		Timestamp:     time.Now(),
	}

	l.transactions[accountID] = append(l.transactions[accountID], tx)
	return &tx, nil
}

// GetBalance returns the current balance of an account.
func (l *AdBillingLedger) GetBalance(accountID string) float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.balances[accountID]
}

// RegisterPartnerKey provisions a scoped API key for a partner organization (Task 079).
func (l *AdBillingLedger) RegisterPartnerKey(partnerID, keyID string, scopes []string, quota int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.partnerKeys[keyID] = PartnerAPIKey{
		KeyID:      keyID,
		PartnerID:  partnerID,
		Scopes:     scopes,
		DailyQuota: quota,
		UsedToday:  0,
		ExpiresAt:  time.Now().Add(365 * 24 * time.Hour),
	}
}

// VerifyPartnerScope validates that the partner API key has the required permission scope.
func (l *AdBillingLedger) VerifyPartnerScope(keyID, requiredScope string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	pk, exists := l.partnerKeys[keyID]
	if !exists || time.Now().After(pk.ExpiresAt) {
		return false
	}

	if pk.UsedToday >= pk.DailyQuota {
		return false // Quota exceeded
	}

	for _, s := range pk.Scopes {
		if s == requiredScope || s == "admin:all" {
			pk.UsedToday++
			l.partnerKeys[keyID] = pk
			return true
		}
	}
	return false
}
