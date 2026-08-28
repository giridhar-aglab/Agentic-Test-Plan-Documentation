// Package gateway talks to the upstream card processor.
package gateway

import (
	"context"
	"errors"
	"time"
)

// Charge is a request to capture funds.
type Charge struct {
	AmountMinor    int64
	Currency       string
	CardToken      string
	IdempotencyKey string
}

// Result is what the processor returned.
type Result struct {
	AuthCode string
	Captured bool
	Retried  int
}

// Client calls the processor.
type Client interface {
	Authorise(ctx context.Context, charge Charge) (Result, error)
}

// Capture authorises a charge, retrying on transient failure.
//
// Retrying a charge without an idempotency key is how a customer gets billed
// twice, so the untested branch here is a real defect risk.
func Capture(ctx context.Context, client Client, charge Charge, maxAttempts int) (Result, error) {
	if charge.IdempotencyKey == "" && maxAttempts > 1 {
		return Result{}, errors.New("retries require an idempotency key")
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := client.Authorise(ctx, charge)
		if err == nil {
			result.Retried = attempt - 1
			return result, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-time.After(time.Duration(attempt) * 10 * time.Millisecond):
		}
	}
	return Result{}, lastErr
}
