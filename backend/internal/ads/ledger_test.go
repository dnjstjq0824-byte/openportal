package ads

import (
	"testing"
)

func TestAdBillingLedgerAndDoubleEntry(t *testing.T) {
	ledger := NewAdBillingLedger()
	acc := "acc-advertiser-1"

	// 1. Initial balance 0
	if bal := ledger.GetBalance(acc); bal != 0 {
		t.Errorf("expected initial balance 0, got %f", bal)
	}

	// 2. Deposit 100,000 KRW
	txDep, err := ledger.Deposit(acc, 100000, "KRW", "dep-ref-1")
	if err != nil {
		t.Fatalf("failed to deposit: %v", err)
	}
	if txDep.BalanceAfter != 100000 {
		t.Errorf("expected balance 100000, got %f", txDep.BalanceAfter)
	}

	// 3. Charge click 500 KRW
	txChg, err := ledger.ChargeClick(acc, 500, "KRW", "click-001")
	if err != nil {
		t.Fatalf("failed to charge click: %v", err)
	}
	if txChg.BalanceAfter != 99500 {
		t.Errorf("expected balance 99500, got %f", txChg.BalanceAfter)
	}

	// 4. Overdraft charge should fail
	_, err = ledger.ChargeClick(acc, 200000, "KRW", "click-huge")
	if err == nil {
		t.Errorf("expected overdraft charge to fail, got nil")
	}
}

func TestPartnerAPIKeyScopes(t *testing.T) {
	ledger := NewAdBillingLedger()

	ledger.RegisterPartnerKey("partner-1", "key-read-only", []string{"ads:read", "reports:read"}, 5)

	// Has ads:read
	if !ledger.VerifyPartnerScope("key-read-only", "ads:read") {
		t.Errorf("expected key to have ads:read scope")
	}

	// Does NOT have ads:write
	if ledger.VerifyPartnerScope("key-read-only", "ads:write") {
		t.Errorf("expected key not to have ads:write scope")
	}
}

