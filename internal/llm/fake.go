package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ScriptedTurn is one response the fake provider will hand back, in order.
type ScriptedTurn struct {
	// MatchSubstring, when set, asserts the request contains this text. An
	// empty value matches anything.
	MatchSubstring string
	Response       Response
	// Err, when set, is returned instead of the response. Used to exercise
	// retry, fallback and budget paths without touching a network.
	Err error
}

// RecordedCall is what the fake saw, kept so tests can assert on the prompts
// and on how many turns an agent actually needed.
type RecordedCall struct {
	Request Request
	Turn    int
}

// FakeProvider is a scriptable Provider. Every test in this repository runs
// against it, which is the point of having the abstraction at all: the entire
// control layer is exercisable with no API key and no network.
type FakeProvider struct {
	mutex         sync.Mutex
	scriptedTurns []ScriptedTurn
	nextTurnIndex int
	recordedCalls []RecordedCall

	// DefaultResponse is returned once the script is exhausted, if set. Leaving
	// it nil makes over-running the script a loud test failure instead of a
	// quiet one.
	DefaultResponse *Response
}

// NewFakeProvider builds a provider that returns the given turns in order.
func NewFakeProvider(scriptedTurns ...ScriptedTurn) *FakeProvider {
	return &FakeProvider{scriptedTurns: scriptedTurns}
}

func (fakeProvider *FakeProvider) Name() string { return "fake" }

func (fakeProvider *FakeProvider) Limits(tier Tier) Limits {
	return Limits{ContextWindowTokens: 8000, MaxOutputTokens: 2000}
}

func (fakeProvider *FakeProvider) CountTokens(ctx context.Context, request Request) (int, error) {
	return EstimateTokens(request), nil
}

func (fakeProvider *FakeProvider) Complete(ctx context.Context, request Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fakeProvider.mutex.Lock()
	defer fakeProvider.mutex.Unlock()

	currentTurn := fakeProvider.nextTurnIndex
	fakeProvider.recordedCalls = append(fakeProvider.recordedCalls, RecordedCall{Request: request, Turn: currentTurn})

	if currentTurn >= len(fakeProvider.scriptedTurns) {
		if fakeProvider.DefaultResponse != nil {
			responseCopy := *fakeProvider.DefaultResponse
			fakeProvider.nextTurnIndex++
			return &responseCopy, nil
		}
		return nil, fmt.Errorf("%w (turn %d, %s)", ErrNoScriptedResponse, currentTurn, DescribeRequest(request))
	}

	scriptedTurn := fakeProvider.scriptedTurns[currentTurn]
	fakeProvider.nextTurnIndex++

	if scriptedTurn.MatchSubstring != "" && !requestContains(request, scriptedTurn.MatchSubstring) {
		return nil, fmt.Errorf("llm: turn %d expected request containing %q, got %s",
			currentTurn, scriptedTurn.MatchSubstring, DescribeRequest(request))
	}
	if scriptedTurn.Err != nil {
		return nil, scriptedTurn.Err
	}

	responseCopy := scriptedTurn.Response
	if responseCopy.ModelName == "" {
		responseCopy.ModelName = "fake-" + string(request.Tier)
	}
	if responseCopy.StopReason == "" {
		if len(responseCopy.ToolCalls) > 0 {
			responseCopy.StopReason = StopToolUse
		} else {
			responseCopy.StopReason = StopEndTurn
		}
	}
	if responseCopy.Usage.Total() == 0 {
		responseCopy.Usage = Usage{InputTokens: EstimateTokens(request), OutputTokens: len(responseCopy.Text) / 4}
	}
	return &responseCopy, nil
}

// RecordedCalls returns a copy of everything the fake was asked.
func (fakeProvider *FakeProvider) RecordedCalls() []RecordedCall {
	fakeProvider.mutex.Lock()
	defer fakeProvider.mutex.Unlock()
	callsCopy := make([]RecordedCall, len(fakeProvider.recordedCalls))
	copy(callsCopy, fakeProvider.recordedCalls)
	return callsCopy
}

// CallCount reports how many completions were requested.
func (fakeProvider *FakeProvider) CallCount() int {
	fakeProvider.mutex.Lock()
	defer fakeProvider.mutex.Unlock()
	return len(fakeProvider.recordedCalls)
}

func requestContains(request Request, needle string) bool {
	if strings.Contains(request.System, needle) {
		return true
	}
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

// TextTurn is a convenience constructor for a plain text response.
func TextTurn(text string) ScriptedTurn {
	return ScriptedTurn{Response: Response{Text: text, StopReason: StopEndTurn}}
}

// ToolCallTurn is a convenience constructor for a response asking for one tool.
func ToolCallTurn(callID, toolName string, arguments any) ScriptedTurn {
	encodedArguments, err := json.Marshal(arguments)
	if err != nil {
		panic("llm: ToolCallTurn arguments must marshal: " + err.Error())
	}
	return ScriptedTurn{
		Response: Response{
			ToolCalls:  []ToolCall{{ID: callID, ToolName: toolName, Arguments: encodedArguments}},
			StopReason: StopToolUse,
		},
	}
}
