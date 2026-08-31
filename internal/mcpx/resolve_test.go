package mcpx

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// catalogueServer advertises an arbitrary set of tool names, standing in for
// the different GitHub MCP implementations in the wild.
func catalogueServer(toolNames ...string) *Server {
	server := &Server{Name: "github", Version: "1"}
	for _, toolName := range toolNames {
		name := toolName
		server.Tools = append(server.Tools, ServerTool{
			Name: name, Description: "a tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return "{}", nil
			},
		})
	}
	return server
}

func TestResolveToolNamesMatchesKnownVocabularies(t *testing.T) {
	// Tool naming is not standardised. Discovering the names beats hardcoding
	// one server's vocabulary and breaking on another that works fine.
	testCases := []struct {
		name         string
		catalogue    []string
		expectedTree string
		expectedFile string
	}{
		{
			"reference server",
			[]string{"get_repository_tree", "get_file_contents", "create_issue"},
			"get_repository_tree", "get_file_contents",
		},
		{
			"shorter names",
			[]string{"get_tree", "get_file"},
			"get_tree", "get_file",
		},
		{
			"read_file variant",
			[]string{"list_repository_files", "read_file"},
			"list_repository_files", "read_file",
		},
		{
			"unfamiliar names matched by hint",
			[]string{"repository_file_tree", "fetch_blob_content"},
			"repository_file_tree", "fetch_blob_content",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := startLoopback(t, catalogueServer(testCase.catalogue...))
			resolved, err := ResolveToolNames(context.Background(), client)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resolved.Tree != testCase.expectedTree {
				t.Errorf("tree: expected %q, got %q", testCase.expectedTree, resolved.Tree)
			}
			if resolved.GetFile != testCase.expectedFile {
				t.Errorf("file: expected %q, got %q", testCase.expectedFile, resolved.GetFile)
			}
		})
	}
}

func TestResolveToolNamesNamesWhatTheServerActuallyOffers(t *testing.T) {
	// An opaque "it didn't work" wastes an afternoon. Listing the real
	// catalogue turns it into a one-line fix.
	client := startLoopback(t, catalogueServer("create_issue", "list_pull_requests"))

	_, err := ResolveToolNames(context.Background(), client)
	if err == nil {
		t.Fatal("a server with no file access must be rejected")
	}
	for _, expectedFragment := range []string{"create_issue", "list_pull_requests", "-mcp-tool-tree"} {
		if !strings.Contains(err.Error(), expectedFragment) {
			t.Errorf("the error should mention %q, got %v", expectedFragment, err)
		}
	}
}
