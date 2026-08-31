package mcpx

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// toolPreferences lists the names a GitHub MCP server might use for each
// capability, best first. Tool naming is not standardised across
// implementations, so hardcoding one server's vocabulary would make the
// integration break on a server that works perfectly well.
var (
	treeToolPreferences = []string{
		"get_repository_tree", "get_tree", "repo_tree", "list_repository_files",
		"list_files", "get_repo_tree", "search_files",
	}
	fileToolPreferences = []string{
		"get_file_contents", "get_file", "read_file", "get_file_content",
		"fetch_file", "repo_get_file",
	}
)

// ResolveToolNames inspects a server's catalogue and works out which tools
// provide the tree and file-read capabilities.
//
// Discovery beats configuration here: the client already has to call
// tools/list, so matching against the real catalogue costs nothing and removes
// the most likely reason this integration fails against an unfamiliar server.
func ResolveToolNames(ctx context.Context, client *Client) (GitHubToolNames, error) {
	descriptors, err := client.ListTools(ctx)
	if err != nil {
		return GitHubToolNames{}, fmt.Errorf("mcp: could not list tools on %s: %w", client.ServerName, err)
	}

	availableNames := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		availableNames = append(availableNames, descriptor.Name)
	}

	// Tree is resolved first because its vocabulary is the more distinctive,
	// then excluded from the file candidates: one tool cannot serve both roles,
	// and a name like "repository_file_tree" would otherwise match each hint.
	resolvedTree := pickBestMatch(availableNames, treeToolPreferences,
		[]string{"tree", "list"}, nil)
	resolved := GitHubToolNames{
		Tree: resolvedTree,
		GetFile: pickBestMatch(availableNames, fileToolPreferences,
			[]string{"content", "blob", "file"}, map[string]bool{resolvedTree: true}),
	}

	var missingCapabilities []string
	if resolved.Tree == "" {
		missingCapabilities = append(missingCapabilities, "listing a repository tree")
	}
	if resolved.GetFile == "" {
		missingCapabilities = append(missingCapabilities, "reading a file")
	}
	if len(missingCapabilities) > 0 {
		sort.Strings(availableNames)
		// Naming what the server does offer turns an opaque failure into a
		// one-line fix with -mcp-tool-tree / -mcp-tool-file.
		return resolved, fmt.Errorf(
			"mcp: server %s offers no tool for %s. It advertises: %s. "+
				"Override with -mcp-tool-tree and -mcp-tool-file if one of these is right",
			client.ServerName, strings.Join(missingCapabilities, " or "),
			strings.Join(availableNames, ", "))
	}
	return resolved, nil
}

// pickBestMatch prefers an exact known name, then falls back to a tool whose
// name contains one of the hint words. Names in excluded are skipped, so a
// capability already claimed by another role is not claimed twice.
func pickBestMatch(availableNames, preferredNames, hintWords []string, excluded map[string]bool) string {
	candidateNames := make([]string, 0, len(availableNames))
	for _, name := range availableNames {
		if !excluded[name] {
			candidateNames = append(candidateNames, name)
		}
	}

	for _, preferred := range preferredNames {
		for _, actual := range candidateNames {
			if strings.EqualFold(actual, preferred) {
				return actual
			}
		}
	}
	for _, hint := range hintWords {
		for _, actual := range candidateNames {
			if strings.Contains(strings.ToLower(actual), hint) {
				return actual
			}
		}
	}
	return ""
}

// Describe renders the resolved catalogue for the -mcp-check diagnostic.
func (toolNames GitHubToolNames) Describe() string {
	return fmt.Sprintf("tree=%q file=%q", toolNames.Tree, toolNames.GetFile)
}
