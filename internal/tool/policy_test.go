package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// flakyTool fails a set number of times before succeeding, so retry and
// breaker behaviour can be asserted without a network.
type flakyTool struct {
	toolName          string
	failuresRemaining int
	failureClass      FailureClass
	idempotent        bool
	invokeCount       int
}

func (flaky *flakyTool) Name() string                 { return flaky.toolName }
func (flaky *flakyTool) Description() string          { return "flaky test tool" }
func (flaky *flakyTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (flaky *flakyTool) Idempotent() bool             { return flaky.idempotent }

func (flaky *flakyTool) Invoke(ctx context.Context, arguments json.RawMessage) (Result, error) {
	flaky.invokeCount++
	if flaky.failuresRemaining > 0 {
		flaky.failuresRemaining--
		return Result{}, NewFailure(flaky.failureClass, flaky.toolName, "injected failure", errors.New("boom"))
	}
	return Text("ok", Internal()), nil
}

func testPolicy() Policy {
	return Policy{
		Timeout: time.Second, MaxAttempts: 3,
		BaseBackoff: time.Millisecond, MaxBackoff: time.Millisecond,
		BreakerThreshold: 2, BreakerCooldown: time.Hour,
		Sleep:  func(context.Context, time.Duration) error { return nil },
		Jitter: func() float64 { return 1 },
	}
}

func TestRetryRecoversFromTransientFailure(t *testing.T) {
	flaky := &flakyTool{toolName: "flaky", failuresRemaining: 2, failureClass: FailureRetryable, idempotent: true}
	registry := NewRegistry()
	registry.Register(flaky, testPolicy())

	result, err := registry.Invoke(context.Background(), "flaky", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("expected recovery after retries, got %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("unexpected content %q", result.Content)
	}
	if flaky.invokeCount != 3 {
		t.Fatalf("expected 3 attempts, got %d", flaky.invokeCount)
	}
}

func TestNonIdempotentToolIsNeverRetried(t *testing.T) {
	// The rule that stops a re-run flooding Jira with duplicate tickets.
	writeTool := &flakyTool{toolName: "jira.push", failuresRemaining: 1, failureClass: FailureRetryable, idempotent: false}
	registry := NewRegistry()
	registry.Register(writeTool, testPolicy())

	if _, err := registry.Invoke(context.Background(), "jira.push", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected the failure to surface rather than be retried away")
	}
	if writeTool.invokeCount != 1 {
		t.Fatalf("a non-idempotent tool must be attempted exactly once, got %d", writeTool.invokeCount)
	}
}

func TestCorrectableFailureIsNotRetriedAndDoesNotTripBreaker(t *testing.T) {
	// A correctable error is the model's mistake. Retrying it wastes attempts,
	// and counting it toward the breaker would take a working tool out of
	// service because the model sent bad arguments.
	correctableTool := &flakyTool{
		toolName: "picky", failuresRemaining: 10, failureClass: FailureCorrectable, idempotent: true,
	}
	registry := NewRegistry()
	registry.Register(correctableTool, testPolicy())

	for callNumber := 1; callNumber <= 5; callNumber++ {
		_, err := registry.Invoke(context.Background(), "picky", json.RawMessage(`{}`))
		if ClassOf(err) != FailureCorrectable {
			t.Fatalf("call %d: expected correctable, got %v", callNumber, ClassOf(err))
		}
	}
	if correctableTool.invokeCount != 5 {
		t.Fatalf("expected one attempt per call, got %d", correctableTool.invokeCount)
	}
	if degraded := registry.DegradedTools(); len(degraded) != 0 {
		t.Fatalf("correctable failures must not open the breaker, got %v", degraded)
	}
}

func TestBreakerOpensAndHidesToolFromModel(t *testing.T) {
	brokenTool := &flakyTool{toolName: "remote", failuresRemaining: 100, failureClass: FailureRetryable, idempotent: true}
	registry := NewRegistry()
	policy := testPolicy()
	policy.MaxAttempts = 1
	registry.Register(brokenTool, policy)

	for callNumber := 0; callNumber < 2; callNumber++ {
		_, _ = registry.Invoke(context.Background(), "remote", json.RawMessage(`{}`))
	}

	if _, err := registry.Invoke(context.Background(), "remote", json.RawMessage(`{}`)); !errors.Is(err, ErrBreakerOpen) {
		t.Fatalf("expected an open breaker, got %v", err)
	}
	// The model must stop being offered a capability that cannot work.
	for _, schema := range registry.Schemas() {
		if schema.Name == "remote" {
			t.Fatal("a tool with an open breaker must be dropped from the schemas shown to the model")
		}
	}
	if degraded := registry.DegradedTools(); len(degraded) != 1 || degraded[0] != "remote" {
		t.Fatalf("expected remote to be reported degraded, got %v", degraded)
	}
}

func TestFallbackUsedOnDegradedPrimaryAndLowersConfidence(t *testing.T) {
	primaryTool := &flakyTool{toolName: "parse", failuresRemaining: 100, failureClass: FailureDegradable, idempotent: true}
	secondaryTool := &flakyTool{toolName: "parse", failuresRemaining: 0, idempotent: true}

	fallbackCalled := false
	chained := &Fallback{
		Primary: primaryTool, Secondary: secondaryTool,
		OnFallback: func(string, error) { fallbackCalled = true },
	}
	result, err := chained.Invoke(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("fallback should have answered, got %v", err)
	}
	if !fallbackCalled {
		t.Fatal("OnFallback should fire so the run can record reduced confidence")
	}
	if result.Confidence >= 1 {
		t.Fatalf("a fallback result must carry reduced confidence, got %v", result.Confidence)
	}
}

func TestFallbackNotUsedForCorrectableFailure(t *testing.T) {
	// Bad arguments will be just as bad for the secondary.
	primaryTool := &flakyTool{toolName: "parse", failuresRemaining: 1, failureClass: FailureCorrectable, idempotent: true}
	secondaryTool := &flakyTool{toolName: "parse", idempotent: true}
	chained := &Fallback{Primary: primaryTool, Secondary: secondaryTool}

	if _, err := chained.Invoke(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected the correctable failure to surface")
	}
	if secondaryTool.invokeCount != 0 {
		t.Fatal("the secondary must not be tried for a correctable failure")
	}
}

func TestUnknownToolIsCorrectableAndNamesAlternatives(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&flakyTool{toolName: "repo.tree", idempotent: true}, testPolicy())

	_, err := registry.Invoke(context.Background(), "repo.walk", json.RawMessage(`{}`))
	if ClassOf(err) != FailureCorrectable {
		t.Fatalf("calling an unknown tool is the model's mistake to fix, got %v", ClassOf(err))
	}
	if advice := AdviceOf(err); !strings.Contains(advice, "repo.tree") {
		t.Fatalf("advice must name what the model may actually call, got %q", advice)
	}
}

func TestScopedRegistryRefusesUnknownTool(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&flakyTool{toolName: "repo.tree", idempotent: true}, testPolicy())
	if _, err := registry.Scoped("jira.push"); err == nil {
		t.Fatal("scoping to a tool that was never registered must fail loudly at wiring time")
	}
}
