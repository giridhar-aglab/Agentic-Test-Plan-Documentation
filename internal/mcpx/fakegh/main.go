// Command fakegh is a stand-in GitHub MCP server backed by a local directory.
// It exists to exercise the real MCP transport end to end without credentials.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/mcpx"
)

func main() {
	root := os.Args[1]
	// "no-tree" reproduces GitHub's default toolset, where get_repository_tree
	// is absent because it lives in the "git" toolset and the catalogue is full
	// of other list_* tools that have nothing to do with files.
	noTree := len(os.Args) > 2 && os.Args[2] == "no-tree"

	treeTools := []mcpx.ServerTool{
		{
			Name: "get_repository_tree", Description: "list the repository tree",
			// This mirrors GitHub's own vocabulary, where the tree tool is
			// built on the Git API and calls the reference "tree_sha" while
			// the file tool calls the same idea "ref". A stub that accepted
			// anything would agree with the client instead of checking it.
			InputSchema: json.RawMessage(`{"type":"object","properties":{
					"owner":{"type":"string"},"repo":{"type":"string"},
					"tree_sha":{"type":"string"},"recursive":{"type":"boolean"}},
					"additionalProperties":false}`),
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				if err := rejectUndeclared(args, "owner", "repo", "tree_sha", "recursive"); err != nil {
					return "", err
				}
				type entry struct {
					Path string `json:"path"`
					Type string `json:"type"`
					SHA  string `json:"sha"`
					Size int64  `json:"size"`
				}
				entries := []entry{}
				filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						if d != nil && d.IsDir() && d.Name() == ".git" {
							return filepath.SkipDir
						}
						return nil
					}
					rel, _ := filepath.Rel(root, p)
					info, _ := d.Info()
					body, _ := os.ReadFile(p)
					entries = append(entries, entry{
						Path: filepath.ToSlash(rel), Type: "blob",
						SHA: fmt.Sprintf("%040x", len(body)), Size: info.Size(),
					})
					return nil
				})
				out, _ := json.Marshal(map[string]any{"tree": entries})
				return string(out), nil
			},
		},
		{
			Name: "get_file_contents", Description: "read one file",
			InputSchema: json.RawMessage(`{"type":"object","properties":{
					"owner":{"type":"string"},"repo":{"type":"string"},
					"path":{"type":"string"},"ref":{"type":"string"}},
					"additionalProperties":false}`),
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				if err := rejectUndeclared(args, "owner", "repo", "path", "ref"); err != nil {
					return "", err
				}
				var p struct {
					Path string `json:"path"`
				}
				json.Unmarshal(args, &p)

				target := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(p.Path, "/")))
				// A directory returns a listing, the way GitHub's contents API
				// does. This is what makes the tool usable for enumeration when
				// no tree tool is available.
				if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
					type row struct {
						Name string `json:"name"`
						Path string `json:"path"`
						Type string `json:"type"`
						SHA  string `json:"sha"`
						Size int64  `json:"size"`
					}
					children, _ := os.ReadDir(target)
					rows := []row{}
					for _, child := range children {
						if child.Name() == ".git" {
							continue
						}
						childInfo, _ := child.Info()
						kind := "file"
						if child.IsDir() {
							kind = "dir"
						}
						childPath := strings.TrimPrefix(
							filepath.ToSlash(filepath.Join(strings.TrimPrefix(p.Path, "/"), child.Name())), "/")
						rows = append(rows, row{
							Name: child.Name(), Path: childPath, Type: kind,
							SHA: fmt.Sprintf("%040x", childInfo.Size()), Size: childInfo.Size(),
						})
					}
					out, _ := json.Marshal(rows)
					return string(out), nil
				}

				body, err := os.ReadFile(target)
				if err != nil {
					return "", fmt.Errorf("404 Not Found: %s", p.Path)
				}
				// base64, the way the real GitHub API returns blobs
				out, _ := json.Marshal(map[string]string{
					"content":  base64.StdEncoding.EncodeToString(body),
					"encoding": "base64",
				})
				return string(out), nil
			},
		},
	}

	server := &mcpx.Server{
		Name: "fake-github", Version: "1",
		Tools: []mcpx.ServerTool{
			{Name: "create_issue", Description: "noise", InputSchema: json.RawMessage(`{"type":"object"}`),
				Handler: func(context.Context, json.RawMessage) (string, error) { return "{}", nil }},
			// These are the decoys that broke resolution in production: they
			// list things, they are not files, and one of them was chosen.
			{Name: "list_branches", Description: "noise", InputSchema: json.RawMessage(`{"type":"object"}`),
				Handler: func(context.Context, json.RawMessage) (string, error) {
					return `[{"name":"master","sha":"1abc373","protected":false}]`, nil
				}},
			{Name: "list_commits", Description: "noise", InputSchema: json.RawMessage(`{"type":"object"}`),
				Handler: func(context.Context, json.RawMessage) (string, error) { return "[]", nil }},
			{Name: "list_tags", Description: "noise", InputSchema: json.RawMessage(`{"type":"object"}`),
				Handler: func(context.Context, json.RawMessage) (string, error) { return "[]", nil }},
		},
	}
	if noTree {
		// Only the file tool survives, and it must list directories.
		server.Tools = append(server.Tools, treeTools[1])
	} else {
		server.Tools = append(server.Tools, treeTools...)
	}
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil &&
		!strings.Contains(err.Error(), "context") {
		fmt.Fprintln(os.Stderr, err)
	}
}

// rejectUndeclared fails the way a schema-validating server fails, so a client
// sending a parameter this tool never advertised is caught here rather than in
// production.
func rejectUndeclared(args json.RawMessage, declared ...string) error {
	var received map[string]any
	if err := json.Unmarshal(args, &received); err != nil {
		return nil
	}
	allowed := map[string]bool{}
	for _, name := range declared {
		allowed[name] = true
	}
	for name := range received {
		if !allowed[name] {
			return fmt.Errorf("Invalid argument %q: not accepted by this tool (accepts: %s)",
				name, strings.Join(declared, ", "))
		}
	}
	return nil
}
