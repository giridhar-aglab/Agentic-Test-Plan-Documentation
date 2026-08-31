package mcpx

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// rawServer answers tools/call with a caller-supplied result object, so a
// content shape this codebase does not produce can still be tested.
//
// It exists because the in-repo Server only ever emits {"type":"text"} blocks.
// Testing the client against it meant the stub agreed with the client instead
// of checking it — and GitHub's server, which returns file contents as an
// embedded resource, emptied every file this system read.
func rawServer(t *testing.T, resultJSON string) *Client {
	t.Helper()

	clientToServerReader, clientToServerWriter := io.Pipe()
	serverToClientReader, serverToClientWriter := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(clientToServerReader)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			var request struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
				continue
			}
			var result string
			switch request.Method {
			case "server/discover":
				result = `{"protocolVersion":"` + ProtocolVersion + `","serverInfo":{"name":"raw","version":"1"}}`
			case "tools/call":
				result = resultJSON
			default:
				result = `{}`
			}
			_, _ = serverToClientWriter.Write([]byte(
				`{"jsonrpc":"2.0","id":` + string(request.ID) + `,"result":` + result + "}\n"))
		}
	}()

	client := NewClient("raw", &pipeTransport{reader: serverToClientReader, writer: clientToServerWriter})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestFileContentsArriveAsAnEmbeddedResource(t *testing.T) {
	// This is the shape GitHub's get_file_contents actually returns for a text
	// file. A client modelling only {"type":"text"} produces an empty string
	// here — indistinguishable from an empty file, and impossible to diagnose
	// from the outside. The analyst re-read every file until its budget was
	// gone.
	client := rawServer(t, `{"content":[{"type":"resource","resource":{"uri":"repo://gin/logger.go","mimeType":"text/x-go","text":"package gin\n\nfunc Logger() {}\n"}}]}`)

	content, isError, err := client.CallTool(context.Background(), "get_file_contents", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError {
		t.Fatal("a successful read must not be reported as an error")
	}
	if !strings.Contains(content, "func Logger()") {
		t.Fatalf("the file contents were dropped, got %q", content)
	}
}

func TestABase64ResourceBlobIsDecoded(t *testing.T) {
	// GitHub sends some text files as a blob rather than as text.
	encoded := base64.StdEncoding.EncodeToString([]byte("package gin\n\nfunc Recovery() {}\n"))
	client := rawServer(t, `{"content":[{"type":"resource","resource":{"uri":"repo://gin/recovery.go","blob":"`+encoded+`"}}]}`)

	content, _, err := client.CallTool(context.Background(), "get_file_contents", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "func Recovery()") {
		t.Fatalf("a base64 blob should be decoded, got %q", content)
	}
}

func TestAnUnreadableResultIsAnErrorRatherThanSilence(t *testing.T) {
	// The failure mode this whole class produces is silence. Whatever arrives,
	// a caller must be able to tell "empty file" from "shape I do not
	// understand" — otherwise the only available move is to try again.
	client := rawServer(t, `{"content":[{"type":"video","data":"AAAA","mimeType":"video/mp4"}]}`)

	_, _, err := client.CallTool(context.Background(), "get_file_contents", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("an unrecognised content block must be reported, not silently dropped")
	}
	if !strings.Contains(err.Error(), "video") {
		t.Errorf("the error should name the block types that arrived, got %v", err)
	}
}

func TestAResourceLinkSaysSoRatherThanLookingEmpty(t *testing.T) {
	// GitHub returns a link instead of content for files of 1 MB or more.
	client := rawServer(t, `{"content":[{"type":"resource_link","uri":"https://github.com/o/r/raw/main/huge.go","name":"huge.go"}]}`)

	content, _, err := client.CallTool(context.Background(), "get_file_contents", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("a link carries no content and must not read as success")
	}
	if !strings.Contains(content, "huge.go") {
		t.Errorf("the link should be visible to whoever has to act on it, got %q", content)
	}
}

func TestTextBlocksStillWork(t *testing.T) {
	client := rawServer(t, `{"content":[{"type":"text","text":"plain result"}]}`)
	content, _, err := client.CallTool(context.Background(), "some_tool", json.RawMessage(`{}`))
	if err != nil || content != "plain result" {
		t.Fatalf("got %q, %v", content, err)
	}
}
