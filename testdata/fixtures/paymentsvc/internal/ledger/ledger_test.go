package ledger

import "testing"

func TestBalance(t *testing.T) {
	if Balance(Account{BalanceMinor: 100}) != 100 {
		t.Fatal("wrong balance")
	}
}
