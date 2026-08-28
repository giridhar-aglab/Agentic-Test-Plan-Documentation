package tool

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Policy is the failure handling applied uniformly to every tool, so no
// individual tool implements its own retry or timeout logic.
type Policy struct {
	Timeout          time.Duration
	MaxAttempts      int
	BaseBackoff      time.Duration
	MaxBackoff       time.Duration
	BreakerThreshold int
	BreakerCooldown  time.Duration
	// Sleep is injectable so tests exercise backoff without waiting.
	Sleep func(ctx context.Context, duration time.Duration) error
	// Jitter returns a fraction in [0,1); injectable for deterministic tests.
	Jitter func() float64
}

// Policy presets, chosen by how far the call has to travel.
func LocalPolicy() Policy {
	return Policy{
		Timeout: 2 * time.Second, MaxAttempts: 1,
		BaseBackoff: 20 * time.Millisecond, MaxBackoff: 200 * time.Millisecond,
		BreakerThreshold: 0,
	}
}

func NetworkPolicy() Policy {
	return Policy{
		Timeout: 30 * time.Second, MaxAttempts: 3,
		BaseBackoff: 250 * time.Millisecond, MaxBackoff: 8 * time.Second,
		BreakerThreshold: 4, BreakerCooldown: 30 * time.Second,
	}
}

func ModelPolicy() Policy {
	return Policy{
		Timeout: 120 * time.Second, MaxAttempts: 3,
		BaseBackoff: 500 * time.Millisecond, MaxBackoff: 16 * time.Second,
		BreakerThreshold: 5, BreakerCooldown: time.Minute,
	}
}

// WritePolicy governs the non-idempotent write path. One attempt, no retry:
// a retry after an ambiguous timeout is how a re-run floods a project with
// duplicate tickets.
func WritePolicy() Policy {
	return Policy{Timeout: 60 * time.Second, MaxAttempts: 1, BreakerThreshold: 0}
}

func (policy Policy) withDefaults() Policy {
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	if policy.Timeout <= 0 {
		policy.Timeout = 30 * time.Second
	}
	if policy.BaseBackoff <= 0 {
		policy.BaseBackoff = 200 * time.Millisecond
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = 8 * time.Second
	}
	if policy.Sleep == nil {
		policy.Sleep = sleepWithContext
	}
	if policy.Jitter == nil {
		policy.Jitter = rand.Float64
	}
	return policy
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// breakerState tracks consecutive failures for one tool.
type breakerState struct {
	consecutiveFailures int
	openedAt            time.Time
}

// ErrBreakerOpen is returned when a tool has been taken out of service. The
// registry also drops the tool from the schema shown to the model, so the model
// stops being offered a capability that cannot work.
var ErrBreakerOpen = errors.New("tool: circuit breaker open")

// guarded wraps one tool with its policy and breaker.
type guarded struct {
	inner  Tool
	policy Policy

	mutex   sync.Mutex
	breaker breakerState
	now     func() time.Time
}

func (guardedTool *guarded) Name() string                 { return guardedTool.inner.Name() }
func (guardedTool *guarded) Description() string          { return guardedTool.inner.Description() }
func (guardedTool *guarded) InputSchema() json.RawMessage { return guardedTool.inner.InputSchema() }
func (guardedTool *guarded) Idempotent() bool             { return guardedTool.inner.Idempotent() }

func (guardedTool *guarded) breakerOpen() bool {
	if guardedTool.policy.BreakerThreshold <= 0 {
		return false
	}
	guardedTool.mutex.Lock()
	defer guardedTool.mutex.Unlock()
	if guardedTool.breaker.consecutiveFailures < guardedTool.policy.BreakerThreshold {
		return false
	}
	// Half-open probe: after the cooldown, let exactly one call through.
	if guardedTool.now().Sub(guardedTool.breaker.openedAt) >= guardedTool.policy.BreakerCooldown {
		guardedTool.breaker.consecutiveFailures = guardedTool.policy.BreakerThreshold - 1
		return false
	}
	return true
}

func (guardedTool *guarded) recordOutcome(succeeded bool) {
	if guardedTool.policy.BreakerThreshold <= 0 {
		return
	}
	guardedTool.mutex.Lock()
	defer guardedTool.mutex.Unlock()
	if succeeded {
		guardedTool.breaker.consecutiveFailures = 0
		return
	}
	guardedTool.breaker.consecutiveFailures++
	if guardedTool.breaker.consecutiveFailures == guardedTool.policy.BreakerThreshold {
		guardedTool.breaker.openedAt = guardedTool.now()
	}
}

func (guardedTool *guarded) Invoke(ctx context.Context, arguments json.RawMessage) (Result, error) {
	if guardedTool.breakerOpen() {
		return Result{}, NewFailure(FailureDegradable, guardedTool.Name(),
			"this capability is temporarily unavailable; continue without it", ErrBreakerOpen)
	}

	policy := guardedTool.policy
	// A non-idempotent tool is never retried regardless of what the policy says.
	maxAttempts := policy.MaxAttempts
	if !guardedTool.inner.Idempotent() {
		maxAttempts = 1
	}

	var lastErr error
	for attemptNumber := 1; attemptNumber <= maxAttempts; attemptNumber++ {
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, policy.Timeout)
		result, err := guardedTool.inner.Invoke(attemptCtx, arguments)
		cancelAttempt()

		if err == nil {
			guardedTool.recordOutcome(true)
			return result, nil
		}
		lastErr = err

		// A correctable failure is the model's problem, not the transport's.
		// It must not consume retries or trip the breaker.
		if ClassOf(err) == FailureCorrectable {
			return Result{}, err
		}
		guardedTool.recordOutcome(false)

		if ClassOf(err) != FailureRetryable || attemptNumber == maxAttempts {
			break
		}
		if sleepErr := policy.Sleep(ctx, guardedTool.backoffFor(attemptNumber, err)); sleepErr != nil {
			return Result{}, sleepErr
		}
	}
	return Result{}, lastErr
}

// backoffFor returns exponential backoff with full jitter, honouring a
// server-supplied Retry-After when one is present rather than guessing.
func (guardedTool *guarded) backoffFor(attemptNumber int, err error) time.Duration {
	var failure *Failure
	if errors.As(err, &failure) && failure.RetryAfterSeconds > 0 {
		return time.Duration(failure.RetryAfterSeconds) * time.Second
	}
	policy := guardedTool.policy
	backoff := policy.BaseBackoff << (attemptNumber - 1)
	if backoff > policy.MaxBackoff || backoff <= 0 {
		backoff = policy.MaxBackoff
	}
	return time.Duration(float64(backoff) * policy.Jitter())
}

// Fallback chains two tools under one name: when the primary degrades, the
// secondary answers. Declared per capability, so a backend outage costs
// fidelity rather than the whole run.
type Fallback struct {
	Primary   Tool
	Secondary Tool
	// OnFallback is called when the secondary is used, so the run can record
	// reduced confidence against whatever the secondary produced.
	OnFallback func(toolName string, cause error)
}

func (fallback *Fallback) Name() string                 { return fallback.Primary.Name() }
func (fallback *Fallback) Description() string          { return fallback.Primary.Description() }
func (fallback *Fallback) InputSchema() json.RawMessage { return fallback.Primary.InputSchema() }
func (fallback *Fallback) Idempotent() bool             { return fallback.Primary.Idempotent() }

func (fallback *Fallback) Invoke(ctx context.Context, arguments json.RawMessage) (Result, error) {
	result, err := fallback.Primary.Invoke(ctx, arguments)
	if err == nil {
		return result, nil
	}
	// Only a degraded or retryable primary earns a fallback. A correctable
	// error means the arguments were wrong, and they will be just as wrong for
	// the secondary.
	failureClass := ClassOf(err)
	if failureClass == FailureCorrectable || failureClass == FailureFatal {
		return Result{}, err
	}
	if fallback.OnFallback != nil {
		fallback.OnFallback(fallback.Name(), err)
	}
	fallbackResult, fallbackErr := fallback.Secondary.Invoke(ctx, arguments)
	if fallbackErr != nil {
		return Result{}, fallbackErr
	}
	if fallbackResult.Confidence == 0 || fallbackResult.Confidence > 0.7 {
		fallbackResult.Confidence = 0.7
	}
	return fallbackResult, nil
}
