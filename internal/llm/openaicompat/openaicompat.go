// Package openaicompat adapts any OpenAI-compatible chat-completions endpoint
// to llm.Provider.
//
// This is the reason the provider abstraction exists. The same adapter reaches
// OpenAI, Ollama, LM Studio, llama.cpp, vLLM, Together, Groq, OpenRouter and
// anything else speaking that wire format — including a model running on your
// own machine, which costs nothing per run.
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

// DefaultBaseURL is OpenAI's own endpoint.
const DefaultBaseURL = "https://api.openai.com/v1"

// LocalOllamaBaseURL is where Ollama serves its OpenAI-compatible API.
const LocalOllamaBaseURL = "http://localhost:11434/v1"

// ModelsByTier maps neutral tiers onto model identifiers.
type ModelsByTier map[llm.Tier]string

// SingleModel maps every tier onto one model, which is what a local runtime
// usually wants: there is no cheap tier when the marginal cost is zero.
func SingleModel(modelName string) ModelsByTier {
	return ModelsByTier{
		llm.TierFast: modelName, llm.TierBalanced: modelName, llm.TierStrong: modelName,
	}
}

// Provider is the OpenAI-compatible adapter.
type Provider struct {
	APIKey     string
	BaseURL    string
	Models     ModelsByTier
	HTTPClient *http.Client

	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Sleep       func(ctx context.Context, duration time.Duration) error
	Jitter      func() float64

	ContextWindowTokens int
	MaxOutputTokens     int
}

// New builds a provider against the given endpoint.
func New(apiKey, baseURL string, models ModelsByTier) *Provider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Provider{
		APIKey: apiKey, BaseURL: baseURL, Models: models,
		// A local model on modest hardware can take a long time per turn, so
		// this timeout is deliberately generous.
		HTTPClient:          &http.Client{Timeout: 300 * time.Second},
		MaxAttempts:         3,
		BaseBackoff:         500 * time.Millisecond,
		MaxBackoff:          16 * time.Second,
		ContextWindowTokens: 128000,
		MaxOutputTokens:     8192,
	}
}

func (provider *Provider) Name() string { return "openai-compatible" }

func (provider *Provider) Limits(tier llm.Tier) llm.Limits {
	contextWindow := provider.ContextWindowTokens
	if contextWindow <= 0 {
		contextWindow = 128000
	}
	maxOutput := provider.MaxOutputTokens
	if maxOutput <= 0 {
		maxOutput = 8192
	}
	return llm.Limits{ContextWindowTokens: contextWindow, MaxOutputTokens: maxOutput}
}

func (provider *Provider) CountTokens(ctx context.Context, request llm.Request) (int, error) {
	return llm.EstimateTokens(request), nil
}

// APIError is an error reported by the endpoint.
type APIError struct {
	StatusCode int
	Message    string
}

func (apiError *APIError) Error() string {
	return fmt.Sprintf("openai-compatible: %d: %s", apiError.StatusCode, apiError.Message)
}

// Retryable reports whether another attempt is worthwhile.
func (apiError *APIError) Retryable() bool {
	return apiError.StatusCode == http.StatusTooManyRequests ||
		apiError.StatusCode == http.StatusRequestTimeout ||
		apiError.StatusCode >= 500
}

// ---------------------------------------------------------------- wire ---

type wireRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Tools       []wireTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	// ToolCalls appear on assistant messages.
	ToolCalls []wireToolCall `json:"tool_calls,omitempty"`
	// ToolCallID appears on tool-role messages.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
		// Arguments is a JSON *string* here, not an object. This is the one
		// real difference from Anthropic's format and the usual source of
		// bugs when porting between the two.
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type wireResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string         `json:"content"`
			ToolCalls []wireToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Complete performs one completion, retrying transient failures.
func (provider *Provider) Complete(ctx context.Context, request llm.Request) (*llm.Response, error) {
	modelName, known := provider.Models[request.Tier]
	if !known || modelName == "" {
		return nil, fmt.Errorf("openai-compatible: no model configured for tier %q", request.Tier)
	}

	encodedBody, err := json.Marshal(provider.buildWireRequest(modelName, request))
	if err != nil {
		return nil, fmt.Errorf("openai-compatible: encode request: %w", err)
	}

	maxAttempts := provider.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attemptNumber := 1; attemptNumber <= maxAttempts; attemptNumber++ {
		response, retryAfter, err := provider.attemptOnce(ctx, encodedBody)
		if err == nil {
			return response, nil
		}
		lastErr = err

		var apiError *APIError
		if errors.As(err, &apiError) && !apiError.Retryable() {
			return nil, err
		}
		if attemptNumber == maxAttempts {
			break
		}
		if sleepErr := provider.sleep(ctx, provider.backoffFor(attemptNumber, retryAfter)); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, lastErr
}

func (provider *Provider) attemptOnce(ctx context.Context, encodedBody []byte) (*llm.Response, time.Duration, error) {
	endpoint := provider.BaseURL
	if endpoint == "" {
		endpoint = DefaultBaseURL
	}
	endpoint = trimTrailingSlash(endpoint) + "/chat/completions"

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encodedBody))
	if err != nil {
		return nil, 0, fmt.Errorf("openai-compatible: build request: %w", err)
	}
	httpRequest.Header.Set("content-type", "application/json")
	// A local runtime usually ignores the key, but sending it costs nothing and
	// keeps one code path for hosted and local endpoints alike.
	if provider.APIKey != "" {
		httpRequest.Header.Set("authorization", "Bearer "+provider.APIKey)
	}

	httpClient := provider.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return nil, 0, fmt.Errorf("openai-compatible: request failed: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("openai-compatible: read response: %w", err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		retryAfter := parseRetryAfter(httpResponse.Header.Get("retry-after"))
		return nil, retryAfter, apiErrorFrom(httpResponse.StatusCode, responseBody)
	}

	var decoded wireResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, 0, fmt.Errorf("openai-compatible: decode response: %w", err)
	}
	if decoded.Error != nil {
		return nil, 0, &APIError{StatusCode: httpResponse.StatusCode, Message: decoded.Error.Message}
	}
	if len(decoded.Choices) == 0 {
		return nil, 0, errors.New("openai-compatible: the response contained no choices")
	}
	return toNeutralResponse(decoded), 0, nil
}

func apiErrorFrom(statusCode int, responseBody []byte) error {
	apiError := &APIError{StatusCode: statusCode, Message: string(responseBody)}
	var decoded wireResponse
	if err := json.Unmarshal(responseBody, &decoded); err == nil && decoded.Error != nil {
		apiError.Message = decoded.Error.Message
	}
	return apiError
}

func (provider *Provider) buildWireRequest(modelName string, request llm.Request) wireRequest {
	built := wireRequest{
		Model:     modelName,
		Messages:  toWireMessages(request.System, request.Messages),
		MaxTokens: request.MaxTokens,
	}
	if request.Temperature > 0 {
		temperature := request.Temperature
		built.Temperature = &temperature
	}
	for _, toolSchema := range request.Tools {
		wrapped := wireTool{Type: "function"}
		wrapped.Function.Name = toolSchema.Name
		wrapped.Function.Description = toolSchema.Description
		wrapped.Function.Parameters = toolSchema.InputSchema
		built.Tools = append(built.Tools, wrapped)
	}
	return built
}

// toWireMessages translates the neutral history. Two shape differences from
// Anthropic matter: the system prompt is a message rather than a field, and a
// tool result is its own tool-role message rather than a block on a user turn.
func toWireMessages(systemPrompt string, messages []llm.Message) []wireMessage {
	wireMessages := []wireMessage{}
	if systemPrompt != "" {
		wireMessages = append(wireMessages, wireMessage{Role: "system", Content: systemPrompt})
	}

	for _, message := range messages {
		switch message.Role {
		case llm.RoleAssistant:
			if message.Text == "" && len(message.ToolCalls) == 0 {
				continue
			}
			assistantMessage := wireMessage{Role: "assistant", Content: message.Text}
			for _, toolCall := range message.ToolCalls {
				wrapped := wireToolCall{ID: toolCall.ID, Type: "function"}
				wrapped.Function.Name = toolCall.ToolName
				// Arguments must be a JSON string on this wire format.
				wrapped.Function.Arguments = string(toolCall.Arguments)
				assistantMessage.ToolCalls = append(assistantMessage.ToolCalls, wrapped)
			}
			wireMessages = append(wireMessages, assistantMessage)

		case llm.RoleTool:
			// Each result is its own message, unlike Anthropic where they are
			// blocks on a single user turn.
			for _, toolResult := range message.ToolResults {
				content := toolResult.Content
				if toolResult.IsError {
					// This format has no is_error flag, so the signal has to
					// live in the text or the model cannot tell a failure from
					// data.
					content = "ERROR: " + content
				}
				wireMessages = append(wireMessages, wireMessage{
					Role: "tool", ToolCallID: toolResult.CallID, Content: content,
				})
			}

		default:
			if message.Text == "" {
				continue
			}
			wireMessages = append(wireMessages, wireMessage{Role: "user", Content: message.Text})
		}
	}
	return wireMessages
}

func toNeutralResponse(decoded wireResponse) *llm.Response {
	choice := decoded.Choices[0]
	response := &llm.Response{
		Text:      choice.Message.Content,
		ModelName: decoded.Model,
		Usage: llm.Usage{
			InputTokens: decoded.Usage.PromptTokens, OutputTokens: decoded.Usage.CompletionTokens,
		},
	}
	for _, toolCall := range choice.Message.ToolCalls {
		arguments := json.RawMessage(toolCall.Function.Arguments)
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		response.ToolCalls = append(response.ToolCalls, llm.ToolCall{
			ID: toolCall.ID, ToolName: toolCall.Function.Name, Arguments: arguments,
		})
	}

	switch choice.FinishReason {
	case "tool_calls", "function_call":
		response.StopReason = llm.StopToolUse
	case "length":
		response.StopReason = llm.StopMaxTokens
	default:
		// Some local runtimes report "stop" even when they emitted tool calls,
		// so trust the payload over the label.
		if len(response.ToolCalls) > 0 {
			response.StopReason = llm.StopToolUse
		} else {
			response.StopReason = llm.StopEndTurn
		}
	}
	return response
}

// ---------------------------------------------------------- small parts ---

func (provider *Provider) sleep(ctx context.Context, duration time.Duration) error {
	if provider.Sleep != nil {
		return provider.Sleep(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (provider *Provider) backoffFor(attemptNumber int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	baseBackoff := provider.BaseBackoff
	if baseBackoff <= 0 {
		baseBackoff = 500 * time.Millisecond
	}
	maxBackoff := provider.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 16 * time.Second
	}
	backoff := baseBackoff << (attemptNumber - 1)
	if backoff > maxBackoff || backoff <= 0 {
		backoff = maxBackoff
	}
	jitter := provider.Jitter
	if jitter == nil {
		jitter = rand.Float64
	}
	return time.Duration(float64(backoff) * jitter())
}

func parseRetryAfter(headerValue string) time.Duration {
	if headerValue == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(headerValue); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func trimTrailingSlash(value string) string {
	for len(value) > 0 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}
