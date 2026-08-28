// Package llm is the provider abstraction. Agents are configured with a model
// tier rather than a model name, so cheap deterministic phases and expensive
// reasoning phases route independently and the vendor stays swappable.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Tier names the class of model a phase needs. Config maps tiers onto concrete
// model identifiers; nothing outside config knows a model name.
type Tier string

const (
	TierFast     Tier = "fast"
	TierBalanced Tier = "balanced"
	TierStrong   Tier = "strong"
)

// Role is the author of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is a request from the model to invoke a tool.
type ToolCall struct {
	ID        string          `json:"id"`
	ToolName  string          `json:"toolName"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult carries a tool's outcome back to the model. IsError marks a
// failure the model is expected to correct, which is a different thing from a
// transport failure — those never reach the model at all.
type ToolResult struct {
	CallID   string `json:"callId"`
	ToolName string `json:"toolName"`
	Content  string `json:"content"`
	IsError  bool   `json:"isError"`
}

// Message is one turn in a conversation.
type Message struct {
	Role        Role         `json:"role"`
	Text        string       `json:"text,omitempty"`
	ToolCalls   []ToolCall   `json:"toolCalls,omitempty"`
	ToolResults []ToolResult `json:"toolResults,omitempty"`
}

// ToolSchema is a tool as the model sees it. Authored once as JSON Schema and
// translated per provider by the adapter.
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Request is a provider-neutral completion request.
type Request struct {
	Tier        Tier         `json:"tier"`
	System      string       `json:"system,omitempty"`
	Messages    []Message    `json:"messages"`
	Tools       []ToolSchema `json:"tools,omitempty"`
	MaxTokens   int          `json:"maxTokens"`
	Temperature float64      `json:"temperature"`
}

// StopReason explains why generation ended. The agent loop branches on this
// and never on the shape of the text.
type StopReason string

const (
	StopEndTurn   StopReason = "end_turn"
	StopToolUse   StopReason = "tool_use"
	StopMaxTokens StopReason = "max_tokens"
)

// Usage reports token consumption so budgets can be enforced accurately rather
// than estimated.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

func (usage Usage) Total() int { return usage.InputTokens + usage.OutputTokens }

// Response is a provider-neutral completion response.
type Response struct {
	Text       string     `json:"text"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	StopReason StopReason `json:"stopReason"`
	Usage      Usage      `json:"usage"`
	ModelName  string     `json:"modelName"`
}

// Limits describes a model's operating envelope.
type Limits struct {
	ContextWindowTokens int
	MaxOutputTokens     int
}

// Provider is the whole surface the rest of the system may use to reach a
// model. Adapters live in subpackages; nothing else imports a vendor SDK.
type Provider interface {
	Name() string
	Complete(ctx context.Context, request Request) (*Response, error)
	CountTokens(ctx context.Context, request Request) (int, error)
	Limits(tier Tier) Limits
}

// ErrNoScriptedResponse is returned by the fake provider when a test asked for
// more turns than it scripted. Surfacing this loudly beats returning an empty
// response that silently looks like a finished conversation.
var ErrNoScriptedResponse = errors.New("llm: fake provider has no scripted response left")

// EstimateTokens is the deliberately crude fallback used when a provider
// cannot count tokens for us. Four characters per token is close enough for
// budget enforcement, and budgets are meant to be conservative anyway.
func EstimateTokens(request Request) int {
	characterCount := len(request.System)
	for _, message := range request.Messages {
		characterCount += len(message.Text)
		for _, toolCall := range message.ToolCalls {
			characterCount += len(toolCall.ToolName) + len(toolCall.Arguments)
		}
		for _, toolResult := range message.ToolResults {
			characterCount += len(toolResult.Content)
		}
	}
	for _, toolSchema := range request.Tools {
		characterCount += len(toolSchema.Name) + len(toolSchema.Description) + len(toolSchema.InputSchema)
	}
	return characterCount / 4
}

// DescribeRequest renders a request compactly for logs and test failures.
func DescribeRequest(request Request) string {
	return fmt.Sprintf("tier=%s messages=%d tools=%d approxTokens=%d",
		request.Tier, len(request.Messages), len(request.Tools), EstimateTokens(request))
}
