// Package mcpx speaks the Model Context Protocol, in both directions.
//
// The official Go SDK would normally do this. It is not vendored here because
// this build could not reach the module proxy, so the client is written
// directly against specification revision 2026-07-28 using only the standard
// library. That revision makes the job small: the protocol is stateless, so
// there is no initialize handshake and no session to keep alive — every request
// carries its own protocol version and client identity in _meta.
//
// The seam that matters is not this file. It is that every remote tool is
// adapted into the same tool.Tool interface as an in-process Go function, so
// the agent loop cannot tell them apart and the policy layer applies retry,
// timeout and breaker rules to both.
package mcpx

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/tool"
)

// ProtocolVersion is the revision this client speaks.
const ProtocolVersion = "2026-07-28"

// Meta keys defined by the specification.
const (
	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientInfo         = "io.modelcontextprotocol/clientInfo"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
)

// Implementation identifies this client to servers.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// rpcRequest is a JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (responseError *rpcError) Error() string {
	return fmt.Sprintf("mcp: rpc error %d: %s", responseError.Code, responseError.Message)
}

// Error codes the specification reserves. UnsupportedProtocolVersion is fatal;
// the rest are transport-level and worth a retry.
const (
	codeUnsupportedProtocolVersion = -32022
	codeInvalidParams              = -32602
)

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// ToolDescriptor is a tool as a server advertises it.
type ToolDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// listToolsResult carries the catalogue plus its cache directives. The 2026
// revision requires ttlMs and cacheScope, which is what lets a client cache the
// catalogue for its stated freshness window instead of re-listing every turn.
type listToolsResult struct {
	Tools      []ToolDescriptor `json:"tools"`
	TTLMs      int64            `json:"ttlMs"`
	CacheScope string           `json:"cacheScope"`
	ResultType string           `json:"resultType"`
}

type discoverResult struct {
	ProtocolVersions []string `json:"protocolVersions"`
	ServerInfo       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// callToolResult is one tool invocation's outcome.
type callToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError    bool   `json:"isError"`
	ResultType string `json:"resultType"`
}

// Transport is a bidirectional byte stream to a server.
type Transport interface {
	io.ReadWriteCloser
}

// CommandTransport runs a server as a subprocess and speaks over its stdio.
type CommandTransport struct {
	command      *exec.Cmd
	stdinWriter  io.WriteCloser
	stdoutReader io.ReadCloser
}

// StartCommand launches an MCP server subprocess.
func StartCommand(ctx context.Context, executable string, arguments []string, environment []string) (*CommandTransport, error) {
	command := exec.CommandContext(ctx, executable, arguments...)
	if len(environment) > 0 {
		command.Env = append(command.Environ(), environment...)
	}
	stdinWriter, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdin pipe: %w", err)
	}
	stdoutReader, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdout pipe: %w", err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("mcp: start %s: %w", executable, err)
	}
	return &CommandTransport{command: command, stdinWriter: stdinWriter, stdoutReader: stdoutReader}, nil
}

func (transport *CommandTransport) Read(buffer []byte) (int, error) {
	return transport.stdoutReader.Read(buffer)
}
func (transport *CommandTransport) Write(buffer []byte) (int, error) {
	return transport.stdinWriter.Write(buffer)
}

func (transport *CommandTransport) Close() error {
	_ = transport.stdinWriter.Close()
	_ = transport.stdoutReader.Close()
	if transport.command.Process != nil {
		_ = transport.command.Process.Kill()
	}
	return transport.command.Wait()
}

// Client is a stateless MCP client over one transport.
type Client struct {
	ServerName string
	Info       Implementation

	transport Transport
	writer    *bufio.Writer
	reader    *bufio.Reader

	mutex          sync.Mutex
	nextRequestID  int64
	cachedTools    []ToolDescriptor
	cacheExpiresAt time.Time

	now func() time.Time
}

// NewClient wraps a transport.
func NewClient(serverName string, transport Transport) *Client {
	return &Client{
		ServerName: serverName,
		Info:       Implementation{Name: "testplan-agent", Version: "0.2"},
		transport:  transport,
		writer:     bufio.NewWriter(transport),
		reader:     bufio.NewReader(transport),
		now:        time.Now,
	}
}

// Close releases the transport.
func (client *Client) Close() error { return client.transport.Close() }

// buildMeta assembles the per-request _meta every call must carry now that the
// protocol is stateless.
func (client *Client) buildMeta() map[string]any {
	return map[string]any{
		metaProtocolVersion:    ProtocolVersion,
		metaClientInfo:         client.Info,
		metaClientCapabilities: map[string]any{},
	}
}

// call performs one request/response round trip.
func (client *Client) call(ctx context.Context, method string, parameters map[string]any) (json.RawMessage, error) {
	if parameters == nil {
		parameters = map[string]any{}
	}
	parameters["_meta"] = client.buildMeta()

	encodedParams, err := json.Marshal(parameters)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode params: %w", err)
	}

	client.mutex.Lock()
	client.nextRequestID++
	requestID := client.nextRequestID
	client.mutex.Unlock()

	encodedRequest, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0", ID: requestID, Method: method, Params: encodedParams,
	})
	if err != nil {
		return nil, fmt.Errorf("mcp: encode request: %w", err)
	}

	// Newline-delimited JSON is the stdio framing.
	if _, err := client.writer.Write(append(encodedRequest, '\n')); err != nil {
		return nil, fmt.Errorf("mcp: write request: %w", err)
	}
	if err := client.writer.Flush(); err != nil {
		return nil, fmt.Errorf("mcp: flush request: %w", err)
	}

	// Skip notifications and any response for a different id; a broken stream
	// loses the in-flight request, which the 2026 transport makes explicit by
	// removing resumability.
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, err := client.reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("mcp: read response: %w", err)
		}
		trimmedLine := strings.TrimSpace(string(line))
		if trimmedLine == "" {
			continue
		}
		var decoded rpcResponse
		if err := json.Unmarshal([]byte(trimmedLine), &decoded); err != nil {
			return nil, fmt.Errorf("mcp: decode response: %w", err)
		}
		if decoded.ID != requestID {
			continue
		}
		if decoded.Error != nil {
			return nil, decoded.Error
		}
		return decoded.Result, nil
	}
}

// Discover negotiates the protocol version up front. Under the stateless
// revision this replaces the old initialize handshake and is the only way to
// find out whether a server can talk to us at all.
func (client *Client) Discover(ctx context.Context) error {
	rawResult, err := client.call(ctx, "server/discover", nil)
	if err != nil {
		var responseError *rpcError
		if errors.As(err, &responseError) && responseError.Code == codeUnsupportedProtocolVersion {
			return fmt.Errorf("mcp: server %s does not support protocol %s: %w",
				client.ServerName, ProtocolVersion, err)
		}
		return err
	}
	var discovered discoverResult
	if err := json.Unmarshal(rawResult, &discovered); err != nil {
		return fmt.Errorf("mcp: decode discover: %w", err)
	}
	for _, supportedVersion := range discovered.ProtocolVersions {
		if supportedVersion == ProtocolVersion {
			return nil
		}
	}
	if len(discovered.ProtocolVersions) == 0 {
		return nil // A server that advertises nothing is given the benefit of the doubt.
	}
	return fmt.Errorf("mcp: server %s speaks %v, not %s",
		client.ServerName, discovered.ProtocolVersions, ProtocolVersion)
}

// ListTools returns the server's catalogue, honouring the cache window the
// server itself specifies.
func (client *Client) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	client.mutex.Lock()
	if client.cachedTools != nil && client.now().Before(client.cacheExpiresAt) {
		toolsCopy := append([]ToolDescriptor{}, client.cachedTools...)
		client.mutex.Unlock()
		return toolsCopy, nil
	}
	client.mutex.Unlock()

	rawResult, err := client.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var listed listToolsResult
	if err := json.Unmarshal(rawResult, &listed); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/list: %w", err)
	}

	cacheDuration := time.Duration(listed.TTLMs) * time.Millisecond
	if cacheDuration <= 0 {
		cacheDuration = 60 * time.Second
	}
	// A private-scope result is still cacheable by us; it just must not be
	// shared onward, which this client never does.
	client.mutex.Lock()
	client.cachedTools = listed.Tools
	client.cacheExpiresAt = client.now().Add(cacheDuration)
	client.mutex.Unlock()

	return listed.Tools, nil
}

// CallTool invokes a remote tool.
func (client *Client) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, bool, error) {
	parameters := map[string]any{"name": toolName}
	if len(arguments) > 0 {
		parameters["arguments"] = json.RawMessage(arguments)
	}
	rawResult, err := client.call(ctx, "tools/call", parameters)
	if err != nil {
		return "", false, err
	}
	var called callToolResult
	if err := json.Unmarshal(rawResult, &called); err != nil {
		return "", false, fmt.Errorf("mcp: decode tools/call: %w", err)
	}

	var contentBuilder strings.Builder
	for _, contentBlock := range called.Content {
		if contentBlock.Type == "text" {
			contentBuilder.WriteString(contentBlock.Text)
		}
	}
	return contentBuilder.String(), called.IsError, nil
}

// RemoteTool adapts one server-side tool to the local Tool interface.
//
// This is the whole point of the package: once a remote tool implements
// tool.Tool, "internal and external tool calls" stop being two code paths and
// become one registry with one policy layer.
type RemoteTool struct {
	client     *Client
	descriptor ToolDescriptor
	// idempotent cannot be inferred from the protocol, so it is declared by the
	// caller. Defaulting to false would disable retry everywhere; defaulting to
	// true would retry a write. The wiring decides, per tool, deliberately.
	idempotent bool
}

// NewRemoteTool wraps a descriptor.
func NewRemoteTool(client *Client, descriptor ToolDescriptor, idempotent bool) *RemoteTool {
	return &RemoteTool{client: client, descriptor: descriptor, idempotent: idempotent}
}

func (remoteTool *RemoteTool) Name() string        { return remoteTool.descriptor.Name }
func (remoteTool *RemoteTool) Description() string { return remoteTool.descriptor.Description }
func (remoteTool *RemoteTool) Idempotent() bool    { return remoteTool.idempotent }

func (remoteTool *RemoteTool) InputSchema() json.RawMessage {
	if len(remoteTool.descriptor.InputSchema) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return remoteTool.descriptor.InputSchema
}

func (remoteTool *RemoteTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	content, isError, err := remoteTool.client.CallTool(ctx, remoteTool.descriptor.Name, arguments)
	if err != nil {
		return tool.Result{}, classifyTransportError(remoteTool.descriptor.Name, err)
	}
	if isError {
		// A server-side tool error is the model's problem, not the transport's.
		// It must reach the model as readable text rather than being retried.
		return tool.Result{}, tool.Correctable(remoteTool.descriptor.Name, content)
	}
	return tool.Result{
		Content: content, Confidence: 1, Source: tool.MCP(remoteTool.client.ServerName),
	}, nil
}

// classifyTransportError decides which of the four failure classes an MCP
// error belongs to. Getting this wrong in either direction is expensive: a
// misclassified correctable error burns retries, and a misclassified transport
// error asks the model to fix something it cannot.
func classifyTransportError(toolName string, err error) error {
	var responseError *rpcError
	if errors.As(err, &responseError) {
		switch {
		case responseError.Code == codeUnsupportedProtocolVersion:
			return tool.NewFailure(tool.FailureFatal, toolName,
				"this MCP server cannot speak our protocol version", err)
		case responseError.Code == codeInvalidParams:
			return tool.Correctable(toolName,
				"the server rejected these arguments: "+responseError.Message)
		}
	}
	// A broken stdio stream is not recoverable by retrying on the same
	// transport, so it degrades rather than spinning.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return tool.NewFailure(tool.FailureDegradable, toolName,
			"the MCP server connection closed; continue without this capability", err)
	}
	return tool.NewFailure(tool.FailureRetryable, toolName,
		"the MCP server did not respond; retrying", err)
}
