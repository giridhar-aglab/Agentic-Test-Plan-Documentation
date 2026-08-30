package mcpx

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ServerTool is one tool this process exposes.
type ServerTool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	// Handler returns the text result. A long-running handler should return a
	// task handle instead of blocking; see PlanHandler below.
	Handler func(ctx context.Context, arguments json.RawMessage) (string, error)
}

// Server exposes tools over stdio JSON-RPC.
//
// The stateless revision keeps this small: no initialize handshake, no session
// registry, no ping. A server has to answer server/discover, tools/list and
// tools/call, and that is the whole surface.
type Server struct {
	Name    string
	Version string
	Tools   []ServerTool

	// Tasks backs the long-running path. A full plan takes minutes, which is
	// far longer than any caller should hold a request open for.
	Tasks *TaskStore
}

// Serve reads requests until the input stream closes.
func (server *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("mcp server: read: %w", err)
		}
		trimmedLine := strings.TrimSpace(string(line))
		if trimmedLine == "" {
			continue
		}

		var request rpcRequest
		if err := json.Unmarshal([]byte(trimmedLine), &request); err != nil {
			continue // Not addressable: without an id there is nobody to answer.
		}
		response := server.handle(ctx, request)
		encodedResponse, err := json.Marshal(response)
		if err != nil {
			return fmt.Errorf("mcp server: encode response: %w", err)
		}
		if _, err := writer.Write(append(encodedResponse, '\n')); err != nil {
			return fmt.Errorf("mcp server: write: %w", err)
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("mcp server: flush: %w", err)
		}
	}
}

func (server *Server) handle(ctx context.Context, request rpcRequest) rpcResponse {
	response := rpcResponse{JSONRPC: "2.0", ID: request.ID}

	switch request.Method {
	case "server/discover":
		response.Result = mustMarshal(map[string]any{
			"protocolVersions": []string{ProtocolVersion},
			"serverInfo":       map[string]string{"name": server.Name, "version": server.Version},
			"capabilities":     map[string]any{"tools": map[string]any{}},
			"resultType":       "complete",
		})

	case "tools/list":
		descriptors := make([]ToolDescriptor, 0, len(server.Tools))
		for _, serverTool := range server.Tools {
			descriptors = append(descriptors, ToolDescriptor{
				Name: serverTool.Name, Description: serverTool.Description, InputSchema: serverTool.InputSchema,
			})
		}
		// ttlMs and cacheScope are required by this revision, and they are what
		// stop a client re-listing the catalogue on every turn.
		response.Result = mustMarshal(map[string]any{
			"tools": descriptors, "ttlMs": 300000, "cacheScope": "private", "resultType": "complete",
		})

	case "tools/call":
		response.Result, response.Error = server.callTool(ctx, request.Params)

	case "tasks/get":
		response.Result, response.Error = server.getTask(request.Params)

	default:
		response.Error = &rpcError{Code: -32601, Message: "method not found: " + request.Method}
	}
	return response
}

func (server *Server) callTool(ctx context.Context, rawParams json.RawMessage) (json.RawMessage, *rpcError) {
	var parameters struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &parameters); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "could not decode params: " + err.Error()}
	}

	for _, serverTool := range server.Tools {
		if serverTool.Name != parameters.Name {
			continue
		}
		resultText, err := serverTool.Handler(ctx, parameters.Arguments)
		if err != nil {
			// A tool that failed for a reason the caller can act on is an
			// isError result, not a protocol error. The distinction is the same
			// one the client side makes, in the other direction.
			return mustMarshal(map[string]any{
				"content":    []map[string]string{{"type": "text", "text": err.Error()}},
				"isError":    true,
				"resultType": "complete",
			}), nil
		}
		return mustMarshal(map[string]any{
			"content":    []map[string]string{{"type": "text", "text": resultText}},
			"isError":    false,
			"resultType": "complete",
		}), nil
	}
	return nil, &rpcError{Code: codeInvalidParams, Message: "no such tool: " + parameters.Name}
}

func (server *Server) getTask(rawParams json.RawMessage) (json.RawMessage, *rpcError) {
	if server.Tasks == nil {
		return nil, &rpcError{Code: -32601, Message: "this server does not implement tasks"}
	}
	var parameters struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(rawParams, &parameters); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	task, found := server.Tasks.Get(parameters.TaskID)
	if !found {
		return nil, &rpcError{Code: codeInvalidParams, Message: "no such task: " + parameters.TaskID}
	}
	return mustMarshal(map[string]any{
		"taskId": task.ID, "status": task.Status, "result": task.Result,
		"error": task.Error, "phase": task.Phase, "resultType": "complete",
	}), nil
}

// TaskStatus is the state of a long-running invocation.
type TaskStatus string

const (
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// Task is one long-running invocation.
type Task struct {
	ID      string     `json:"id"`
	Status  TaskStatus `json:"status"`
	Phase   string     `json:"phase"`
	Result  string     `json:"result,omitempty"`
	Error   string     `json:"error,omitempty"`
	Started time.Time  `json:"started"`
}

// TaskStore holds in-flight invocations.
//
// A full plan run takes minutes, so the tool returns a handle immediately and
// the caller polls tasks/get. That is what the tasks extension exists for, and
// it is also what makes the human approval gate survivable: a reviewer who
// takes a day costs one stored task, not a held connection.
type TaskStore struct {
	mutex          sync.Mutex
	tasksByID      map[string]*Task
	sequenceNumber int
}

// NewTaskStore builds an empty store.
func NewTaskStore() *TaskStore {
	return &TaskStore{tasksByID: map[string]*Task{}}
}

// Start registers a new running task.
func (taskStore *TaskStore) Start() *Task {
	taskStore.mutex.Lock()
	defer taskStore.mutex.Unlock()
	taskStore.sequenceNumber++
	task := &Task{
		ID:      fmt.Sprintf("task-%d-%d", time.Now().Unix(), taskStore.sequenceNumber),
		Status:  TaskRunning,
		Started: time.Now().UTC(),
	}
	taskStore.tasksByID[task.ID] = task
	return task
}

// Get returns a task by id.
func (taskStore *TaskStore) Get(taskID string) (Task, bool) {
	taskStore.mutex.Lock()
	defer taskStore.mutex.Unlock()
	task, found := taskStore.tasksByID[taskID]
	if !found {
		return Task{}, false
	}
	return *task, true
}

// SetPhase records progress so a polling caller can see movement.
func (taskStore *TaskStore) SetPhase(taskID, phaseName string) {
	taskStore.mutex.Lock()
	defer taskStore.mutex.Unlock()
	if task, found := taskStore.tasksByID[taskID]; found {
		task.Phase = phaseName
	}
}

// Complete stores a successful result.
func (taskStore *TaskStore) Complete(taskID, result string) {
	taskStore.mutex.Lock()
	defer taskStore.mutex.Unlock()
	if task, found := taskStore.tasksByID[taskID]; found {
		task.Status = TaskCompleted
		task.Result = result
	}
}

// Fail stores a failure.
func (taskStore *TaskStore) Fail(taskID, message string) {
	taskStore.mutex.Lock()
	defer taskStore.mutex.Unlock()
	if task, found := taskStore.tasksByID[taskID]; found {
		task.Status = TaskFailed
		task.Error = message
	}
}

func mustMarshal(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		// Every value marshalled here is a literal built in this file, so a
		// failure would be a programming error rather than a runtime condition.
		panic("mcp server: marshal: " + err.Error())
	}
	return encoded
}
