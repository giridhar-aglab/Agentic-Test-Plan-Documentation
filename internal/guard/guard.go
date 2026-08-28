// Package guard holds the loop-avoidance layer.
//
// Five guards, defence in depth. The strongest is not in this package at all:
// the phase DAG and scoped registries mean agents cannot invoke each other, so
// mutual recursion is not something to detect but something that cannot be
// constructed. What lives here catches the remaining cases, where a single
// agent spins on its own.
package guard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

// StopReason explains why a guard ended an agent loop. Typed rather than a
// string, because the orchestrator branches on it: a budget stop still yields
// partial results, while a scope violation is a bug.
type StopReason string

const (
	StopBudgetExhausted StopReason = "budget_exhausted"
	StopRepeatedCalls   StopReason = "repeated_tool_calls"
	StopNoProgress      StopReason = "no_progress"
	StopOutOfScope      StopReason = "out_of_scope"
	StopIterationCap    StopReason = "iteration_cap"
)

// Stop is the error a guard returns to end the loop.
type Stop struct {
	Reason StopReason
	Detail string
	Guard  string
}

func (stop *Stop) Error() string {
	return fmt.Sprintf("guard %s stopped the loop (%s): %s", stop.Guard, stop.Reason, stop.Detail)
}

// IsStop reports whether an error is a deliberate guard stop rather than a
// fault. A stop means "wrap up and emit what you have", never "panic".
func IsStop(err error) (*Stop, bool) {
	var stop *Stop
	if errors.As(err, &stop) {
		return stop, true
	}
	return nil, false
}

// State is what guards may inspect between iterations.
type State struct {
	AgentName          string
	IterationCount     int
	BlackboardRevision int
	TokensUsed         int
	ToolCallCount      int
	StartedAt          time.Time
}

// Guard hooks into the agent loop at three points.
type Guard interface {
	Name() string
	BeforeIteration(ctx context.Context, state *State) error
	BeforeToolCall(ctx context.Context, state *State, toolCall llm.ToolCall) error
	AfterToolCall(ctx context.Context, state *State, toolCall llm.ToolCall, content string, isError bool) error
}

// Chain runs guards in order, returning the first stop.
type Chain []Guard

func (chain Chain) Name() string { return "chain" }

func (chain Chain) BeforeIteration(ctx context.Context, state *State) error {
	for _, singleGuard := range chain {
		if err := singleGuard.BeforeIteration(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (chain Chain) BeforeToolCall(ctx context.Context, state *State, toolCall llm.ToolCall) error {
	for _, singleGuard := range chain {
		if err := singleGuard.BeforeToolCall(ctx, state, toolCall); err != nil {
			return err
		}
	}
	return nil
}

func (chain Chain) AfterToolCall(ctx context.Context, state *State, toolCall llm.ToolCall, content string, isError bool) error {
	for _, singleGuard := range chain {
		if err := singleGuard.AfterToolCall(ctx, state, toolCall, content, isError); err != nil {
			return err
		}
	}
	return nil
}

// base gives guards no-op implementations of the hooks they do not use.
type base struct{}

func (base) BeforeIteration(context.Context, *State) error                           { return nil }
func (base) BeforeToolCall(context.Context, *State, llm.ToolCall) error              { return nil }
func (base) AfterToolCall(context.Context, *State, llm.ToolCall, string, bool) error { return nil }

// ---------------------------------------------------------------- budget ---

// Budget caps what one agent, or a whole run, may consume.
type Budget struct {
	MaxIterations int
	MaxToolCalls  int
	MaxTokens     int
	MaxWallClock  time.Duration
}

// BudgetGuard enforces a budget. Exceeding it is a clean termination with
// partial results, never a panic.
type BudgetGuard struct {
	base
	budget Budget
	now    func() time.Time
}

// NewBudgetGuard builds a budget guard.
func NewBudgetGuard(budget Budget) *BudgetGuard {
	return &BudgetGuard{budget: budget, now: time.Now}
}

func (budgetGuard *BudgetGuard) Name() string { return "budget" }

func (budgetGuard *BudgetGuard) BeforeIteration(ctx context.Context, state *State) error {
	budget := budgetGuard.budget
	if budget.MaxIterations > 0 && state.IterationCount >= budget.MaxIterations {
		return &Stop{Reason: StopIterationCap, Guard: "budget",
			Detail: fmt.Sprintf("%s reached %d iterations", state.AgentName, budget.MaxIterations)}
	}
	if budget.MaxTokens > 0 && state.TokensUsed >= budget.MaxTokens {
		return &Stop{Reason: StopBudgetExhausted, Guard: "budget",
			Detail: fmt.Sprintf("%s used %d tokens of %d", state.AgentName, state.TokensUsed, budget.MaxTokens)}
	}
	if budget.MaxWallClock > 0 && budgetGuard.now().Sub(state.StartedAt) >= budget.MaxWallClock {
		return &Stop{Reason: StopBudgetExhausted, Guard: "budget",
			Detail: fmt.Sprintf("%s exceeded %s wall clock", state.AgentName, budget.MaxWallClock)}
	}
	return nil
}

func (budgetGuard *BudgetGuard) BeforeToolCall(ctx context.Context, state *State, toolCall llm.ToolCall) error {
	if budgetGuard.budget.MaxToolCalls > 0 && state.ToolCallCount >= budgetGuard.budget.MaxToolCalls {
		return &Stop{Reason: StopBudgetExhausted, Guard: "budget",
			Detail: fmt.Sprintf("%s reached %d tool calls", state.AgentName, budgetGuard.budget.MaxToolCalls)}
	}
	return nil
}

// ----------------------------------------------------------- repeat calls ---

// RepeatCallGuard detects a model calling the same tool with the same
// arguments. The graduated response matters: serving a cached result is both
// cheaper and more correct than refusing, and only a model that keeps asking
// after being told to change approach is actually stuck.
type RepeatCallGuard struct {
	base
	cacheAtCount     int
	adviseAtCount    int
	terminateAtCount int

	mutex               sync.Mutex
	countsByFingerprint map[string]int
	cachedByFingerprint map[string]string
}

// NewRepeatCallGuard builds the guard with its three thresholds: the
// occurrence at which a cached result is served instead of re-invoking, the
// occurrence at which the model is told plainly to change approach, and the
// occurrence at which the agent is terminated.
//
// The graduation is the point. Serving a cache is cheaper and just as correct;
// only a model that keeps asking after being told to stop is actually stuck.
func NewRepeatCallGuard(cacheAtCount, adviseAtCount, terminateAtCount int) *RepeatCallGuard {
	return &RepeatCallGuard{
		cacheAtCount: cacheAtCount, adviseAtCount: adviseAtCount, terminateAtCount: terminateAtCount,
		countsByFingerprint: map[string]int{}, cachedByFingerprint: map[string]string{},
	}
}

// DefaultRepeatCallGuard is the 2nd-cache, 3rd-advise, 4th-terminate policy.
func DefaultRepeatCallGuard() *RepeatCallGuard { return NewRepeatCallGuard(2, 3, 4) }

func (repeatGuard *RepeatCallGuard) Name() string { return "repeat-call" }

// RepeatDisposition tells the loop how to handle a call the guard has seen
// before.
type RepeatDisposition int

const (
	// RepeatProceed means invoke the tool normally.
	RepeatProceed RepeatDisposition = iota
	// RepeatServeCache means return the stored result without invoking.
	RepeatServeCache
	// RepeatAdvise means return a correctable error telling the model to change
	// approach.
	RepeatAdvise
)

// Disposition reports how the loop should treat this call, given how many
// times it has already been made. It does not mutate the counter; BeforeToolCall
// does that.
func (repeatGuard *RepeatCallGuard) Disposition(toolCall llm.ToolCall) (RepeatDisposition, string) {
	repeatGuard.mutex.Lock()
	defer repeatGuard.mutex.Unlock()

	fingerprint := Fingerprint(toolCall)
	occurrenceCount := repeatGuard.countsByFingerprint[fingerprint]

	if repeatGuard.adviseAtCount > 0 && occurrenceCount >= repeatGuard.adviseAtCount {
		return RepeatAdvise, RepeatAdvice(toolCall, occurrenceCount)
	}
	if repeatGuard.cacheAtCount > 0 && occurrenceCount >= repeatGuard.cacheAtCount {
		if cachedContent, found := repeatGuard.cachedByFingerprint[fingerprint]; found {
			return RepeatServeCache, cachedContent
		}
	}
	return RepeatProceed, ""
}

// Fingerprint hashes a call so argument key order cannot disguise a repeat.
func Fingerprint(toolCall llm.ToolCall) string {
	canonicalArguments := canonicalJSON(toolCall.Arguments)
	digest := sha256.Sum256([]byte(toolCall.ToolName + "\x00" + canonicalArguments))
	return hex.EncodeToString(digest[:8])
}

func canonicalJSON(raw json.RawMessage) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	var builder strings.Builder
	writeCanonical(&builder, decoded)
	return builder.String()
}

func writeCanonical(builder *strings.Builder, value any) {
	switch typedValue := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typedValue))
		for key := range typedValue {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteByte('{')
		for keyIndex, key := range keys {
			if keyIndex > 0 {
				builder.WriteByte(',')
			}
			fmt.Fprintf(builder, "%q:", key)
			writeCanonical(builder, typedValue[key])
		}
		builder.WriteByte('}')
	case []any:
		builder.WriteByte('[')
		for elementIndex, element := range typedValue {
			if elementIndex > 0 {
				builder.WriteByte(',')
			}
			writeCanonical(builder, element)
		}
		builder.WriteByte(']')
	default:
		encoded, _ := json.Marshal(typedValue)
		builder.Write(encoded)
	}
}

// OccurrenceCount reports how many times a call has been seen. Exposed for
// tests and for the run report.
func (repeatGuard *RepeatCallGuard) OccurrenceCount(toolCall llm.ToolCall) int {
	repeatGuard.mutex.Lock()
	defer repeatGuard.mutex.Unlock()
	return repeatGuard.countsByFingerprint[Fingerprint(toolCall)]
}

func (repeatGuard *RepeatCallGuard) BeforeToolCall(ctx context.Context, state *State, toolCall llm.ToolCall) error {
	repeatGuard.mutex.Lock()
	defer repeatGuard.mutex.Unlock()

	fingerprint := Fingerprint(toolCall)
	repeatGuard.countsByFingerprint[fingerprint]++
	occurrenceCount := repeatGuard.countsByFingerprint[fingerprint]

	if repeatGuard.terminateAtCount > 0 && occurrenceCount >= repeatGuard.terminateAtCount {
		return &Stop{Reason: StopRepeatedCalls, Guard: "repeat-call",
			Detail: fmt.Sprintf("%s called %s with identical arguments %d times",
				state.AgentName, toolCall.ToolName, occurrenceCount)}
	}
	return nil
}

func (repeatGuard *RepeatCallGuard) AfterToolCall(ctx context.Context, state *State, toolCall llm.ToolCall, content string, isError bool) error {
	if isError {
		return nil
	}
	repeatGuard.mutex.Lock()
	defer repeatGuard.mutex.Unlock()
	repeatGuard.cachedByFingerprint[Fingerprint(toolCall)] = content
	return nil
}

// RepeatAdvice is the message handed to the model on the middle occurrence:
// specific, and it names what to do instead.
func RepeatAdvice(toolCall llm.ToolCall, occurrenceCount int) string {
	return fmt.Sprintf(
		"You have already called %s with these exact arguments %d times and received the same result. "+
			"Repeating it will not produce new information. Either call a different tool, change the "+
			"arguments, or state your conclusion from what you already have.",
		toolCall.ToolName, occurrenceCount)
}

// -------------------------------------------------------------- progress ---

// ProgressGuard ends an agent that is burning iterations without adding
// anything to the blackboard. Progress is measured by the blackboard revision
// counter, so "did anything happen" is an integer comparison rather than a
// deep diff.
type ProgressGuard struct {
	base
	barrenIterationLimit int

	lastSeenRevision       int
	consecutiveBarrenCount int
	hasSeenAnyIteration    bool
}

// NewProgressGuard stops an agent after the given number of consecutive
// iterations that change nothing.
func NewProgressGuard(barrenIterationLimit int) *ProgressGuard {
	return &ProgressGuard{barrenIterationLimit: barrenIterationLimit}
}

func (progressGuard *ProgressGuard) Name() string { return "progress" }

func (progressGuard *ProgressGuard) BeforeIteration(ctx context.Context, state *State) error {
	if !progressGuard.hasSeenAnyIteration {
		progressGuard.hasSeenAnyIteration = true
		progressGuard.lastSeenRevision = state.BlackboardRevision
		return nil
	}
	if state.BlackboardRevision == progressGuard.lastSeenRevision {
		progressGuard.consecutiveBarrenCount++
	} else {
		progressGuard.consecutiveBarrenCount = 0
		progressGuard.lastSeenRevision = state.BlackboardRevision
	}
	if progressGuard.barrenIterationLimit > 0 && progressGuard.consecutiveBarrenCount >= progressGuard.barrenIterationLimit {
		return &Stop{Reason: StopNoProgress, Guard: "progress",
			Detail: fmt.Sprintf("%s ran %d iterations without changing the blackboard",
				state.AgentName, progressGuard.consecutiveBarrenCount)}
	}
	return nil
}

// ----------------------------------------------------------------- scope ---

// ScopeChecker reports whether a tool is in an agent's registry.
type ScopeChecker interface {
	Has(toolName string) bool
	Names() []string
}

// ScopeGuard refuses calls to tools outside the agent's scope. The registry
// would refuse anyway; catching it here makes the reason explicit in the stop
// rather than surfacing as a puzzling tool error.
type ScopeGuard struct {
	base
	scopeChecker ScopeChecker
}

// NewScopeGuard builds a scope guard over a registry.
func NewScopeGuard(scopeChecker ScopeChecker) *ScopeGuard {
	return &ScopeGuard{scopeChecker: scopeChecker}
}

func (scopeGuard *ScopeGuard) Name() string { return "scope" }

func (scopeGuard *ScopeGuard) BeforeToolCall(ctx context.Context, state *State, toolCall llm.ToolCall) error {
	if scopeGuard.scopeChecker.Has(toolCall.ToolName) {
		return nil
	}
	return &Stop{Reason: StopOutOfScope, Guard: "scope",
		Detail: fmt.Sprintf("%s attempted %s, which is not in its scope %v",
			state.AgentName, toolCall.ToolName, scopeGuard.scopeChecker.Names())}
}
