package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/tool"
)

// stubTool answers with a fixed body, or a classified failure.
type stubTool struct {
	toolName    string
	body        string
	failWith    error
	invokeCount int
}

func (stub *stubTool) Name() string                 { return stub.toolName }
func (stub *stubTool) Description() string          { return "stub" }
func (stub *stubTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (stub *stubTool) Idempotent() bool             { return true }

func (stub *stubTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	stub.invokeCount++
	if stub.failWith != nil {
		return tool.Result{}, stub.failWith
	}
	return tool.Text(stub.body, tool.Internal()), nil
}

func newTestLoop(t *testing.T, provider llm.Provider, tools []tool.Tool, guards guard.Chain) (*Loop, *memory.Blackboard) {
	t.Helper()
	registry := tool.NewRegistry()
	for _, registeredTool := range tools {
		registry.Register(registeredTool, tool.LocalPolicy())
	}
	blackboard := memory.NewBlackboard("test-run")
	return NewLoop(provider, registry, blackboard, guards), blackboard
}

func TestLoopFeedsCorrectableErrorBackToModel(t *testing.T) {
	// The central distinction: a tool rejecting its arguments is the model's
	// problem, and it can only fix what it can read.
	brokenTool := &stubTool{
		toolName: "repo_read_file",
		failWith: tool.Correctable("repo_read_file",
			"no such path \"missing.go\"; call repo_tree to list valid paths"),
	}
	provider := llm.NewFakeProvider(
		llm.ToolCallTurn("c1", "repo_read_file", map[string]string{"path": "missing.go"}),
		llm.TextTurn("I could not find that file."),
	)
	loop, _ := newTestLoop(t, provider, []tool.Tool{brokenTool}, guard.Chain{})

	outcome, err := loop.Run(context.Background(), Task{AgentName: "analyst", Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("a correctable error must not fail the run: %v", err)
	}
	if !outcome.Completed() {
		t.Fatalf("the loop should have finished normally, stopped by %v", outcome.StoppedBy)
	}

	// The second request must carry the advice, or the model has nothing to
	// correct from.
	recordedCalls := provider.RecordedCalls()
	if len(recordedCalls) != 2 {
		t.Fatalf("expected two completions, got %d", len(recordedCalls))
	}
	if !requestMentions(recordedCalls[1].Request, "call repo_tree to list valid paths") {
		t.Fatal("the model's next turn must contain the actionable advice from the failed tool")
	}
}

func TestLoopMarksDegradedToolOnBlackboard(t *testing.T) {
	degradedTool := &stubTool{
		toolName: "repo_commits",
		failWith: tool.NewFailure(tool.FailureDegradable, "repo_commits",
			"commit history unavailable; continue without churn signal", nil),
	}
	provider := llm.NewFakeProvider(
		llm.ToolCallTurn("c1", "repo_commits", map[string]string{}),
		llm.TextTurn("done"),
	)
	loop, blackboard := newTestLoop(t, provider, []tool.Tool{degradedTool}, guard.Chain{})

	if _, err := loop.Run(context.Background(), Task{AgentName: "risk", Tier: llm.TierFast}); err != nil {
		t.Fatalf("a degradable failure must not fail the run: %v", err)
	}
	degraded := blackboard.DegradedTools()
	if _, recorded := degraded["repo_commits"]; !recorded {
		t.Fatalf("a degraded capability must be recorded for the report, got %v", degraded)
	}
}

func TestLoopServesCachedResultForRepeatedCall(t *testing.T) {
	repeatedTool := &stubTool{toolName: "repo_tree", body: "a.go\nb.go"}
	provider := llm.NewFakeProvider(
		llm.ToolCallTurn("c1", "repo_tree", map[string]string{}),
		llm.ToolCallTurn("c2", "repo_tree", map[string]string{}),
		llm.TextTurn("done"),
	)
	repeatGuard := guard.DefaultRepeatCallGuard()
	loop, _ := newTestLoop(t, provider, []tool.Tool{repeatedTool}, guard.Chain{repeatGuard})
	loop.RepeatGuard = repeatGuard

	if _, err := loop.Run(context.Background(), Task{AgentName: "surveyor", Tier: llm.TierFast}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repeatedTool.invokeCount != 1 {
		t.Fatalf("the second identical call should be served from cache, tool invoked %d times",
			repeatedTool.invokeCount)
	}
}

func TestLoopStopsOnRepeatedCallsAndReturnsPartialOutcome(t *testing.T) {
	stubbornTool := &stubTool{toolName: "repo_tree", body: "same answer"}
	turns := []llm.ScriptedTurn{}
	for callNumber := 0; callNumber < 6; callNumber++ {
		turns = append(turns, llm.ToolCallTurn("c", "repo_tree", map[string]string{}))
	}
	provider := llm.NewFakeProvider(turns...)
	repeatGuard := guard.DefaultRepeatCallGuard()
	loop, _ := newTestLoop(t, provider, []tool.Tool{stubbornTool}, guard.Chain{repeatGuard})
	loop.RepeatGuard = repeatGuard

	outcome, err := loop.Run(context.Background(), Task{AgentName: "surveyor", Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("a guard stop is an orderly end, not an error: %v", err)
	}
	if outcome.StoppedBy == nil || outcome.StoppedBy.Reason != guard.StopRepeatedCalls {
		t.Fatalf("expected a repeated-calls stop, got %v", outcome.StoppedBy)
	}
	if outcome.IterationCount == 0 {
		t.Fatal("a stopped outcome must still report the work done before the stop")
	}
}

func TestLoopStopsOnIterationBudget(t *testing.T) {
	spinningTool := &stubTool{toolName: "probe", body: "still going"}
	turns := []llm.ScriptedTurn{}
	for callNumber := 0; callNumber < 20; callNumber++ {
		// Distinct arguments each time, so only the budget can stop this.
		turns = append(turns, llm.ToolCallTurn("c", "probe", map[string]int{"n": callNumber}))
	}
	provider := llm.NewFakeProvider(turns...)
	budgetGuard := guard.NewBudgetGuard(guard.Budget{MaxIterations: 4})
	loop, _ := newTestLoop(t, provider, []tool.Tool{spinningTool}, guard.Chain{budgetGuard})

	outcome, err := loop.Run(context.Background(), Task{AgentName: "analyst", Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.StoppedBy == nil || outcome.StoppedBy.Reason != guard.StopIterationCap {
		t.Fatalf("expected an iteration-cap stop, got %v", outcome.StoppedBy)
	}
	if outcome.IterationCount != 4 {
		t.Fatalf("expected exactly 4 iterations before the cap, got %d", outcome.IterationCount)
	}
}

func TestLoopSpillsLargeToolResults(t *testing.T) {
	bulkyTool := &stubTool{toolName: "repo_tree", body: strings.Repeat("some/long/path.go\n", 500)}
	provider := llm.NewFakeProvider(
		llm.ToolCallTurn("c1", "repo_tree", map[string]string{}),
		llm.TextTurn("done"),
	)
	loop, _ := newTestLoop(t, provider, []tool.Tool{bulkyTool}, guard.Chain{})
	loop.SpillThresholdBytes = 500

	if _, err := loop.Run(context.Background(), Task{AgentName: "surveyor", Tier: llm.TierFast}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secondRequest := provider.RecordedCalls()[1].Request
	if !requestMentions(secondRequest, "[handle: spill-") {
		t.Fatal("a large payload must be replaced in context by a digest and a handle")
	}
	if llm.EstimateTokens(secondRequest) > 1000 {
		t.Fatalf("spilling should keep the context small, got roughly %d tokens",
			llm.EstimateTokens(secondRequest))
	}
}

func requestMentions(request llm.Request, needle string) bool {
	for _, message := range request.Messages {
		if strings.Contains(message.Text, needle) {
			return true
		}
		for _, toolResult := range message.ToolResults {
			if strings.Contains(toolResult.Content, needle) {
				return true
			}
		}
	}
	return false
}

// spillingTool returns a payload big enough to be spilled out of context.
type spillingTool struct{ payload string }

func (spillingTool) Name() string { return "big_read" }
func (spillingTool) Description() string {
	return "returns a large payload"
}
func (spillingTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (spillingTool) Idempotent() bool             { return true }
func (spilling spillingTool) Invoke(context.Context, json.RawMessage) (tool.Result, error) {
	return tool.Text(spilling.payload, tool.Internal()), nil
}

func TestASpilledResultCanActuallyBeRetrieved(t *testing.T) {
	// The digest tells the model to fetch the rest by handle. For a long time
	// nothing could: a model handed a truncated file and an unusable
	// instruction re-reads the file until its budget is gone, which is an
	// agent loop caused by the system, not the model. The instruction and the
	// capability have to ship together.
	largePayload := strings.Repeat("package main // a long file\n", 4000)
	provider := llm.NewFakeProvider(
		llm.ScriptedTurn{Response: llm.Response{
			StopReason: llm.StopToolUse,
			ToolCalls: []llm.ToolCall{{
				ID: "c1", ToolName: "big_read", Arguments: json.RawMessage(`{}`),
			}},
		}},
		llm.ScriptedTurn{Response: llm.Response{StopReason: llm.StopEndTurn, Text: "done"}},
	)
	loop, _ := newTestLoop(t, provider, []tool.Tool{spillingTool{payload: largePayload}}, guard.Chain{})
	loop.SpillThresholdBytes = 1000

	// The digest names context_fetch, so context_fetch must be callable.
	if !loop.Registry.Has("context_fetch") {
		t.Fatal("a loop that can spill must offer the tool its own digest names")
	}

	outcome, err := loop.Run(context.Background(), Task{AgentName: "a", Instruction: "read it"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = outcome

	handles := loop.SpillStore.Handles()
	if len(handles) != 1 {
		t.Fatalf("expected the oversized result to be spilled, got %v", handles)
	}
	fetched, err := loop.Registry.Invoke(context.Background(), "context_fetch",
		json.RawMessage(`{"handle":"`+handles[0]+`"}`))
	if err != nil {
		t.Fatalf("the handle in the digest must resolve: %v", err)
	}
	if !strings.Contains(fetched.Content, "package main") {
		t.Fatal("the fetched content should be the spilled payload")
	}
}

func TestAnOrdinarySourceFileIsNeverSpilled(t *testing.T) {
	// Spilling is for outliers. Digesting the very file an agent was told to
	// analyse makes it spend turns recovering what it just asked for — the
	// threshold was 4 KB, which is smaller than most real source files.
	loop := NewLoop(llm.NewFakeProvider(), tool.NewRegistry(), memory.NewBlackboard("r"), guard.Chain{})
	const typicalLargeSourceFile = 40000 // ~1200 lines of Go
	if loop.SpillThresholdBytes < typicalLargeSourceFile {
		t.Fatalf("the spill threshold (%d) is below an ordinary source file (%d); "+
			"every read would be digested", loop.SpillThresholdBytes, typicalLargeSourceFile)
	}
}

func TestAnUnknownHandleIsCorrectableAndNamesTheRealOnes(t *testing.T) {
	loop := NewLoop(llm.NewFakeProvider(), tool.NewRegistry(), memory.NewBlackboard("r"), guard.Chain{})
	realHandle := loop.SpillStore.Put("some spilled content")

	_, err := loop.Registry.Invoke(context.Background(), "context_fetch",
		json.RawMessage(`{"handle":"spill-999-deadbeef"}`))
	if err == nil {
		t.Fatal("an unknown handle must be refused")
	}
	if !strings.Contains(tool.AdviceOf(err), realHandle) {
		t.Errorf("the correction should name the handles that exist, got %q", tool.AdviceOf(err))
	}
}
