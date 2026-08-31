package mcpx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// GitHubSource reads a repository through a GitHub MCP server.
//
// It satisfies repo.Source, which is why nothing downstream — not the Surveyor,
// not the Analyst, not the parsers — knows or cares that the bytes arrive over
// a network. Swapping a local checkout for GitHub is a wiring change in main.
type GitHubSource struct {
	Client     *Client
	Owner      string
	Repository string
	// Ref is a branch, tag or commit SHA. Empty means the default branch.
	Ref string
	// MaxFiles enforces the stated repository ceiling.
	MaxFiles int
	// ToolNames lets the wiring point at whichever server is configured, since
	// tool naming is not standardised across GitHub MCP implementations.
	ToolNames GitHubToolNames
	// Logf reports degradations — a tree tool that had to be abandoned, a
	// subdirectory that could not be read. Silence here would make a partial
	// listing look like a complete one.
	Logf func(format string, arguments ...any)

	mutex         sync.Mutex
	cachedEntries []repo.Entry
	// contentCache holds file bodies for the run. Four hundred blob fetches
	// meet GitHub's secondary rate limiter, so not fetching the same file twice
	// matters more here than it ever did against a local disk.
	contentCache map[string]string
}

// GitHubToolNames names the remote tools this source depends on, along with
// the parameters each one declares.
//
// The parameter sets come from the server's own schemas. They matter because
// servers disagree about vocabulary — GitHub's tree tool is built on the Git
// API and names its reference "tree_sha", while its file tool calls the same
// idea "ref" — and sending a parameter a tool does not declare is rejected
// outright by any server validating its schema. An empty set means "not
// discovered", in which case every argument is sent and the server decides.
type GitHubToolNames struct {
	Tree              string
	GetFile           string
	TreeParameters    map[string]bool
	GetFileParameters map[string]bool
}

// refParameterNames are the names a tool might give the "which commit"
// argument, best first.
var refParameterNames = []string{"ref", "tree_sha", "sha", "branch", "commit"}

// buildArguments keeps only the parameters a tool declares, and routes the ref
// to whichever name that tool uses for it. When nothing was discovered every
// argument is passed through unchanged.
func buildArguments(declared map[string]bool, fixed map[string]any, ref string, extras map[string]any) ([]byte, error) {
	arguments := map[string]any{}
	for key, value := range fixed {
		arguments[key] = value
	}

	if len(declared) == 0 {
		if ref != "" {
			arguments["ref"] = ref
		}
		for key, value := range extras {
			arguments[key] = value
		}
		return json.Marshal(arguments)
	}

	for key := range arguments {
		if !declared[key] {
			delete(arguments, key)
		}
	}
	for _, candidate := range refParameterNames {
		if !declared[candidate] {
			continue
		}
		switch {
		case ref != "":
			arguments[candidate] = ref
		case candidate != "ref":
			// A tree tool built on the Git API needs an explicit tree-ish; the
			// default branch is not implied the way it is for a file read.
			arguments[candidate] = "HEAD"
		}
		break
	}
	for key, value := range extras {
		if declared[key] {
			arguments[key] = value
		}
	}
	return json.Marshal(arguments)
}

// DefaultGitHubToolNames matches the common GitHub MCP server naming.
func DefaultGitHubToolNames() GitHubToolNames {
	return GitHubToolNames{Tree: "get_repository_tree", GetFile: "get_file_contents"}
}

// NewGitHubSource builds a source over an MCP client.
func NewGitHubSource(client *Client, owner, repository, ref string, maxFiles int) *GitHubSource {
	return &GitHubSource{
		Client: client, Owner: owner, Repository: repository, Ref: ref,
		MaxFiles: maxFiles, ToolNames: DefaultGitHubToolNames(),
		contentCache: map[string]string{},
	}
}

func (githubSource *GitHubSource) Name() string {
	return fmt.Sprintf("github:%s/%s", githubSource.Owner, githubSource.Repository)
}

// treeEntry is one node in the response from the tree tool. Both the REST
// shape and the flattened shapes some servers return are accepted, because
// pinning this to one server's exact output would make the integration
// brittle for no benefit.
type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type treeResponse struct {
	Tree      []treeEntry `json:"tree"`
	Entries   []treeEntry `json:"entries"`
	Files     []treeEntry `json:"files"`
	Truncated bool        `json:"truncated"`
}

// Tree fetches the repository listing once per run.
//
// A tree tool is the fast path, not the only one. When the server offers none —
// GitHub's lives in the "git" toolset, which is off by default — or when the
// one it offers returns something this decoder cannot recognise, the listing is
// built by walking directories through the file tool instead. One unfamiliar
// tool should degrade the run, not end it.
func (githubSource *GitHubSource) Tree(ctx context.Context) ([]repo.Entry, error) {
	githubSource.mutex.Lock()
	if githubSource.cachedEntries != nil {
		entriesCopy := append([]repo.Entry{}, githubSource.cachedEntries...)
		githubSource.mutex.Unlock()
		return entriesCopy, nil
	}
	githubSource.mutex.Unlock()

	entries, err := githubSource.treeViaTreeTool(ctx)
	if err != nil {
		if errors.Is(err, errTruncatedTree) {
			// Truncation is not a capability gap. The server saw the whole
			// repository and told us it did not send all of it, so walking
			// around it would produce the same silent half-plan by a slower
			// route.
			return nil, err
		}
		if githubSource.ToolNames.Tree != "" {
			githubSource.logf("github: %v", err)
			githubSource.logf("github: falling back to walking directories through %q",
				githubSource.ToolNames.GetFile)
		}
		entries, err = githubSource.treeViaDirectoryWalk(ctx)
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(entries, func(leftIndex, rightIndex int) bool {
		return entries[leftIndex].Path < entries[rightIndex].Path
	})
	if githubSource.MaxFiles > 0 && len(entries) > githubSource.MaxFiles {
		return nil, &repo.ErrTooManyFiles{FileCount: len(entries), MaxFiles: githubSource.MaxFiles}
	}

	githubSource.mutex.Lock()
	githubSource.cachedEntries = entries
	githubSource.mutex.Unlock()
	return entries, nil
}

func (githubSource *GitHubSource) logf(format string, arguments ...any) {
	if githubSource.Logf != nil {
		githubSource.Logf(format, arguments...)
	}
}

// errNoTreeTool marks the absence of a tree tool, which is a routine
// configuration rather than a failure worth reporting.
var errNoTreeTool = errors.New("no tree tool was resolved")

func (githubSource *GitHubSource) treeViaTreeTool(ctx context.Context) ([]repo.Entry, error) {
	if githubSource.ToolNames.Tree == "" {
		return nil, errNoTreeTool
	}

	arguments, err := buildArguments(
		githubSource.ToolNames.TreeParameters,
		map[string]any{"owner": githubSource.Owner, "repo": githubSource.Repository},
		githubSource.Ref,
		map[string]any{"recursive": true},
	)
	if err != nil {
		return nil, err
	}

	content, isError, err := githubSource.Client.CallTool(ctx, githubSource.ToolNames.Tree, arguments)
	if err != nil {
		return nil, fmt.Errorf("github: tree: %w", err)
	}
	if isError {
		return nil, fmt.Errorf("github: tree: %s", content)
	}

	rawEntries, err := decodeTreeEntries(content)
	if err != nil {
		return nil, fmt.Errorf("%q did not return a usable file listing: %w", githubSource.ToolNames.Tree, err)
	}

	var decoded treeResponse
	if json.Unmarshal([]byte(content), &decoded) == nil && decoded.Truncated {
		// A truncated tree would silently produce a plan that looks complete
		// over a repository it never fully saw. This one is not recoverable by
		// walking either, so it stops the run rather than degrading.
		return nil, errTruncatedTree
	}

	entries := make([]repo.Entry, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		if rawEntry.Type != "" && rawEntry.Type != "blob" && rawEntry.Type != "file" {
			continue
		}
		entries = append(entries, repo.Entry{
			Path: rawEntry.Path, BlobSHA: rawEntry.SHA, SizeBytes: rawEntry.Size,
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%q returned no files", githubSource.ToolNames.Tree)
	}
	return entries, nil
}

var errTruncatedTree = errors.New(
	"github: the repository tree was truncated by the server; narrow the scan to a subdirectory")

// decodeTreeEntries accepts the shapes a tree tool may return: an envelope
// keyed by tree/entries/files, or a bare array.
//
// It insists the result actually looks like files. A server with no tree tool
// enabled once resolved to list_branches, whose objects carry a name and a sha
// but no path — enough to decode, not enough to mean anything, and the run died
// on the far side of a successful call.
func decodeTreeEntries(content string) ([]treeEntry, error) {
	trimmed := strings.TrimSpace(content)

	var rawEntries []treeEntry
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &rawEntries); err != nil {
			return nil, fmt.Errorf("decode (%.120s): %w", trimmed, err)
		}
	} else {
		var decoded treeResponse
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil, fmt.Errorf("decode (%.120s): %w", trimmed, err)
		}
		for _, candidate := range [][]treeEntry{decoded.Tree, decoded.Entries, decoded.Files} {
			if len(candidate) > 0 {
				rawEntries = candidate
				break
			}
		}
	}

	if len(rawEntries) == 0 {
		return nil, fmt.Errorf("no recognisable file listing in (%.200s)", trimmed)
	}
	for _, rawEntry := range rawEntries {
		if rawEntry.Path != "" {
			return rawEntries, nil
		}
	}
	return nil, fmt.Errorf("the entries carry no paths, so this is not a file listing (%.200s)", trimmed)
}

// directoryEntry is one row of a directory listing from the file tool.
type directoryEntry struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

// maxDirectoriesWalked bounds the fallback. A repository deep enough to exceed
// this is past the stated ceiling anyway, and an unbounded walk against a
// rate-limited API is the wrong way to find that out.
const maxDirectoriesWalked = 400

// treeViaDirectoryWalk enumerates files one directory at a time through the
// file tool, which returns a listing when handed a directory path.
func (githubSource *GitHubSource) treeViaDirectoryWalk(ctx context.Context) ([]repo.Entry, error) {
	entries := []repo.Entry{}
	pending := []string{""}
	directoriesWalked := 0

	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		directoryPath := pending[0]
		pending = pending[1:]

		directoriesWalked++
		if directoriesWalked > maxDirectoriesWalked {
			return nil, fmt.Errorf(
				"github: this repository has more than %d directories; "+
					"narrow the scan with -source on a subdirectory, or enable a tree tool "+
					"on the server (GITHUB_TOOLSETS=repos,git) so the whole listing arrives in one call",
				maxDirectoriesWalked)
		}

		listing, err := githubSource.listDirectory(ctx, directoryPath)
		if err != nil {
			if directoryPath == "" {
				return nil, err
			}
			// One unreadable subdirectory should cost that subdirectory, not
			// the run. The gap is visible because its files never appear.
			githubSource.logf("github: skipping %s: %v", directoryPath, err)
			continue
		}

		for _, row := range listing {
			path := row.Path
			if path == "" {
				path = strings.TrimPrefix(directoryPath+"/"+row.Name, "/")
			}
			switch row.Type {
			case "dir", "tree":
				pending = append(pending, path)
			default:
				entries = append(entries, repo.Entry{
					Path: path, BlobSHA: row.SHA, SizeBytes: row.Size,
				})
			}
		}

		if githubSource.MaxFiles > 0 && len(entries) > githubSource.MaxFiles {
			return nil, &repo.ErrTooManyFiles{
				FileCount: len(entries), MaxFiles: githubSource.MaxFiles,
			}
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf(
			"github: walking %s/%s through %q produced no files",
			githubSource.Owner, githubSource.Repository, githubSource.ToolNames.GetFile)
	}
	return entries, nil
}

func (githubSource *GitHubSource) listDirectory(ctx context.Context, directoryPath string) ([]directoryEntry, error) {
	requestPath := directoryPath
	if requestPath == "" {
		requestPath = "/"
	}
	arguments, err := buildArguments(
		githubSource.ToolNames.GetFileParameters,
		map[string]any{
			"owner": githubSource.Owner, "repo": githubSource.Repository, "path": requestPath,
		},
		githubSource.Ref, nil,
	)
	if err != nil {
		return nil, err
	}

	content, isError, err := githubSource.Client.CallTool(ctx, githubSource.ToolNames.GetFile, arguments)
	if err != nil {
		return nil, err
	}
	if isError {
		return nil, errors.New(content)
	}

	trimmed := strings.TrimSpace(content)
	var listing []directoryEntry
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &listing); err != nil {
			return nil, fmt.Errorf("decode listing (%.120s): %w", trimmed, err)
		}
		return listing, nil
	}

	var envelope struct {
		Entries []directoryEntry `json:"entries"`
		Files   []directoryEntry `json:"files"`
		Tree    []directoryEntry `json:"tree"`
	}
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return nil, fmt.Errorf("decode listing (%.120s): %w", trimmed, err)
	}
	for _, candidate := range [][]directoryEntry{envelope.Entries, envelope.Files, envelope.Tree} {
		if len(candidate) > 0 {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("no directory listing in (%.200s)", trimmed)
}

// fileResponse covers the shapes a GitHub MCP server may return for a blob.
type fileResponse struct {
	Content  string `json:"content"`
	Text     string `json:"text"`
	Encoding string `json:"encoding"`
}

// ReadFile fetches one file, memoised for the run.
func (githubSource *GitHubSource) ReadFile(ctx context.Context, path string) (string, error) {
	githubSource.mutex.Lock()
	if cachedContent, found := githubSource.contentCache[path]; found {
		githubSource.mutex.Unlock()
		return cachedContent, nil
	}
	githubSource.mutex.Unlock()

	arguments, err := buildArguments(
		githubSource.ToolNames.GetFileParameters,
		map[string]any{
			"owner": githubSource.Owner, "repo": githubSource.Repository, "path": path,
		},
		githubSource.Ref,
		nil,
	)
	if err != nil {
		return "", err
	}

	content, isError, err := githubSource.Client.CallTool(ctx, githubSource.ToolNames.GetFile, arguments)
	if err != nil {
		return "", fmt.Errorf("github: read %s: %w", path, err)
	}
	if isError {
		if strings.Contains(strings.ToLower(content), "not found") {
			return "", &repo.ErrNotFound{Path: path}
		}
		return "", fmt.Errorf("github: read %s: %s", path, content)
	}

	fileText := extractFileText(content)
	if strings.TrimSpace(fileText) == "" {
		// An empty string here is almost never an empty file. It is a content
		// shape this client did not understand, and passing it on as success
		// leaves the analyst reading nothing, repeatedly.
		return "", fmt.Errorf(
			"github: %s returned no usable content for %s (raw reply: %.200s)",
			githubSource.ToolNames.GetFile, path, content)
	}

	githubSource.mutex.Lock()
	githubSource.contentCache[path] = fileText
	githubSource.mutex.Unlock()
	return fileText, nil
}

// extractFileText handles servers that return the raw file and servers that
// return a JSON envelope around it.
func extractFileText(rawContent string) string {
	trimmedContent := strings.TrimSpace(rawContent)
	if !strings.HasPrefix(trimmedContent, "{") {
		return rawContent
	}
	var decoded fileResponse
	if err := json.Unmarshal([]byte(trimmedContent), &decoded); err != nil {
		return rawContent
	}
	if decoded.Text != "" {
		return decoded.Text
	}
	if decoded.Content != "" && decoded.Encoding != "base64" {
		return decoded.Content
	}
	if decoded.Content != "" {
		if decodedBytes, err := decodeBase64(decoded.Content); err == nil {
			return string(decodedBytes)
		}
	}
	return rawContent
}

// decodeBase64 tolerates the newline-wrapped base64 the GitHub REST API emits.
func decodeBase64(encoded string) ([]byte, error) {
	cleaned := strings.NewReplacer("\n", "", "\r", "", " ", "").Replace(encoded)
	return base64.StdEncoding.DecodeString(cleaned)
}
