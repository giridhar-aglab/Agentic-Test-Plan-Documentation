// Package ledger records money movements.
package ledger

import "errors"

// ErrInsufficientFunds is returned when a debit exceeds the balance.
var ErrInsufficientFunds = errors.New("insufficient funds")

// Account is a single balance in minor units.
type Account struct {
	ID           string
	BalanceMinor int64
	Currency     string
	Frozen       bool
}

// Entry is one posted movement.
type Entry struct {
	AccountID   string
	AmountMinor int64
	Reference   string
}

// Post applies an entry to an account, returning the updated account.
//
// This is the riskiest function in the service: it branches on currency,
// frozen state, sign, and overflow, and every branch is a way to lose money.
func Post(account Account, entry Entry, currency string) (Account, error) {
	if account.ID != entry.AccountID {
		return account, errors.New("entry does not belong to account")
	}
	if account.Frozen {
		return account, errors.New("account is frozen")
	}
	if currency != "" && account.Currency != currency {
		return account, errors.New("currency mismatch")
	}
	if entry.AmountMinor == 0 {
		return account, errors.New("zero amount")
	}
	if entry.AmountMinor < 0 && account.BalanceMinor+entry.AmountMinor < 0 {
		return account, ErrInsufficientFunds
	}
	if entry.AmountMinor > 0 && account.BalanceMinor > 0 &&
		account.BalanceMinor+entry.AmountMinor < account.BalanceMinor {
		return account, errors.New("balance overflow")
	}
	account.BalanceMinor += entry.AmountMinor
	return account, nil
}

// Balance returns the current balance.
func Balance(account Account) int64 {
	return account.BalanceMinor
}
