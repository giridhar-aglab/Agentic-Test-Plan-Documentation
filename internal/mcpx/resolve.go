package mcpx

import (
	"context"
	"encoding/json"
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
		"list_files", "get_repo_tree",
	}
	fileToolPreferences = []string{
		"get_file_contents", "get_file", "read_file", "get_file_content",
		"fetch_file", "repo_get_file",
	}

	// A hint word has to actually mean what the role needs. "list" did not: on
	// a real GitHub server with no tree tool enabled it matched list_branches,
	// whose branch objects decode into nothing and killed the run half a second
	// in. The tree hints are plural on purpose — "file" singular is the reading
	// tool, and a hint that matches both roles picks a fight over one tool.
	treeHintWords = []string{"tree", "files"}
	fileHintWords = []string{"content", "blob", "file"}

	// disqualifyingWords name the many things a GitHub server can list that
	// are not files. A name containing one of these is never a candidate,
	// however well the rest of it reads.
	disqualifyingWords = []string{
		"branch", "commit", "tag", "release", "issue", "pull", "workflow",
		"alert", "notification", "discussion", "gist", "star", "label",
		"secret", "collaborator", "team", "project", "review", "comment",
		"milestone", "webhook", "deployment", "package", "dependab", "job",
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
	// The file tool is resolved first because it is the one with no substitute,
	// then excluded from the tree candidates: one tool cannot serve both roles,
	// and a name like "repository_file_tree" would otherwise match each hint.
	resolvedFile := pickBestMatch(availableNames, fileToolPreferences, fileHintWords, nil)
	resolved := GitHubToolNames{
		GetFile: resolvedFile,
		Tree: pickBestMatch(availableNames, treeToolPreferences,
			treeHintWords, map[string]bool{resolvedFile: true}),
	}
	// Names alone are not enough: servers disagree about parameter vocabulary
	// too — one wants "ref", another "tree_sha". The schema is already in hand,
	// so read it rather than guess.
	resolved.TreeParameters = declaredParameters(descriptors, resolved.Tree)
	resolved.GetFileParameters = declaredParameters(descriptors, resolved.GetFile)

	// Reading a file is the one capability with no substitute. A tree tool is
	// merely the fast way to enumerate: GitHub ships theirs in the "git"
	// toolset, which is off by default, and refusing to run without it would
	// block the common configuration for no reason. Walking directories
	// through the file tool costs one call per directory and gets there too.
	if resolved.GetFile == "" {
		sort.Strings(availableNames)
		// Naming what the server does offer turns an opaque failure into a
		// one-line fix with -mcp-tool-tree / -mcp-tool-file.
		return resolved, fmt.Errorf(
			"mcp: server %s offers no tool for reading a file. It advertises: %s. "+
				"Override with -mcp-tool-file if one of these is right",
			client.ServerName, strings.Join(availableNames, ", "))
	}
	return resolved, nil
}

// pickBestMatch prefers an exact known name, then falls back to a tool whose
// name contains one of the hint words. Names in excluded are skipped, so a
// capability already claimed by another role is not claimed twice.
func pickBestMatch(availableNames, preferredNames, hintWords []string, excluded map[string]bool) string {
	candidateNames := make([]string, 0, len(availableNames))
	for _, name := range availableNames {
		if !excluded[name] && !isDisqualified(name) {
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

// isDisqualified rejects a name that plainly describes something other than
// repository files.
func isDisqualified(toolName string) bool {
	lowered := strings.ToLower(toolName)
	for _, word := range disqualifyingWords {
		if strings.Contains(lowered, word) {
			return true
		}
	}
	return false
}

// declaredParameters extracts the property names a tool's input schema
// declares. An empty result means the schema was absent or unreadable, which
// callers must treat as "unknown", never as "accepts nothing".
func declaredParameters(descriptors []ToolDescriptor, toolName string) map[string]bool {
	if toolName == "" {
		return nil
	}
	for _, descriptor := range descriptors {
		if descriptor.Name != toolName || len(descriptor.InputSchema) == 0 {
			continue
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
			return nil
		}
		if len(schema.Properties) == 0 {
			return nil
		}
		declared := make(map[string]bool, len(schema.Properties))
		for propertyName := range schema.Properties {
			declared[propertyName] = true
		}
		return declared
	}
	return nil
}

// Describe renders the resolved catalogue for the -mcp-check diagnostic.
func (toolNames GitHubToolNames) Describe() string {
	treeDescription := fmt.Sprintf("%q", toolNames.Tree)
	if toolNames.Tree == "" {
		treeDescription = "none — will walk directories through the file tool"
	}
	description := fmt.Sprintf("tree=%s file=%q", treeDescription, toolNames.GetFile)
	if parameters := sortedKeys(toolNames.TreeParameters); len(parameters) > 0 {
		description += fmt.Sprintf("\n              tree accepts: %s", strings.Join(parameters, ", "))
	}
	if parameters := sortedKeys(toolNames.GetFileParameters); len(parameters) > 0 {
		description += fmt.Sprintf("\n              file accepts: %s", strings.Join(parameters, ", "))
	}
	return description
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
