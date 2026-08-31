package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	provider := New("test-key", testServer.URL, SingleModel("test-model"))
	provider.HTTPClient = testServer.Client()
	provider.Sleep = func(context.Context, time.Duration) error { return nil }
	provider.Jitter = func() float64 { return 1 }
	return provider
}

func TestToolCallArgumentsCrossTheStringBoundaryBothWays(t *testing.T) {
	// This format carries tool arguments as a JSON *string*, not an object.
	// Getting it wrong in either direction silently breaks every agent.
	var capturedBody map[string]any
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		bodyBytes, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(bodyBytes, &capturedBody)

		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
			"model":"test-model",
			"choices":[{"message":{"content":"reading it",
				"tool_calls":[{"id":"call_1","type":"function",
					"function":{"name":"repo.read_file","arguments":"{\"path\":\"a.go\"}"}}]},
				"finish_reason":"tool_calls"}],
			"usage":{"prompt_tokens":80,"completion_tokens":20}}`))
	})

	response, err := provider.Complete(context.Background(), llm.Request{
		Tier:   llm.TierBalanced,
		System: "you are an analyst",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "analyse a.go"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
				{ID: "call_0", ToolName: "repo.tree", Arguments: json.RawMessage(`{"depth":2}`)},
			}},
			{Role: llm.RoleTool, ToolResults: []llm.ToolResult{
				{CallID: "call_0", ToolName: "repo.tree", Content: "a.go"},
			}},
		},
		Tools: []llm.ToolSchema{{
			Name: "repo.read_file", Description: "read", InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Outbound: arguments must have been serialised as a string.
	messages, _ := capturedBody["messages"].([]any)
	assistantMessage, _ := messages[2].(map[string]any)
	toolCalls, _ := assistantMessage["tool_calls"].([]any)
	firstCall, _ := toolCalls[0].(map[string]any)
	function, _ := firstCall["function"].(map[string]any)
	if _, isString := function["arguments"].(string); !isString {
		t.Fatalf("arguments must be sent as a JSON string, got %T", function["arguments"])
	}

	// Inbound: the string must have been parsed back into raw JSON.
	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(response.ToolCalls))
	}
	var parsedArguments struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(response.ToolCalls[0].Arguments, &parsedArguments); err != nil {
		t.Fatalf("returned arguments are not valid JSON: %v", err)
	}
	if parsedArguments.Path != "a.go" {
		t.Fatalf("unexpected arguments %s", response.ToolCalls[0].Arguments)
	}
	if response.StopReason != llm.StopToolUse {
		t.Errorf("expected a tool-use stop, got %q", response.StopReason)
	}
}

func TestSystemPromptBecomesTheFirstMessage(t *testing.T) {
	// Unlike Anthropic, the system prompt is a message here rather than a field.
	wireMessages := toWireMessages("you are an analyst", []llm.Message{
		{Role: llm.RoleUser, Text: "hello"},
	})
	if len(wireMessages) != 2 || wireMessages[0].Role != "system" {
		t.Fatalf("the system prompt must lead the message list, got %+v", wireMessages)
	}
}

func TestEachToolResultBecomesItsOwnMessage(t *testing.T) {
	// Anthropic groups results as blocks on one user turn; this format needs
	// one tool-role message per result, each carrying its call id.
	wireMessages := toWireMessages("", []llm.Message{
		{Role: llm.RoleTool, ToolResults: []llm.ToolResult{
			{CallID: "a", Content: "first"},
			{CallID: "b", Content: "second"},
		}},
	})
	if len(wireMessages) != 2 {
		t.Fatalf("expected two tool messages, got %d", len(wireMessages))
	}
	for index, expectedID := range []string{"a", "b"} {
		if wireMessages[index].Role != "tool" || wireMessages[index].ToolCallID != expectedID {
			t.Errorf("message %d: expected tool role with id %q, got %+v", index, expectedID, wireMessages[index])
		}
	}
}

func TestToolErrorsAreMarkedInTheContent(t *testing.T) {
	// This format has no is_error flag, so the signal must survive in the text
	// or the model cannot tell a failure from data.
	wireMessages := toWireMessages("", []llm.Message{
		{Role: llm.RoleTool, ToolResults: []llm.ToolResult{
			{CallID: "a", Content: "no such path", IsError: true},
		}},
	})
	if !strings.HasPrefix(wireMessages[0].Content, "ERROR:") {
		t.Fatalf("a failed tool result must be distinguishable, got %q", wireMessages[0].Content)
	}
}

func TestToolCallsWithoutAToolCallsFinishReasonAreStillHonoured(t *testing.T) {
	// Several local runtimes report "stop" even when they emitted tool calls.
	// Trusting the label over the payload would strand the agent loop.
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
			"model":"local","choices":[{"message":{"content":"",
				"tool_calls":[{"id":"c1","type":"function",
					"function":{"name":"repo.tree","arguments":"{}"}}]},
				"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	})

	response, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.StopReason != llm.StopToolUse {
		t.Fatalf("a response carrying tool calls must stop for tool use, got %q", response.StopReason)
	}
}

func TestEmptyArgumentsBecomeAnEmptyObject(t *testing.T) {
	// A runtime that omits arguments entirely would otherwise produce invalid
	// JSON downstream.
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
			"model":"local","choices":[{"message":{
				"tool_calls":[{"id":"c1","type":"function","function":{"name":"repo.tree","arguments":""}}]},
				"finish_reason":"tool_calls"}]}`))
	})

	response, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(response.ToolCalls[0].Arguments) != "{}" {
		t.Fatalf("expected an empty object, got %q", response.ToolCalls[0].Arguments)
	}
}

func TestRetriesRateLimitButNotBadRequest(t *testing.T) {
	rateLimitAttempts := 0
	rateLimited := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		rateLimitAttempts++
		if rateLimitAttempts == 1 {
			responseWriter.Header().Set("retry-after", "3")
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			return
		}
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},
			"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	})
	if _, err := rateLimited.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err != nil {
		t.Fatalf("a rate limit should be retried: %v", err)
	}
	if rateLimitAttempts != 2 {
		t.Fatalf("expected recovery on the second attempt, got %d", rateLimitAttempts)
	}

	badRequestAttempts := 0
	badRequest := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		badRequestAttempts++
		responseWriter.WriteHeader(http.StatusBadRequest)
		_, _ = responseWriter.Write([]byte(`{"error":{"message":"unknown model"}}`))
	})
	_, err := badRequest.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err == nil || !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("the endpoint's own message must reach the caller, got %v", err)
	}
	if badRequestAttempts != 1 {
		t.Fatalf("a bad request must not be retried, got %d attempts", badRequestAttempts)
	}
}

func TestBaseURLTrailingSlashIsTolerated(t *testing.T) {
	// People paste base URLs with and without a trailing slash; both must work.
	var observedPath string
	testServer := httptest.NewServer(http.HandlerFunc(
		func(responseWriter http.ResponseWriter, request *http.Request) {
			observedPath = request.URL.Path
			responseWriter.Header().Set("content-type", "application/json")
			_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},
				"finish_reason":"stop"}]}`))
		}))
	defer testServer.Close()

	provider := New("", testServer.URL+"/", SingleModel("m"))
	provider.HTTPClient = testServer.Client()
	if _, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observedPath != "/chat/completions" {
		t.Fatalf("unexpected path %q", observedPath)
	}
}

func TestNoAuthorizationHeaderWhenNoKeyIsSet(t *testing.T) {
	// A local runtime needs no key, and sending an empty bearer token upsets
	// some of them.
	var sawAuthorization bool
	testServer := httptest.NewServer(http.HandlerFunc(
		func(responseWriter http.ResponseWriter, request *http.Request) {
			sawAuthorization = request.Header.Get("authorization") != ""
			responseWriter.Header().Set("content-type", "application/json")
			_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},
				"finish_reason":"stop"}]}`))
		}))
	defer testServer.Close()

	provider := New("", testServer.URL, SingleModel("m"))
	provider.HTTPClient = testServer.Client()
	if _, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawAuthorization {
		t.Fatal("no authorization header should be sent when no key is configured")
	}
}

func TestSingleModelCoversEveryTier(t *testing.T) {
	// A local runtime has no cheap tier: the marginal cost is zero, so one
	// model serves all three.
	models := SingleModel("qwen2.5-coder")
	for _, tier := range []llm.Tier{llm.TierFast, llm.TierBalanced, llm.TierStrong} {
		if models[tier] != "qwen2.5-coder" {
			t.Errorf("tier %q is unmapped, so any agent using it would fail", tier)
		}
	}
}
