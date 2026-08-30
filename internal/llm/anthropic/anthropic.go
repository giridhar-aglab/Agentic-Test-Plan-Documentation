// Package anthropic adapts the Anthropic Messages API to llm.Provider.
//
// It uses net/http directly rather than a vendor SDK, which keeps the whole
// system a single dependency-free binary and keeps the translation between the
// neutral request shape and the wire format visible in one file.
package anthropic

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

// DefaultEndpoint is the Messages API.
const DefaultEndpoint = "https://api.anthropic.com/v1/messages"

// DefaultAPIVersion is the required anthropic-version header.
const DefaultAPIVersion = "2023-06-01"

// ModelsByTier maps the neutral tiers onto concrete models. Agents ask for a
// tier, so swapping a model is a config change and never a code change.
type ModelsByTier map[llm.Tier]string

// DefaultModels is the shipped mapping. Cheap deterministic phases and
// expensive reasoning phases route independently, which is most of the cost
// control in the system.
func DefaultModels() ModelsByTier {
	return ModelsByTier{
		llm.TierFast:     "claude-haiku-4-5",
		llm.TierBalanced: "claude-sonnet-4-5",
		llm.TierStrong:   "claude-opus-4-5",
	}
}

// Provider is the Anthropic adapter.
type Provider struct {
	APIKey     string
	Endpoint   string
	APIVersion string
	Models     ModelsByTier
	HTTPClient *http.Client

	// MaxAttempts bounds retries of a single completion. Retry lives here
	// rather than in the agent loop because only the adapter can tell a
	// rate-limit from a bad request.
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Sleep       func(ctx context.Context, duration time.Duration) error
	Jitter      func() float64

	// ContextWindowTokens describes the models' envelope for the working set.
	ContextWindowTokens int
}

// New builds a provider with sensible defaults.
func New(apiKey string) *Provider {
	return &Provider{
		APIKey:              apiKey,
		Endpoint:            DefaultEndpoint,
		APIVersion:          DefaultAPIVersion,
		Models:              DefaultModels(),
		HTTPClient:          &http.Client{Timeout: 120 * time.Second},
		MaxAttempts:         3,
		BaseBackoff:         500 * time.Millisecond,
		MaxBackoff:          16 * time.Second,
		ContextWindowTokens: 180000,
	}
}

func (provider *Provider) Name() string { return "anthropic" }

func (provider *Provider) Limits(tier llm.Tier) llm.Limits {
	contextWindow := provider.ContextWindowTokens
	if contextWindow <= 0 {
		contextWindow = 180000
	}
	return llm.Limits{ContextWindowTokens: contextWindow, MaxOutputTokens: 8192}
}

// CountTokens estimates rather than calling the counting endpoint. Budgets are
// enforced against actual usage reported with each response, so an estimate
// here is only ever used for pre-flight sizing.
func (provider *Provider) CountTokens(ctx context.Context, request llm.Request) (int, error) {
	return llm.EstimateTokens(request), nil
}

// ErrNoAPIKey is returned when the provider is constructed without credentials.
var ErrNoAPIKey = errors.New("anthropic: no API key configured")

// APIError is a non-retryable error reported by the API.
type APIError struct {
	StatusCode int
	Type       string
	Message    string
}

func (apiError *APIError) Error() string {
	return fmt.Sprintf("anthropic: %d %s: %s", apiError.StatusCode, apiError.Type, apiError.Message)
}

// Retryable reports whether this status is worth another attempt.
func (apiError *APIError) Retryable() bool {
	return apiError.StatusCode == http.StatusTooManyRequests ||
		apiError.StatusCode == http.StatusRequestTimeout ||
		apiError.StatusCode >= 500
}

// ---------------------------------------------------------------- wire ---

type wireRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	System      string        `json:"system,omitempty"`
	Messages    []wireMessage `json:"messages"`
	Tools       []wireTool    `json:"tools,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type wireMessage struct {
	Role    string      `json:"role"`
	Content []wireBlock `json:"content"`
}

type wireBlock struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type wireTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type wireResponse struct {
	Model      string      `json:"model"`
	Content    []wireBlock `json:"content"`
	StopReason string      `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete performs one completion, retrying transient failures.
func (provider *Provider) Complete(ctx context.Context, request llm.Request) (*llm.Response, error) {
	if provider.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	modelName, known := provider.Models[request.Tier]
	if !known {
		return nil, fmt.Errorf("anthropic: no model configured for tier %q", request.Tier)
	}

	encodedBody, err := json.Marshal(provider.buildWireRequest(modelName, request))
	if err != nil {
		return nil, fmt.Errorf("anthropic: encode request: %w", err)
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
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint(), bytes.NewReader(encodedBody))
	if err != nil {
		return nil, 0, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpRequest.Header.Set("content-type", "application/json")
	httpRequest.Header.Set("x-api-key", provider.APIKey)
	httpRequest.Header.Set("anthropic-version", provider.apiVersion())

	httpResponse, err := provider.httpClient().Do(httpRequest)
	if err != nil {
		return nil, 0, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("anthropic: read response: %w", err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		// Honour the server's own backoff instruction rather than guessing one.
		retryAfter := parseRetryAfter(httpResponse.Header.Get("retry-after"))
		return nil, retryAfter, apiErrorFrom(httpResponse.StatusCode, responseBody)
	}

	var decoded wireResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, 0, fmt.Errorf("anthropic: decode response: %w", err)
	}
	if decoded.Error != nil {
		return nil, 0, &APIError{StatusCode: httpResponse.StatusCode,
			Type: decoded.Error.Type, Message: decoded.Error.Message}
	}
	return toNeutralResponse(decoded), 0, nil
}

func apiErrorFrom(statusCode int, responseBody []byte) error {
	apiError := &APIError{StatusCode: statusCode, Type: "api_error", Message: string(responseBody)}
	var decoded wireResponse
	if err := json.Unmarshal(responseBody, &decoded); err == nil && decoded.Error != nil {
		apiError.Type = decoded.Error.Type
		apiError.Message = decoded.Error.Message
	}
	return apiError
}

func (provider *Provider) buildWireRequest(modelName string, request llm.Request) wireRequest {
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	built := wireRequest{
		Model:     modelName,
		MaxTokens: maxTokens,
		System:    request.System,
		Messages:  toWireMessages(request.Messages),
	}
	if request.Temperature > 0 {
		temperature := request.Temperature
		built.Temperature = &temperature
	}
	for _, toolSchema := range request.Tools {
		built.Tools = append(built.Tools, wireTool{
			Name: toolSchema.Name, Description: toolSchema.Description, InputSchema: toolSchema.InputSchema,
		})
	}
	return built
}

// toWireMessages translates the neutral history into Anthropic's block format.
// Tool results are user-role blocks in this API, which is the one asymmetry
// worth knowing about when reading this translation.
func toWireMessages(messages []llm.Message) []wireMessage {
	wireMessages := make([]wireMessage, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case llm.RoleAssistant:
			blocks := []wireBlock{}
			if message.Text != "" {
				blocks = append(blocks, wireBlock{Type: "text", Text: message.Text})
			}
			for _, toolCall := range message.ToolCalls {
				blocks = append(blocks, wireBlock{
					Type: "tool_use", ID: toolCall.ID, Name: toolCall.ToolName, Input: toolCall.Arguments,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			wireMessages = append(wireMessages, wireMessage{Role: "assistant", Content: blocks})

		case llm.RoleTool:
			blocks := make([]wireBlock, 0, len(message.ToolResults))
			for _, toolResult := range message.ToolResults {
				blocks = append(blocks, wireBlock{
					Type: "tool_result", ToolUseID: toolResult.CallID,
					Content: toolResult.Content, IsError: toolResult.IsError,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			wireMessages = append(wireMessages, wireMessage{Role: "user", Content: blocks})

		default:
			if message.Text == "" {
				continue
			}
			wireMessages = append(wireMessages, wireMessage{
				Role: "user", Content: []wireBlock{{Type: "text", Text: message.Text}},
			})
		}
	}
	return wireMessages
}

func toNeutralResponse(decoded wireResponse) *llm.Response {
	response := &llm.Response{
		ModelName: decoded.Model,
		Usage: llm.Usage{
			InputTokens: decoded.Usage.InputTokens, OutputTokens: decoded.Usage.OutputTokens,
		},
	}
	for _, block := range decoded.Content {
		switch block.Type {
		case "text":
			response.Text += block.Text
		case "tool_use":
			response.ToolCalls = append(response.ToolCalls, llm.ToolCall{
				ID: block.ID, ToolName: block.Name, Arguments: block.Input,
			})
		}
	}
	switch decoded.StopReason {
	case "tool_use":
		response.StopReason = llm.StopToolUse
	case "max_tokens":
		response.StopReason = llm.StopMaxTokens
	default:
		response.StopReason = llm.StopEndTurn
	}
	return response
}

// ---------------------------------------------------------- small parts ---

func (provider *Provider) endpoint() string {
	if provider.Endpoint != "" {
		return provider.Endpoint
	}
	return DefaultEndpoint
}

func (provider *Provider) apiVersion() string {
	if provider.APIVersion != "" {
		return provider.APIVersion
	}
	return DefaultAPIVersion
}

func (provider *Provider) httpClient() *http.Client {
	if provider.HTTPClient != nil {
		return provider.HTTPClient
	}
	return http.DefaultClient
}

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
