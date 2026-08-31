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
	for _, expectedFragment := range []string{"create_issue", "list_pull_requests", "-mcp-tool-file"} {
		if !strings.Contains(err.Error(), expectedFragment) {
			t.Errorf("the error should mention %q, got %v", expectedFragment, err)
		}
	}
}

// schemaServer advertises tools with real input schemas, so parameter
// discovery can be exercised rather than only name matching.
func schemaServer(schemasByTool map[string]string) *Server {
	server := &Server{Name: "github", Version: "1"}
	for toolName, schema := range schemasByTool {
		server.Tools = append(server.Tools, ServerTool{
			Name: toolName, Description: "a tool",
			InputSchema: json.RawMessage(schema),
			Handler: func(ctx context.Context, arguments json.RawMessage) (string, error) {
				return "{}", nil
			},
		})
	}
	return server
}

func TestResolveReadsEachToolsParameterVocabulary(t *testing.T) {
	// GitHub's own server names the same idea differently in two tools: the
	// tree tool is built on the Git API and wants "tree_sha", while the file
	// tool wants "ref". Sending the wrong one is a 400, so the schemas are read
	// rather than assumed.
	client := startLoopback(t, schemaServer(map[string]string{
		"get_repository_tree": `{"type":"object","properties":{
			"owner":{"type":"string"},"repo":{"type":"string"},
			"tree_sha":{"type":"string"},"recursive":{"type":"boolean"},
			"path_filter":{"type":"array"}}}`,
		"get_file_contents": `{"type":"object","properties":{
			"owner":{"type":"string"},"repo":{"type":"string"},
			"path":{"type":"string"},"ref":{"type":"string"}}}`,
	}))

	resolved, err := ResolveToolNames(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.TreeParameters["tree_sha"] || resolved.TreeParameters["ref"] {
		t.Fatalf("tree parameters were misread: %v", resolved.TreeParameters)
	}
	if !resolved.GetFileParameters["ref"] {
		t.Fatalf("file parameters were misread: %v", resolved.GetFileParameters)
	}
	// The diagnostic has to show this, or a vocabulary mismatch is invisible
	// until a run fails halfway through.
	if !strings.Contains(resolved.Describe(), "tree_sha") {
		t.Errorf("-mcp-check must show the accepted parameters, got %q", resolved.Describe())
	}
}

func TestArgumentsAreRoutedToTheParameterNamesAToolDeclares(t *testing.T) {
	treeParameters := map[string]bool{
		"owner": true, "repo": true, "tree_sha": true, "recursive": true,
	}
	encoded, err := buildArguments(treeParameters,
		map[string]any{"owner": "gin-gonic", "repo": "gin", "path": "ignored"},
		"v1.9.1", map[string]any{"recursive": true, "depth": 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var arguments map[string]any
	if err := json.Unmarshal(encoded, &arguments); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if arguments["tree_sha"] != "v1.9.1" {
		t.Errorf("the ref must reach the name this tool uses, got %v", arguments)
	}
	if _, present := arguments["ref"]; present {
		t.Errorf("an undeclared parameter must not be sent, got %v", arguments)
	}
	for _, undeclared := range []string{"path", "depth"} {
		if _, present := arguments[undeclared]; present {
			t.Errorf("%q is not declared by this tool and must be dropped, got %v", undeclared, arguments)
		}
	}
}

func TestATreeToolNeedingATreeishGetsOneWhenNoRefWasGiven(t *testing.T) {
	// A file read defaults to the default branch; a Git-API tree call does not.
	// Omitting the tree-ish there fails, so the default has to be explicit.
	encoded, _ := buildArguments(
		map[string]bool{"owner": true, "repo": true, "tree_sha": true},
		map[string]any{"owner": "o", "repo": "r"}, "", nil)

	var arguments map[string]any
	_ = json.Unmarshal(encoded, &arguments)
	if arguments["tree_sha"] != "HEAD" {
		t.Fatalf("expected an explicit default tree-ish, got %v", arguments)
	}
}

func TestNoRefIsInventedForAToolThatDefaultsOnItsOwn(t *testing.T) {
	// "ref" is optional on a file read and means the default branch. Sending
	// "HEAD" there would be a guess where the server already has a right answer.
	encoded, _ := buildArguments(
		map[string]bool{"owner": true, "repo": true, "path": true, "ref": true},
		map[string]any{"owner": "o", "repo": "r", "path": "a.go"}, "", nil)

	var arguments map[string]any
	_ = json.Unmarshal(encoded, &arguments)
	if _, present := arguments["ref"]; present {
		t.Fatalf("no ref should be invented, got %v", arguments)
	}
}

func TestAnUndiscoveredSchemaSendsEverythingAndLetsTheServerDecide(t *testing.T) {
	// An empty parameter set means "not discovered", never "accepts nothing".
	// Treating the two the same would send an empty request to a server whose
	// schema simply could not be read.
	encoded, _ := buildArguments(nil,
		map[string]any{"owner": "o", "repo": "r"}, "main", map[string]any{"recursive": true})

	var arguments map[string]any
	_ = json.Unmarshal(encoded, &arguments)
	if arguments["owner"] != "o" || arguments["ref"] != "main" || arguments["recursive"] != true {
		t.Fatalf("everything should be passed through when nothing was discovered, got %v", arguments)
	}
}

func TestAToolThatListsSomethingOtherThanFilesIsNeverChosen(t *testing.T) {
	// This is the bug that reached production. GitHub ships get_repository_tree
	// in the "git" toolset, which is off by default, so a real catalogue often
	// has no tree tool at all. A hint word of "list" then matched list_branches,
	// whose branch objects decode into nothing — the run died half a second in,
	// on the far side of a call that had succeeded.
	client := startLoopback(t, catalogueServer(
		"list_branches", "list_commits", "list_tags", "list_releases",
		"list_issues", "list_pull_requests", "get_file_contents", "create_issue",
	))

	resolved, err := ResolveToolNames(context.Background(), client)
	if err != nil {
		t.Fatalf("a catalogue with a file tool must resolve: %v", err)
	}
	if resolved.Tree != "" {
		t.Errorf("no tool here lists files, so tree must stay empty, got %q", resolved.Tree)
	}
	if resolved.GetFile != "get_file_contents" {
		t.Errorf("the file tool must still be found, got %q", resolved.GetFile)
	}
}

func TestAMissingTreeToolIsNotFatalButAMissingFileToolIs(t *testing.T) {
	// Reading a file has no substitute. Listing does: directories can be walked
	// through the file tool, so refusing to run without a tree tool would block
	// the default GitHub configuration for no reason.
	withFileOnly := startLoopback(t, catalogueServer("get_file_contents"))
	if _, err := ResolveToolNames(context.Background(), withFileOnly); err != nil {
		t.Fatalf("a file tool alone is enough to run: %v", err)
	}

	withNeither := startLoopback(t, catalogueServer("list_branches", "create_issue"))
	_, err := ResolveToolNames(context.Background(), withNeither)
	if err == nil {
		t.Fatal("a catalogue with no file tool must fail loudly")
	}
	if !strings.Contains(err.Error(), "list_branches") {
		t.Errorf("the error must name what the server does offer, got %v", err)
	}
}

func TestBranchObjectsAreRejectedAsAFileListing(t *testing.T) {
	// The exact payload that got through: valid JSON, decodes cleanly into
	// entries, and means nothing because none of them is a file.
	branchListing := `[{"name":"benchmarks","sha":"1abc373","protected":false},
		{"name":"better-bind-errors","sha":"2def","protected":false}]`
	if _, err := decodeTreeEntries(branchListing); err == nil {
		t.Fatal("a branch listing must not be accepted as a file listing")
	}

	// A bare array of real entries is fine, though: some servers return one.
	entries, err := decodeTreeEntries(`[{"path":"a.go","type":"blob","sha":"x","size":10}]`)
	if err != nil {
		t.Fatalf("a bare array of file entries must be accepted: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "a.go" {
		t.Fatalf("unexpected entries %+v", entries)
	}
}
