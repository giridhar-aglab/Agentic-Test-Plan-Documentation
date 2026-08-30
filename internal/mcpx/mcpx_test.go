package mcpx

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/tool"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// pipeTransport joins a client to a server running in the same process, which
// lets both halves of the protocol be tested against each other rather than
// against a mock of one side's assumptions.
type pipeTransport struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (transport *pipeTransport) Read(buffer []byte) (int, error) {
	return transport.reader.Read(buffer)
}
func (transport *pipeTransport) Write(buffer []byte) (int, error) {
	return transport.writer.Write(buffer)
}
func (transport *pipeTransport) Close() error {
	_ = transport.reader.Close()
	return transport.writer.Close()
}

// startLoopback runs a server over pipes and returns a client wired to it.
func startLoopback(t *testing.T, server *Server) *Client {
	t.Helper()

	clientToServerReader, clientToServerWriter := io.Pipe()
	serverToClientReader, serverToClientWriter := io.Pipe()

	serverContext, cancelServer := context.WithCancel(context.Background())
	go func() {
		_ = server.Serve(serverContext, clientToServerReader, serverToClientWriter)
		_ = serverToClientWriter.Close()
	}()

	client := NewClient("loopback", &pipeTransport{reader: serverToClientReader, writer: clientToServerWriter})
	t.Cleanup(func() {
		cancelServer()
		_ = client.Close()
	})
	return client
}

func echoServer() *Server {
	return &Server{
		Name: "test-server", Version: "1.0",
		Tools: []ServerTool{
			{
				Name: "echo", Description: "echo the message back",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
				Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
					var parameters struct {
						Message string `json:"message"`
					}
					if err := json.Unmarshal(arguments, &parameters); err != nil {
						return "", err
					}
					if parameters.Message == "" {
						return "", errors.New("\"message\" is required")
					}
					return parameters.Message, nil
				},
			},
		},
	}
}

func TestDiscoverNegotiatesProtocolVersion(t *testing.T) {
	client := startLoopback(t, echoServer())
	if err := client.Discover(context.Background()); err != nil {
		t.Fatalf("discover should succeed against a server on the same revision: %v", err)
	}
}

// rawResponder is a hand-written JSON-RPC server that lets a test control the
// exact bytes on the wire, which the loopback Server deliberately cannot.
func rawResponder(t *testing.T, respond func(method string, params json.RawMessage) (any, *rpcError)) *Client {
	t.Helper()

	clientToServerReader, clientToServerWriter := io.Pipe()
	serverToClientReader, serverToClientWriter := io.Pipe()

	go func() {
		defer serverToClientWriter.Close()
		reader := bufio.NewReader(clientToServerReader)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var request rpcRequest
			if err := json.Unmarshal(line, &request); err != nil {
				return
			}
			result, responseError := respond(request.Method, request.Params)
			response := rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: responseError}
			if responseError == nil {
				encoded, _ := json.Marshal(result)
				response.Result = encoded
			}
			encodedResponse, _ := json.Marshal(response)
			if _, err := serverToClientWriter.Write(append(encodedResponse, '\n')); err != nil {
				return
			}
		}
	}()

	client := NewClient("raw", &pipeTransport{reader: serverToClientReader, writer: clientToServerWriter})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestDiscoverRejectsAServerOnADifferentRevision(t *testing.T) {
	// There is no initialize handshake under this revision, so server/discover
	// is the only chance to learn we cannot talk to each other. Discovering it
	// here beats discovering it as a puzzling failure mid-run.
	client := rawResponder(t, func(method string, params json.RawMessage) (any, *rpcError) {
		return map[string]any{
			"protocolVersions": []string{"2025-06-18"},
			"serverInfo":       map[string]string{"name": "old-server", "version": "1"},
		}, nil
	})

	err := client.Discover(context.Background())
	if err == nil {
		t.Fatal("a server on a different revision must be rejected")
	}
	if !strings.Contains(err.Error(), "2025-06-18") || !strings.Contains(err.Error(), ProtocolVersion) {
		t.Fatalf("the error must name both revisions so the mismatch is actionable, got %v", err)
	}
}

func TestDiscoverSurfacesAnUnsupportedVersionError(t *testing.T) {
	client := rawResponder(t, func(method string, params json.RawMessage) (any, *rpcError) {
		return nil, &rpcError{Code: codeUnsupportedProtocolVersion, Message: "unsupported"}
	})

	err := client.Discover(context.Background())
	if err == nil || !strings.Contains(err.Error(), "does not support protocol") {
		t.Fatalf("an UnsupportedProtocolVersion error must be reported plainly, got %v", err)
	}
}

func TestDiscoverToleratesAServerThatAdvertisesNothing(t *testing.T) {
	// Refusing to work with a server that simply omits the field would be
	// stricter than the spec requires and would break usable integrations.
	client := rawResponder(t, func(method string, params json.RawMessage) (any, *rpcError) {
		return map[string]any{"serverInfo": map[string]string{"name": "terse"}}, nil
	})
	if err := client.Discover(context.Background()); err != nil {
		t.Fatalf("a server advertising no versions should be given the benefit of the doubt, got %v", err)
	}
}

func TestEveryRequestCarriesProtocolMetaOnTheWire(t *testing.T) {
	// Statelessness means the version and identity travel on every request
	// rather than being established once. This asserts the bytes, not the
	// helper that builds them.
	observedMetaByMethod := map[string]map[string]any{}
	client := rawResponder(t, func(method string, params json.RawMessage) (any, *rpcError) {
		var decoded struct {
			Meta map[string]any `json:"_meta"`
		}
		_ = json.Unmarshal(params, &decoded)
		observedMetaByMethod[method] = decoded.Meta

		switch method {
		case "server/discover":
			return map[string]any{"protocolVersions": []string{ProtocolVersion}}, nil
		case "tools/list":
			return map[string]any{"tools": []ToolDescriptor{}, "ttlMs": 1000}, nil
		default:
			return map[string]any{"content": []map[string]string{{"type": "text", "text": "ok"}}}, nil
		}
	})

	if err := client.Discover(context.Background()); err != nil {
		t.Fatalf("discover: %v", err)
	}
	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if _, _, err := client.CallTool(context.Background(), "anything", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("tools/call: %v", err)
	}

	for _, method := range []string{"server/discover", "tools/list", "tools/call"} {
		meta, present := observedMetaByMethod[method]
		if !present {
			t.Fatalf("%s was never observed", method)
		}
		if meta[metaProtocolVersion] != ProtocolVersion {
			t.Errorf("%s did not carry the protocol version, got %v", method, meta[metaProtocolVersion])
		}
		if _, present := meta[metaClientInfo]; !present {
			t.Errorf("%s did not identify the client", method)
		}
		if _, present := meta[metaClientCapabilities]; !present {
			t.Errorf("%s did not carry client capabilities", method)
		}
	}
}

func TestListToolsCachesForTheServerStatedWindow(t *testing.T) {
	client := startLoopback(t, echoServer())

	firstListing, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	if len(firstListing) != 1 || firstListing[0].Name != "echo" {
		t.Fatalf("unexpected catalogue: %+v", firstListing)
	}

	// A second call inside the window must not hit the wire. Proving that by
	// closing the transport is crude but unambiguous.
	_ = client.transport.Close()
	secondListing, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("a cached listing must not require the transport: %v", err)
	}
	if len(secondListing) != 1 {
		t.Fatalf("cached catalogue lost entries: %+v", secondListing)
	}
}

func TestListToolsRefetchesAfterTheCacheExpires(t *testing.T) {
	client := startLoopback(t, echoServer())
	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("first listing failed: %v", err)
	}

	// Wind the clock past the server's stated ttlMs.
	client.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	refreshed, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("expired cache should be refetched: %v", err)
	}
	if len(refreshed) != 1 {
		t.Fatalf("unexpected catalogue after refetch: %+v", refreshed)
	}
}

func TestRemoteToolIsIndistinguishableFromALocalTool(t *testing.T) {
	// The point of the whole package: once adapted, a remote tool goes into the
	// same registry, under the same policy, as an in-process Go function.
	client := startLoopback(t, echoServer())
	descriptors, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	registry := tool.NewRegistry()
	registry.Register(NewRemoteTool(client, descriptors[0], true), tool.NetworkPolicy())

	arguments, _ := json.Marshal(map[string]string{"message": "hello from the server"})
	result, err := registry.Invoke(context.Background(), "echo", arguments)
	if err != nil {
		t.Fatalf("remote invoke failed: %v", err)
	}
	if result.Content != "hello from the server" {
		t.Fatalf("unexpected content %q", result.Content)
	}
	if result.Source.Kind != "mcp" || result.Source.Server != "loopback" {
		t.Fatalf("provenance must record which server answered, got %+v", result.Source)
	}
}

func TestServerToolErrorBecomesCorrectableNotRetryable(t *testing.T) {
	// A tool rejecting its arguments is the model's problem. Classifying it as
	// retryable would burn the budget re-sending arguments that will keep
	// failing.
	client := startLoopback(t, echoServer())
	descriptors, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	remoteTool := NewRemoteTool(client, descriptors[0], true)

	_, err = remoteTool.Invoke(context.Background(), json.RawMessage(`{}`))
	if tool.ClassOf(err) != tool.FailureCorrectable {
		t.Fatalf("a server-side tool error must be correctable, got %v", tool.ClassOf(err))
	}
	if !strings.Contains(tool.AdviceOf(err), "message") {
		t.Fatalf("the server's own message must reach the model, got %q", tool.AdviceOf(err))
	}
}

func TestUnknownToolIsCorrectable(t *testing.T) {
	client := startLoopback(t, echoServer())
	unknownTool := NewRemoteTool(client, ToolDescriptor{Name: "does_not_exist"}, true)

	_, err := unknownTool.Invoke(context.Background(), json.RawMessage(`{}`))
	if tool.ClassOf(err) != tool.FailureCorrectable {
		t.Fatalf("invalid params must map to correctable, got %v", tool.ClassOf(err))
	}
}

func TestClosedConnectionDegradesRatherThanSpinning(t *testing.T) {
	// The 2026 transport removed stream resumability, so a broken stream is not
	// worth retrying on the same connection.
	client := startLoopback(t, echoServer())
	remoteTool := NewRemoteTool(client, ToolDescriptor{Name: "echo"}, true)
	_ = client.transport.Close()

	_, err := remoteTool.Invoke(context.Background(), json.RawMessage(`{"message":"x"}`))
	failureClass := tool.ClassOf(err)
	if failureClass != tool.FailureDegradable && failureClass != tool.FailureRetryable {
		t.Fatalf("a dead connection should degrade or retry, got %v", failureClass)
	}
}

func TestTaskStoreTracksALongRunningInvocation(t *testing.T) {
	taskStore := NewTaskStore()
	task := taskStore.Start()

	if stored, found := taskStore.Get(task.ID); !found || stored.Status != TaskRunning {
		t.Fatalf("a started task must be running, got %+v", stored)
	}
	taskStore.SetPhase(task.ID, "analyse")
	taskStore.Complete(task.ID, "# Test Plan")

	stored, _ := taskStore.Get(task.ID)
	if stored.Status != TaskCompleted || stored.Result != "# Test Plan" {
		t.Fatalf("completion must be visible to a polling caller, got %+v", stored)
	}
	if stored.Phase != "analyse" {
		t.Errorf("phase progress must be visible while running, got %q", stored.Phase)
	}
	if _, found := taskStore.Get("no-such-task"); found {
		t.Error("an unknown task id must not resolve")
	}
}

func TestGitHubSourceRefusesATruncatedTree(t *testing.T) {
	// A truncated tree would produce a plan that looks complete over a
	// repository it never fully saw.
	server := &Server{
		Name: "github", Version: "1",
		Tools: []ServerTool{{
			Name: "get_repository_tree", InputSchema: json.RawMessage(`{"type":"object"}`),
			Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return `{"tree":[{"path":"a.go","type":"blob","sha":"abc","size":10}],"truncated":true}`, nil
			},
		}},
	}
	client := startLoopback(t, server)
	githubSource := NewGitHubSource(client, "owner", "repo", "main", 400)

	if _, err := githubSource.Tree(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "truncated") {
		t.Fatalf("a truncated tree must be refused, got %v", err)
	}
}

func TestGitHubSourceEnforcesTheFileCeiling(t *testing.T) {
	server := &Server{
		Name: "github", Version: "1",
		Tools: []ServerTool{{
			Name: "get_repository_tree", InputSchema: json.RawMessage(`{"type":"object"}`),
			Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return `{"tree":[
					{"path":"a.go","type":"blob","sha":"a","size":1},
					{"path":"b.go","type":"blob","sha":"b","size":1},
					{"path":"c.go","type":"blob","sha":"c","size":1}]}`, nil
			},
		}},
	}
	client := startLoopback(t, server)
	githubSource := NewGitHubSource(client, "owner", "repo", "main", 2)

	_, err := githubSource.Tree(context.Background())
	var tooManyFiles *repo.ErrTooManyFiles
	if !errors.As(err, &tooManyFiles) {
		t.Fatalf("expected the ceiling to be enforced, got %v", err)
	}
}

func TestGitHubSourceDecodesAndCachesFileContent(t *testing.T) {
	fetchCount := 0
	server := &Server{
		Name: "github", Version: "1",
		Tools: []ServerTool{{
			Name: "get_file_contents", InputSchema: json.RawMessage(`{"type":"object"}`),
			Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
				fetchCount++
				// base64 of "package main\n"
				return `{"content":"cGFja2FnZSBtYWluCg==","encoding":"base64"}`, nil
			},
		}},
	}
	client := startLoopback(t, server)
	githubSource := NewGitHubSource(client, "owner", "repo", "main", 400)

	firstRead, err := githubSource.ReadFile(context.Background(), "main.go")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if firstRead != "package main\n" {
		t.Fatalf("base64 content was not decoded, got %q", firstRead)
	}

	// Four hundred blob fetches meet GitHub's secondary rate limiter, so not
	// fetching the same file twice is not an optimisation but a requirement.
	if _, err := githubSource.ReadFile(context.Background(), "main.go"); err != nil {
		t.Fatalf("second read failed: %v", err)
	}
	if fetchCount != 1 {
		t.Fatalf("the same file must not be fetched twice in a run, got %d fetches", fetchCount)
	}
}

func TestGitHubSourceReportsMissingPathAsNotFound(t *testing.T) {
	server := &Server{
		Name: "github", Version: "1",
		Tools: []ServerTool{{
			Name: "get_file_contents", InputSchema: json.RawMessage(`{"type":"object"}`),
			Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return "", errors.New("404 Not Found")
			},
		}},
	}
	client := startLoopback(t, server)
	githubSource := NewGitHubSource(client, "owner", "repo", "main", 400)

	_, err := githubSource.ReadFile(context.Background(), "missing.go")
	var notFound *repo.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("a missing path must map to ErrNotFound so the tool layer can make it correctable, got %v", err)
	}
}

func TestGitHubSourceSatisfiesRepoSource(t *testing.T) {
	// A compile-time assertion that the seam actually holds: if this stops
	// being true, nothing downstream can use GitHub.
	var _ repo.Source = (*GitHubSource)(nil)
}
