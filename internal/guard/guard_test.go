package guard

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

func call(toolName, arguments string) llm.ToolCall {
	return llm.ToolCall{ID: "c1", ToolName: toolName, Arguments: json.RawMessage(arguments)}
}

func TestFingerprintIgnoresArgumentKeyOrder(t *testing.T) {
	// Key order must not disguise a repeat, or the guard is trivially defeated
	// by a model that shuffles its JSON.
	first := call("repo_read_file", `{"path":"a.go","startLine":1}`)
	second := call("repo_read_file", `{"startLine":1,"path":"a.go"}`)
	if Fingerprint(first) != Fingerprint(second) {
		t.Fatal("identical calls with reordered keys must share a fingerprint")
	}
}

func TestFingerprintDistinguishesDifferentArguments(t *testing.T) {
	if Fingerprint(call("repo_read_file", `{"path":"a.go"}`)) ==
		Fingerprint(call("repo_read_file", `{"path":"b.go"}`)) {
		t.Fatal("different arguments must not collide")
	}
}

func TestRepeatCallGuardGraduatesFromCacheToAdviceToStop(t *testing.T) {
	repeatGuard := DefaultRepeatCallGuard() // cache at 2, advise at 3, terminate at 4
	state := &State{AgentName: "analyst", StartedAt: time.Now()}
	toolCall := call("repo_read_file", `{"path":"a.go"}`)
	ctx := context.Background()

	// First occurrence proceeds and its result is cached.
	if err := repeatGuard.BeforeToolCall(ctx, state, toolCall); err != nil {
		t.Fatalf("first call must proceed, got %v", err)
	}
	if disposition, _ := repeatGuard.Disposition(toolCall); disposition != RepeatProceed {
		t.Fatalf("first call disposition should be proceed, got %v", disposition)
	}
	if err := repeatGuard.AfterToolCall(ctx, state, toolCall, "file contents", false); err != nil {
		t.Fatalf("recording a result must not fail: %v", err)
	}

	// Second occurrence is served from cache: cheaper and just as correct.
	if err := repeatGuard.BeforeToolCall(ctx, state, toolCall); err != nil {
		t.Fatalf("second call must not stop the loop, got %v", err)
	}
	disposition, payload := repeatGuard.Disposition(toolCall)
	if disposition != RepeatServeCache || payload != "file contents" {
		t.Fatalf("second call should serve the cached result, got %v %q", disposition, payload)
	}

	// Third occurrence tells the model plainly to change approach.
	if err := repeatGuard.BeforeToolCall(ctx, state, toolCall); err != nil {
		t.Fatalf("third call must not stop the loop, got %v", err)
	}
	disposition, advice := repeatGuard.Disposition(toolCall)
	if disposition != RepeatAdvise {
		t.Fatalf("third call should advise, got %v", disposition)
	}
	if advice == "" {
		t.Fatal("advice must be non-empty and actionable")
	}

	// Fourth occurrence ends the agent.
	err := repeatGuard.BeforeToolCall(ctx, state, toolCall)
	stop, isStop := IsStop(err)
	if !isStop || stop.Reason != StopRepeatedCalls {
		t.Fatalf("fourth identical call must stop the agent, got %v", err)
	}
}

func TestRepeatCallGuardDoesNotCacheErrors(t *testing.T) {
	repeatGuard := DefaultRepeatCallGuard()
	state := &State{AgentName: "analyst", StartedAt: time.Now()}
	toolCall := call("repo_read_file", `{"path":"missing.go"}`)
	ctx := context.Background()

	_ = repeatGuard.BeforeToolCall(ctx, state, toolCall)
	_ = repeatGuard.AfterToolCall(ctx, state, toolCall, "no such path", true)
	_ = repeatGuard.BeforeToolCall(ctx, state, toolCall)

	if disposition, _ := repeatGuard.Disposition(toolCall); disposition == RepeatServeCache {
		t.Fatal("an error result must not be cached and replayed as if it were data")
	}
}

func TestProgressGuardStopsBarrenAgent(t *testing.T) {
	progressGuard := NewProgressGuard(2)
	state := &State{AgentName: "author", BlackboardRevision: 7, StartedAt: time.Now()}
	ctx := context.Background()

	if err := progressGuard.BeforeIteration(ctx, state); err != nil {
		t.Fatalf("first iteration establishes a baseline, got %v", err)
	}
	if err := progressGuard.BeforeIteration(ctx, state); err != nil {
		t.Fatalf("one barren iteration is not yet a stop, got %v", err)
	}
	err := progressGuard.BeforeIteration(ctx, state)
	stop, isStop := IsStop(err)
	if !isStop || stop.Reason != StopNoProgress {
		t.Fatalf("expected a no-progress stop, got %v", err)
	}
}

func TestProgressGuardResetsWhenBlackboardChanges(t *testing.T) {
	progressGuard := NewProgressGuard(2)
	state := &State{AgentName: "author", BlackboardRevision: 1, StartedAt: time.Now()}
	ctx := context.Background()

	_ = progressGuard.BeforeIteration(ctx, state)
	_ = progressGuard.BeforeIteration(ctx, state) // barren
	state.BlackboardRevision = 2                  // work happened
	if err := progressGuard.BeforeIteration(ctx, state); err != nil {
		t.Fatalf("progress must reset the barren counter, got %v", err)
	}
	if err := progressGuard.BeforeIteration(ctx, state); err != nil {
		t.Fatalf("counter should have reset, got %v", err)
	}
}

func TestBudgetGuardStopsOnEachDimension(t *testing.T) {
	testCases := []struct {
		name           string
		budget         Budget
		state          State
		expectedReason StopReason
	}{
		{
			name:           "iterations",
			budget:         Budget{MaxIterations: 3},
			state:          State{IterationCount: 3},
			expectedReason: StopIterationCap,
		},
		{
			name:           "tokens",
			budget:         Budget{MaxTokens: 1000},
			state:          State{TokensUsed: 1000},
			expectedReason: StopBudgetExhausted,
		},
		{
			name:           "wall clock",
			budget:         Budget{MaxWallClock: time.Minute},
			state:          State{StartedAt: time.Now().Add(-2 * time.Minute)},
			expectedReason: StopBudgetExhausted,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			budgetGuard := NewBudgetGuard(testCase.budget)
			stateCopy := testCase.state
			stateCopy.AgentName = "analyst"
			if stateCopy.StartedAt.IsZero() {
				stateCopy.StartedAt = time.Now()
			}
			err := budgetGuard.BeforeIteration(context.Background(), &stateCopy)
			stop, isStop := IsStop(err)
			if !isStop || stop.Reason != testCase.expectedReason {
				t.Fatalf("expected %s, got %v", testCase.expectedReason, err)
			}
		})
	}
}

// fixedScope is a ScopeChecker over a static set.
type fixedScope struct{ allowed map[string]bool }

func (scope fixedScope) Has(toolName string) bool { return scope.allowed[toolName] }
func (scope fixedScope) Names() []string {
	names := []string{}
	for toolName := range scope.allowed {
		names = append(names, toolName)
	}
	return names
}

func TestScopeGuardBlocksOutOfScopeTool(t *testing.T) {
	scopeGuard := NewScopeGuard(fixedScope{allowed: map[string]bool{"repo_tree": true}})
	state := &State{AgentName: "surveyor", StartedAt: time.Now()}

	if err := scopeGuard.BeforeToolCall(context.Background(), state, call("repo_tree", `{}`)); err != nil {
		t.Fatalf("an in-scope tool must be allowed, got %v", err)
	}
	err := scopeGuard.BeforeToolCall(context.Background(), state, call("jira_push", `{}`))
	stop, isStop := IsStop(err)
	if !isStop || stop.Reason != StopOutOfScope {
		t.Fatalf("expected an out-of-scope stop, got %v", err)
	}
}

func TestChainReturnsFirstStop(t *testing.T) {
	chain := Chain{
		NewBudgetGuard(Budget{MaxIterations: 1}),
		NewProgressGuard(1),
	}
	state := &State{AgentName: "risk", IterationCount: 5, StartedAt: time.Now()}
	stop, isStop := IsStop(chain.BeforeIteration(context.Background(), state))
	if !isStop || stop.Guard != "budget" {
		t.Fatalf("the first guard to object should win, got %v", stop)
	}
}
