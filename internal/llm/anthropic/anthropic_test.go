package anthropic

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

// newTestProvider points a provider at a test server and removes the waiting
// from retries so the suite stays fast.
func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	provider := New("test-key")
	provider.Endpoint = testServer.URL
	provider.HTTPClient = testServer.Client()
	provider.Sleep = func(context.Context, time.Duration) error { return nil }
	provider.Jitter = func() float64 { return 1 }
	return provider
}

func TestCompleteTranslatesToolUseInBothDirections(t *testing.T) {
	var capturedBody map[string]any
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		bodyBytes, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(bodyBytes, &capturedBody)

		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{
			"model":"claude-sonnet-4-5",
			"stop_reason":"tool_use",
			"content":[
				{"type":"text","text":"Reading the file."},
				{"type":"tool_use","id":"tu_1","name":"repo.read_file","input":{"path":"a.go"}}
			],
			"usage":{"input_tokens":120,"output_tokens":45}
		}`))
	})

	response, err := provider.Complete(context.Background(), llm.Request{
		Tier:   llm.TierBalanced,
		System: "you are an analyst",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "Analyse a.go"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
				{ID: "tu_0", ToolName: "repo.tree", Arguments: json.RawMessage(`{}`)},
			}},
			{Role: llm.RoleTool, ToolResults: []llm.ToolResult{
				{CallID: "tu_0", ToolName: "repo.tree", Content: "a.go"},
			}},
		},
		Tools: []llm.ToolSchema{{
			Name: "repo.read_file", Description: "read a file",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Outbound: the tier must resolve to a model, and a tool result must become
	// a user-role block, which is the one asymmetry in this API.
	if capturedBody["model"] != "claude-sonnet-4-5" {
		t.Errorf("tier should resolve to a model, got %v", capturedBody["model"])
	}
	wireMessages, _ := capturedBody["messages"].([]any)
	if len(wireMessages) != 3 {
		t.Fatalf("expected 3 wire messages, got %d", len(wireMessages))
	}
	lastMessage, _ := wireMessages[2].(map[string]any)
	if lastMessage["role"] != "user" {
		t.Errorf("a tool result must be sent as a user-role message, got %v", lastMessage["role"])
	}

	// Inbound: text and tool calls must both survive translation.
	if response.Text != "Reading the file." {
		t.Errorf("unexpected text %q", response.Text)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].ToolName != "repo.read_file" {
		t.Fatalf("tool call did not survive translation: %+v", response.ToolCalls)
	}
	if response.StopReason != llm.StopToolUse {
		t.Errorf("expected a tool_use stop reason, got %q", response.StopReason)
	}
	if response.Usage.Total() != 165 {
		t.Errorf("usage must be reported so budgets are enforced on real numbers, got %d",
			response.Usage.Total())
	}
}

func TestCompleteRetriesRateLimitAndHonoursRetryAfter(t *testing.T) {
	attemptCount := 0
	var observedBackoff time.Duration

	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			responseWriter.Header().Set("retry-after", "7")
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			_, _ = responseWriter.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
			return
		}
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"model":"m","stop_reason":"end_turn",
			"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	})
	provider.Sleep = func(_ context.Context, duration time.Duration) error {
		observedBackoff = duration
		return nil
	}

	response, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("a rate limit should be retried, got %v", err)
	}
	if response.Text != "ok" || attemptCount != 2 {
		t.Fatalf("expected recovery on the second attempt, got %d attempts", attemptCount)
	}
	// Honouring the server's own instruction beats guessing a backoff.
	if observedBackoff != 7*time.Second {
		t.Fatalf("expected the Retry-After value to be used, got %s", observedBackoff)
	}
}

func TestCompleteDoesNotRetryABadRequest(t *testing.T) {
	// A 400 will be a 400 next time too. Retrying it wastes the budget and
	// delays the real error reaching the caller.
	attemptCount := 0
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		attemptCount++
		responseWriter.WriteHeader(http.StatusBadRequest)
		_, _ = responseWriter.Write([]byte(`{"error":{"type":"invalid_request_error","message":"bad tool schema"}}`))
	})

	_, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err == nil {
		t.Fatal("expected the bad request to surface")
	}
	if attemptCount != 1 {
		t.Fatalf("a non-retryable status must be attempted once, got %d", attemptCount)
	}
	if !strings.Contains(err.Error(), "bad tool schema") {
		t.Fatalf("the API's own message must reach the caller, got %v", err)
	}
}

func TestCompleteRetriesServerErrorsThenGivesUp(t *testing.T) {
	attemptCount := 0
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		attemptCount++
		responseWriter.WriteHeader(http.StatusBadGateway)
	})

	if _, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err == nil {
		t.Fatal("expected the failure to surface after retries are exhausted")
	}
	if attemptCount != provider.MaxAttempts {
		t.Fatalf("expected %d attempts, got %d", provider.MaxAttempts, attemptCount)
	}
}

func TestCompleteRefusesWithoutAnAPIKey(t *testing.T) {
	provider := New("")
	if _, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err != ErrNoAPIKey {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}

func TestCompleteRefusesAnUnmappedTier(t *testing.T) {
	provider := New("k")
	provider.Models = ModelsByTier{llm.TierFast: "m"}
	_, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierStrong})
	if err == nil || !strings.Contains(err.Error(), "no model configured") {
		t.Fatalf("an unmapped tier must fail loudly at the call site, got %v", err)
	}
}

func TestEveryTierHasADefaultModel(t *testing.T) {
	defaults := DefaultModels()
	for _, tier := range []llm.Tier{llm.TierFast, llm.TierBalanced, llm.TierStrong} {
		if defaults[tier] == "" {
			t.Errorf("tier %q has no default model, so any agent using it would fail at runtime", tier)
		}
	}
}

func TestEmptyAssistantMessagesAreDropped(t *testing.T) {
	// The API rejects a message with no content blocks, and an assistant turn
	// with neither text nor tool calls is easy to produce when a loop ends.
	wireMessages := toWireMessages([]llm.Message{
		{Role: llm.RoleAssistant, Text: "", ToolCalls: nil},
		{Role: llm.RoleUser, Text: "hello"},
	})
	if len(wireMessages) != 1 || wireMessages[0].Role != "user" {
		t.Fatalf("an empty assistant turn must be dropped, got %+v", wireMessages)
	}
}
