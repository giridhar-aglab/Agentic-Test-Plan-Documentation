// Package agent holds the bounded iteration loop every specialist shares, and
// the specialists themselves.
package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/tool"
)

// Task is what an agent is asked to do. It is deliberately small: an agent's
// real input is the blackboard, not a prompt.
type Task struct {
	AgentName    string
	Instruction  string
	SystemPrompt string
	Tier         llm.Tier
}

// Outcome reports how a loop ended.
type Outcome struct {
	AgentName      string
	FinalText      string
	IterationCount int
	ToolCallCount  int
	TokensUsed     int
	StoppedBy      *guard.Stop
	Duration       time.Duration
}

// Completed reports whether the loop ended on its own terms rather than being
// cut short by a guard.
func (outcome Outcome) Completed() bool { return outcome.StoppedBy == nil }

// Loop is the shared agent runtime. Every specialist runs one of these; what
// differs between agents is their registry, their prompt, and what they write
// to the blackboard.
type Loop struct {
	Provider   llm.Provider
	Registry   *tool.Registry
	Guards     guard.Chain
	Blackboard *memory.Blackboard
	SpillStore *memory.SpillStore

	// SpillThresholdBytes is the payload size above which a tool result is
	// stored by handle and replaced in context by a digest.
	SpillThresholdBytes int
	// MaxOutputTokens caps a single completion.
	MaxOutputTokens int
	// RepeatGuard, when set, lets the loop serve a cached result for a repeated
	// call instead of re-invoking the tool.
	RepeatGuard *guard.RepeatCallGuard

	nowFunc func() time.Time
}

// NewLoop builds a loop with sensible defaults.
func NewLoop(provider llm.Provider, registry *tool.Registry, blackboard *memory.Blackboard, guards guard.Chain) *Loop {
	return &Loop{
		Provider: provider, Registry: registry, Blackboard: blackboard, Guards: guards,
		SpillStore: memory.NewSpillStore(), SpillThresholdBytes: 4000,
		MaxOutputTokens: 2000, nowFunc: time.Now,
	}
}

// Run executes the bounded iteration loop.
//
// The loop exits on an explicit condition. "The model returned no tool calls"
// ends this loop, but whether that means the phase is done is the
// orchestrator's decision, made against the phase postcondition — not the
// model's.
func (loop *Loop) Run(ctx context.Context, task Task) (Outcome, error) {
	workingSet := memory.NewWorkingSet(task.SystemPrompt, loop.Provider.Limits(task.Tier).ContextWindowTokens)
	workingSet.Append(llm.Message{Role: llm.RoleUser, Text: task.Instruction})

	loopState := &guard.State{
		AgentName:          task.AgentName,
		BlackboardRevision: loop.Blackboard.Revision(),
		StartedAt:          loop.nowFunc(),
	}
	outcome := Outcome{AgentName: task.AgentName}

	for {
		loopState.BlackboardRevision = loop.Blackboard.Revision()
		if err := loop.Guards.BeforeIteration(ctx, loopState); err != nil {
			return loop.finish(outcome, loopState, err)
		}
		if err := ctx.Err(); err != nil {
			return loop.finish(outcome, loopState, err)
		}

		completionRequest := llm.Request{
			Tier:      task.Tier,
			System:    workingSet.SystemPrompt,
			Messages:  workingSet.Messages(),
			Tools:     loop.Registry.Schemas(),
			MaxTokens: loop.MaxOutputTokens,
		}
		response, err := loop.Provider.Complete(ctx, completionRequest)
		if err != nil {
			return loop.finish(outcome, loopState, fmt.Errorf("agent %s: completion: %w", task.AgentName, err))
		}

		loopState.IterationCount++
		loopState.TokensUsed += response.Usage.Total()

		if len(response.ToolCalls) == 0 {
			outcome.FinalText = response.Text
			return loop.finish(outcome, loopState, nil)
		}

		workingSet.Append(llm.Message{
			Role: llm.RoleAssistant, Text: response.Text, ToolCalls: response.ToolCalls,
		})

		toolResults := make([]llm.ToolResult, 0, len(response.ToolCalls))
		for _, toolCall := range response.ToolCalls {
			if err := loop.Guards.BeforeToolCall(ctx, loopState, toolCall); err != nil {
				workingSet.Append(llm.Message{Role: llm.RoleTool, ToolResults: toolResults})
				return loop.finish(outcome, loopState, err)
			}

			toolResult := loop.invokeOne(ctx, loopState, toolCall)
			toolResults = append(toolResults, toolResult)
			loopState.ToolCallCount++

			if err := loop.Guards.AfterToolCall(ctx, loopState, toolCall, toolResult.Content, toolResult.IsError); err != nil {
				workingSet.Append(llm.Message{Role: llm.RoleTool, ToolResults: toolResults})
				return loop.finish(outcome, loopState, err)
			}
		}
		workingSet.Append(llm.Message{Role: llm.RoleTool, ToolResults: toolResults})
	}
}

// invokeOne dispatches a single tool call and converts whatever comes back
// into something the model can act on.
//
// This is where the central distinction lives: a correctable failure becomes a
// tool result the model reads and fixes, while a transport failure has already
// been retried, broken and fallen back by the policy layer and only reaches
// here as a degraded capability the model is told to work around.
func (loop *Loop) invokeOne(ctx context.Context, loopState *guard.State, toolCall llm.ToolCall) llm.ToolResult {
	if loop.RepeatGuard != nil {
		disposition, payload := loop.RepeatGuard.Disposition(toolCall)
		switch disposition {
		case guard.RepeatServeCache:
			return llm.ToolResult{
				CallID: toolCall.ID, ToolName: toolCall.ToolName,
				Content: payload + "\n\n(cached: identical call already made in this phase)",
			}
		case guard.RepeatAdvise:
			// Deliberately IsError: this is a correctable situation, and the
			// model should read it as one.
			return llm.ToolResult{
				CallID: toolCall.ID, ToolName: toolCall.ToolName,
				Content: payload, IsError: true,
			}
		}
	}

	result, err := loop.Registry.Invoke(ctx, toolCall.ToolName, toolCall.Arguments)
	if err != nil {
		failureClass := tool.ClassOf(err)
		advice := tool.AdviceOf(err)

		if failureClass == tool.FailureDegradable {
			loop.Blackboard.NoteDegradedTool(toolCall.ToolName, advice)
		}
		return llm.ToolResult{
			CallID: toolCall.ID, ToolName: toolCall.ToolName,
			Content: advice, IsError: true,
		}
	}

	content := result.Content
	if loop.SpillThresholdBytes > 0 && len(content) > loop.SpillThresholdBytes {
		handle := loop.SpillStore.Put(content)
		content = fmt.Sprintf("%s\n\n[handle: %s]",
			memory.Digest(content, loop.SpillThresholdBytes/4), handle)
	}
	return llm.ToolResult{CallID: toolCall.ID, ToolName: toolCall.ToolName, Content: content}
}

func (loop *Loop) finish(outcome Outcome, loopState *guard.State, err error) (Outcome, error) {
	outcome.IterationCount = loopState.IterationCount
	outcome.ToolCallCount = loopState.ToolCallCount
	outcome.TokensUsed = loopState.TokensUsed
	outcome.Duration = loop.nowFunc().Sub(loopState.StartedAt)

	if err == nil {
		return outcome, nil
	}
	// A guard stop is a deliberate, orderly end: the caller keeps the partial
	// results and records why. Anything else is a real error.
	if stop, isStop := guard.IsStop(err); isStop {
		outcome.StoppedBy = stop
		return outcome, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		outcome.StoppedBy = &guard.Stop{
			Reason: guard.StopBudgetExhausted, Guard: "context", Detail: err.Error(),
		}
		return outcome, nil
	}
	return outcome, err
}
