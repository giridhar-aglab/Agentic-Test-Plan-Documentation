package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

// legalFunctionName is the pattern OpenAI enforces on tool names. A name that
// violates it is rejected with a 400 before the model is ever consulted, so the
// test server enforces it too — a stub that accepts anything let a real bug
// (every tool in this system is named with a dot) reach production.
var legalFunctionName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// systemToolNames are the real tool names this system registers. They are
// underscore-only *because* of this constraint: the names used to contain dots,
// which OpenAI rejects outright, and sanitising them on the wire silently made
// the model's tool list disagree with the tool names written into the prompts.
var systemToolNames = []string{
	"repo_tree", "repo_read_file", "repo_commits", "code_parse_go",
	"analysis_emit_component", "scenario_emit", "review_emit", "jira_push",
	"report_validate",
}

// enforceFunctionNamePattern wraps a handler so any request carrying an illegal
// function name — in the tool catalogue or in replayed history — fails the way
// the real endpoint fails, rather than passing silently.
func enforceFunctionNamePattern(t *testing.T, handler http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		bodyBytes, _ := io.ReadAll(request.Body)
		request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		var decoded struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
			Messages []struct {
				ToolCalls []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(bodyBytes, &decoded)

		reject := func(field, name string) {
			responseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = responseWriter.Write([]byte(`{"error":{"message":"Invalid '` + field +
				`': string does not match pattern '^[a-zA-Z0-9_-]+$'. Got: ` + name + `"}}`))
		}
		for index, wrapped := range decoded.Tools {
			if !legalFunctionName.MatchString(wrapped.Function.Name) {
				reject("tools["+strconv.Itoa(index)+"].function.name", wrapped.Function.Name)
				return
			}
		}
		for _, message := range decoded.Messages {
			for _, toolCall := range message.ToolCalls {
				if !legalFunctionName.MatchString(toolCall.Function.Name) {
					reject("messages[].tool_calls[].function.name", toolCall.Function.Name)
					return
				}
			}
		}
		handler(responseWriter, request)
	}
}

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	testServer := httptest.NewServer(enforceFunctionNamePattern(t, handler))
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
					"function":{"name":"repo_read_file","arguments":"{\"path\":\"a.go\"}"}}]},
				"finish_reason":"tool_calls"}],
			"usage":{"prompt_tokens":80,"completion_tokens":20}}`))
	})

	response, err := provider.Complete(context.Background(), llm.Request{
		Tier:   llm.TierBalanced,
		System: "you are an analyst",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "analyse a.go"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
				{ID: "call_0", ToolName: "repo_tree", Arguments: json.RawMessage(`{"depth":2}`)},
			}},
			{Role: llm.RoleTool, ToolResults: []llm.ToolResult{
				{CallID: "call_0", ToolName: "repo_tree", Content: "a.go"},
			}},
		},
		Tools: []llm.ToolSchema{{
			Name: "repo_read_file", Description: "read", InputSchema: json.RawMessage(`{"type":"object"}`),
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
	if response.ToolCalls[0].ToolName != "repo_read_file" {
		t.Fatalf("the wire name must map back to the registered tool name, got %q",
			response.ToolCalls[0].ToolName)
	}
	if response.StopReason != llm.StopToolUse {
		t.Errorf("expected a tool-use stop, got %q", response.StopReason)
	}
}

func TestSystemPromptBecomesTheFirstMessage(t *testing.T) {
	// Unlike Anthropic, the system prompt is a message here rather than a field.
	wireMessages := toWireMessages("you are an analyst", []llm.Message{
		{Role: llm.RoleUser, Text: "hello"},
	}, toolNameMapping{})
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
	}, toolNameMapping{})
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
	}, toolNameMapping{})
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
					"function":{"name":"repo_tree","arguments":"{}"}}]},
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
				"tool_calls":[{"id":"c1","type":"function","function":{"name":"repo_tree","arguments":""}}]},
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

func TestEverySystemToolNameSurvivesTheWireNamePattern(t *testing.T) {
	// Every tool in this system is named with a dot. OpenAI rejects that with a
	// 400 before the model is consulted, so the whole catalogue has to be
	// translated — and the tools sent must all be legal, all be distinct, and
	// all map back to what the registry knows.
	tools := make([]llm.ToolSchema, 0, len(systemToolNames))
	for _, toolName := range systemToolNames {
		tools = append(tools, llm.ToolSchema{
			Name: toolName, Description: "d", InputSchema: json.RawMessage(`{"type":"object"}`),
		})
	}

	var capturedBody map[string]any
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		bodyBytes, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(bodyBytes, &capturedBody)
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},
			"finish_reason":"stop"}]}`))
	})

	if _, err := provider.Complete(context.Background(), llm.Request{
		Tier: llm.TierBalanced, Tools: tools,
	}); err != nil {
		t.Fatalf("the real tool catalogue must be accepted by the endpoint: %v", err)
	}

	sentTools, _ := capturedBody["tools"].([]any)
	if len(sentTools) != len(systemToolNames) {
		t.Fatalf("expected %d tools on the wire, got %d", len(systemToolNames), len(sentTools))
	}
	seen := map[string]bool{}
	mapping := buildToolNameMapping(tools)
	for _, entry := range sentTools {
		wrapped, _ := entry.(map[string]any)
		function, _ := wrapped["function"].(map[string]any)
		wireName, _ := function["name"].(string)

		if !legalFunctionName.MatchString(wireName) {
			t.Errorf("wire tool name %q violates the endpoint's pattern", wireName)
		}
		if seen[wireName] {
			t.Errorf("wire tool name %q was emitted twice, so a returned call is ambiguous", wireName)
		}
		seen[wireName] = true
		if realName := mapping.realName(wireName); !containsString(systemToolNames, realName) {
			t.Errorf("wire name %q maps back to %q, which is not a registered tool", wireName, realName)
		}
	}
}

func TestTheRealToolNamesCrossTheWireUnchanged(t *testing.T) {
	// The prompts name these tools in prose. If the adapter rewrites any of
	// them, the model is told to call something that is not in its tool list —
	// which is exactly how a whole run gets spent on eight futile iterations
	// per file. Translation must be a no-op for every name this system uses.
	tools := make([]llm.ToolSchema, 0, len(systemToolNames))
	for _, toolName := range systemToolNames {
		tools = append(tools, llm.ToolSchema{Name: toolName})
	}
	mapping := buildToolNameMapping(tools)
	for _, toolName := range systemToolNames {
		if mapping.realToWire[toolName] != toolName {
			t.Errorf("%q is rewritten to %q on the wire; the prompts would name a tool "+
				"the model was never offered", toolName, mapping.realToWire[toolName])
		}
	}
}

func TestCollidingToolNamesGetDistinctWireNames(t *testing.T) {
	// "repo_tree" and "repo-tree" both sanitise to "repo_tree". Collapsing them
	// would make the model's choice unrecoverable on the way back.
	mapping := buildToolNameMapping([]llm.ToolSchema{
		{Name: "repo_tree"}, {Name: "repo-tree"}, {Name: "repo/tree"},
	})
	if mapping.realToWire["repo_tree"] == mapping.realToWire["repo/tree"] {
		t.Fatalf("colliding names shared a wire name: %v", mapping.realToWire)
	}
	for realName, wireName := range mapping.realToWire {
		if !legalFunctionName.MatchString(wireName) {
			t.Errorf("%q produced an illegal wire name %q", realName, wireName)
		}
		if mapping.realName(wireName) != realName {
			t.Errorf("%q did not round trip: %q -> %q", realName, wireName, mapping.realName(wireName))
		}
	}
}

func TestReplayedHistoryUsesTheSameWireNames(t *testing.T) {
	// The model must never see a name in its own past turns that differs from
	// the name it was offered — that is how a loop starts guessing.
	mapping := buildToolNameMapping([]llm.ToolSchema{{Name: "analysis_emit_component"}})
	wireMessages := toWireMessages("", []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "c1", ToolName: "analysis_emit_component", Arguments: json.RawMessage(`{}`)},
		}},
	}, mapping)

	replayedName := wireMessages[0].ToolCalls[0].Function.Name
	if replayedName != mapping.realToWire["analysis_emit_component"] {
		t.Fatalf("history replay used %q but the catalogue offered %q",
			replayedName, mapping.realToWire["analysis_emit_component"])
	}
	if !legalFunctionName.MatchString(replayedName) {
		t.Fatalf("replayed name %q violates the endpoint's pattern", replayedName)
	}
}

func TestAnUnknownReturnedNameIsPassedThroughUntouched(t *testing.T) {
	// A model that invents a tool should reach the registry, which rejects it as
	// a correctable error the model can act on. Swallowing it here would hide
	// the mistake from the only component able to explain it.
	mapping := buildToolNameMapping([]llm.ToolSchema{{Name: "repo_tree"}})
	if got := mapping.realName("repo_invented"); got != "repo_invented" {
		t.Fatalf("an unknown name must pass through, got %q", got)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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

func TestAStatedWaitInTheBodyIsHonoured(t *testing.T) {
	// OpenAI does not send Retry-After for tokens-per-minute limits — it puts
	// the wait in the message. A client reading only the header backed off for
	// half a second when the server asked for fourteen, exhausted its attempts,
	// and lost the agent. Most of one real run's risks produced nothing this
	// way.
	var observedBackoff time.Duration
	attemptCount := 0
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			_, _ = responseWriter.Write([]byte(`{"error":{"message":"Rate limit reached for gpt-4o on tokens per min (TPM): Limit 30000, Used 30000, Requested 707. Please try again in 13.94s."}}`))
			return
		}
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},
			"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	})
	provider.Sleep = func(_ context.Context, duration time.Duration) error {
		observedBackoff = duration
		return nil
	}

	response, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err != nil {
		t.Fatalf("a stated wait should be waited out, not failed: %v", err)
	}
	if response.Text != "ok" {
		t.Fatalf("expected recovery, got %q", response.Text)
	}
	if observedBackoff < 13*time.Second {
		t.Fatalf("the server asked for 13.94s; backed off %s instead", observedBackoff)
	}
}

func TestAStatedWaitInMillisecondsIsHonoured(t *testing.T) {
	var observedBackoff time.Duration
	attemptCount := 0
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			_, _ = responseWriter.Write([]byte(`{"error":{"message":"Rate limit reached. Please try again in 686ms."}}`))
			return
		}
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	})
	provider.Sleep = func(_ context.Context, duration time.Duration) error {
		observedBackoff = duration
		return nil
	}

	if _, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observedBackoff < 686*time.Millisecond || observedBackoff > 3*time.Second {
		t.Fatalf("expected roughly 686ms, got %s", observedBackoff)
	}
}

func TestTheHeaderStillWinsWhenTheServerSendsOne(t *testing.T) {
	// Retry-After is the documented mechanism and must keep taking precedence.
	if delay := retryDelayFor("7", &APIError{Message: "Please try again in 2s."}); delay != 7*time.Second {
		t.Fatalf("expected the header to win, got %s", delay)
	}
}

func TestRateLimitsGetEnoughAttemptsToOutlastThem(t *testing.T) {
	// Three short attempts is not enough for a limit that resets on a minute
	// boundary.
	provider := New("k", "http://example.invalid", SingleModel("m"))
	if provider.MaxAttempts < 5 {
		t.Errorf("a rate-limited endpoint needs more than %d attempts", provider.MaxAttempts)
	}
	if provider.MaxBackoff < 60*time.Second {
		t.Errorf("max backoff %s is shorter than a rate-limit window", provider.MaxBackoff)
	}
}

func TestARateLimitPausesEveryConcurrentCaller(t *testing.T) {
	// Retrying one request is not enough when several agents share an
	// organisation's quota. Each retries independently, they collide again,
	// and the requests waiting on the limit are what re-trigger it — which is
	// how four risks were still lost after per-request retry was fixed.
	//
	// A rate limit belongs to the account, so the pause must be shared.
	var requestMutex sync.Mutex
	var requestTimes []time.Time
	rateLimitSent := false

	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		requestMutex.Lock()
		requestTimes = append(requestTimes, time.Now())
		first := !rateLimitSent
		rateLimitSent = true
		requestMutex.Unlock()

		if first {
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			_, _ = responseWriter.Write([]byte(
				`{"error":{"message":"Rate limit reached. Please try again in 5s."}}`))
			return
		}
		responseWriter.Header().Set("content-type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"model":"m","choices":[{"message":{"content":"ok"},
			"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	})

	// Record what the provider asked to sleep for, per caller.
	var sleepMutex sync.Mutex
	var sleeps []time.Duration
	provider.Sleep = func(_ context.Context, duration time.Duration) error {
		sleepMutex.Lock()
		sleeps = append(sleeps, duration)
		sleepMutex.Unlock()
		return nil
	}

	// One caller trips the limit, then others arrive.
	if _, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast}); err != nil {
		t.Fatalf("the first caller should recover: %v", err)
	}

	var waitGroup sync.WaitGroup
	for range 3 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _ = provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
		}()
	}
	waitGroup.Wait()

	sleepMutex.Lock()
	defer sleepMutex.Unlock()
	// The later callers must have been held back by the shared cooldown, not
	// allowed straight through to collide again.
	heldBack := 0
	for _, duration := range sleeps {
		if duration > 0 {
			heldBack++
		}
	}
	if heldBack < 2 {
		t.Fatalf("a rate limit must pause the other callers too; only %d of %d waited",
			heldBack, len(sleeps))
	}
}

func TestAnExhaustedRateLimitNamesTheLever(t *testing.T) {
	// "429" on its own leaves a reader with nothing to change.
	provider := newTestProvider(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusTooManyRequests)
		_, _ = responseWriter.Write([]byte(`{"error":{"message":"Rate limit reached."}}`))
	})
	provider.MaxAttempts = 2

	_, err := provider.Complete(context.Background(), llm.Request{Tier: llm.TierFast})
	if err == nil {
		t.Fatal("expected the rate limit to surface")
	}
	if !strings.Contains(err.Error(), "-concurrency") {
		t.Errorf("the error should name what to change, got %v", err)
	}
}
