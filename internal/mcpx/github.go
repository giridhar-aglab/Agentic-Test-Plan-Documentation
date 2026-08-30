package mcpx

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

	mutex         sync.Mutex
	cachedEntries []repo.Entry
	// contentCache holds file bodies for the run. Four hundred blob fetches
	// meet GitHub's secondary rate limiter, so not fetching the same file twice
	// matters more here than it ever did against a local disk.
	contentCache map[string]string
}

// GitHubToolNames names the remote tools this source depends on.
type GitHubToolNames struct {
	Tree    string
	GetFile string
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
	Truncated bool        `json:"truncated"`
}

// Tree fetches the repository listing once per run.
func (githubSource *GitHubSource) Tree(ctx context.Context) ([]repo.Entry, error) {
	githubSource.mutex.Lock()
	if githubSource.cachedEntries != nil {
		entriesCopy := append([]repo.Entry{}, githubSource.cachedEntries...)
		githubSource.mutex.Unlock()
		return entriesCopy, nil
	}
	githubSource.mutex.Unlock()

	arguments, err := json.Marshal(map[string]any{
		"owner": githubSource.Owner, "repo": githubSource.Repository,
		"ref": githubSource.Ref, "recursive": true,
	})
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

	var decoded treeResponse
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return nil, fmt.Errorf("github: decode tree (%.120s): %w", content, err)
	}
	rawEntries := decoded.Tree
	if len(rawEntries) == 0 {
		rawEntries = decoded.Entries
	}
	if decoded.Truncated {
		// A truncated tree would silently produce a plan that looks complete
		// over a repository it never fully saw.
		return nil, fmt.Errorf("github: the repository tree was truncated by the server; " +
			"narrow the scan to a subdirectory")
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

	arguments, err := json.Marshal(map[string]any{
		"owner": githubSource.Owner, "repo": githubSource.Repository,
		"path": path, "ref": githubSource.Ref,
	})
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
