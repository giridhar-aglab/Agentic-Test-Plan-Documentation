// Package tool is the single contract every capability implements, whether it
// runs in this process, on an MCP server, or behind an HTTP API. The agent loop
// cannot tell them apart; only the policy layer knows the difference.
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/giri-ms19/testplan-agent/internal/model"
)

// Result is what a tool hands back. Content is what the model sees.
//
// IsError deserves care: it marks a failure the model is expected to correct,
// such as a bad argument or a path that does not exist. A transport failure is
// a different thing entirely and is returned as a Go error, where the policy
// layer handles it without the model ever seeing it.
type Result struct {
	Content    string           `json:"content"`
	IsError    bool             `json:"isError"`
	Handle     string           `json:"handle,omitempty"`
	Confidence float64          `json:"confidence,omitempty"`
	Source     model.Provenance `json:"source"`
}

// Tool is the one interface.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	// Idempotent gates automatic retry. A tool that creates something must
	// return false, or an ambiguous timeout becomes a duplicate.
	Idempotent() bool
	Invoke(ctx context.Context, arguments json.RawMessage) (Result, error)
}

// FailureClass determines the policy applied to an error. Classifying before
// handling is what keeps the four responses distinct instead of collapsing
// them all into "return err".
type FailureClass int

const (
	// FailureRetryable is transient: timeout, 429, 5xx, connection reset.
	FailureRetryable FailureClass = iota
	// FailureDegradable means the capability is gone but the run is not.
	FailureDegradable
	// FailureFatal means the run cannot produce anything useful.
	FailureFatal
	// FailureCorrectable is the model's mistake, returned to it as a result.
	FailureCorrectable
)

func (failureClass FailureClass) String() string {
	switch failureClass {
	case FailureRetryable:
		return "retryable"
	case FailureDegradable:
		return "degradable"
	case FailureFatal:
		return "fatal"
	case FailureCorrectable:
		return "correctable"
	default:
		return "unknown"
	}
}

// Failure is a classified tool error.
type Failure struct {
	Class    FailureClass
	ToolName string
	// Advice is written for the model, not for a log. It says what went wrong
	// and what to do instead, because a model can only correct a mistake it
	// can read.
	Advice string
	Cause  error
	// RetryAfterSeconds honours a server's own backoff instruction rather than
	// guessing one.
	RetryAfterSeconds int
}

func (failure *Failure) Error() string {
	if failure.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", failure.ToolName, failure.Class, failure.Cause)
	}
	return fmt.Sprintf("%s: %s: %s", failure.ToolName, failure.Class, failure.Advice)
}

func (failure *Failure) Unwrap() error { return failure.Cause }

// Retryable reports whether the policy layer should try again.
func (failure *Failure) Retryable() bool { return failure.Class == FailureRetryable }

// NewFailure builds a classified failure.
func NewFailure(class FailureClass, toolName, advice string, cause error) *Failure {
	return &Failure{Class: class, ToolName: toolName, Advice: advice, Cause: cause}
}

// Correctable builds the failure kind that reaches the model. The advice must
// be actionable: name the problem and the tool that fixes it.
func Correctable(toolName, advice string) *Failure {
	return &Failure{Class: FailureCorrectable, ToolName: toolName, Advice: advice}
}

// ClassOf extracts the failure class from any error, defaulting to retryable
// for unclassified errors so an unexpected fault gets one more chance rather
// than killing a run.
func ClassOf(err error) FailureClass {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Class
	}
	return FailureRetryable
}

// AdviceOf extracts model-facing advice from an error.
func AdviceOf(err error) string {
	var failure *Failure
	if errors.As(err, &failure) && failure.Advice != "" {
		return failure.Advice
	}
	return err.Error()
}

// Internal is the provenance for in-process tools.
func Internal() model.Provenance {
	return model.Provenance{Kind: model.ProvenanceInternal}
}

// MCP is the provenance for a named MCP server.
func MCP(serverName string) model.Provenance {
	return model.Provenance{Kind: model.ProvenanceMCP, Server: serverName}
}

// External is the provenance for a direct network call.
func External(hostName string) model.Provenance {
	return model.Provenance{Kind: model.ProvenanceExternal, Server: hostName}
}

// Text builds a successful result from a string.
func Text(content string, source model.Provenance) Result {
	return Result{Content: content, Confidence: 1, Source: source}
}

// JSON builds a successful result from any marshalable value.
func JSON(value any, source model.Provenance) (Result, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("tool: marshal result: %w", err)
	}
	return Result{Content: string(encoded), Confidence: 1, Source: source}, nil
}

// ErrorResult builds the result form of a correctable failure, which is what
// the agent loop feeds back into the conversation.
func ErrorResult(advice string, source model.Provenance) Result {
	return Result{Content: advice, IsError: true, Source: source}
}
