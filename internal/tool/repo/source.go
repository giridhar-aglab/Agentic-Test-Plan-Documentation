// Package repo provides the repository-access tools.
//
// In production these are backed by the GitHub MCP server. The Source
// interface is the seam: a local directory implementation lets the whole
// pipeline be tested against a fixture repository with no network, and the MCP
// client drops in behind the same interface.
package repo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is one file in the repository tree.
type Entry struct {
	Path      string `json:"path"`
	BlobSHA   string `json:"blobSha"`
	SizeBytes int64  `json:"sizeBytes"`
}

// Source is where file contents come from. Blob SHAs are the cache key
// throughout the system, so an implementation must return git's own object
// hash rather than inventing one.
type Source interface {
	Name() string
	Tree(ctx context.Context) ([]Entry, error)
	ReadFile(ctx context.Context, path string) (string, error)
}

// ErrNotFound is returned for a path that is not in the tree.
type ErrNotFound struct{ Path string }

func (notFound *ErrNotFound) Error() string { return "repo: no such path: " + notFound.Path }

// LocalSource reads a directory on disk. Used for fixtures and for the
// shallow-clone fallback when the GitHub server is unreachable.
type LocalSource struct {
	RootDirectory string
	// MaxFiles enforces the stated repository ceiling. Refusing loudly above
	// the limit beats silently truncating and producing a plan that looks
	// complete.
	MaxFiles int

	cachedEntries []Entry
}

// NewLocalSource builds a source over a directory.
func NewLocalSource(rootDirectory string, maxFiles int) *LocalSource {
	return &LocalSource{RootDirectory: rootDirectory, MaxFiles: maxFiles}
}

func (localSource *LocalSource) Name() string { return "local:" + localSource.RootDirectory }

// ErrTooManyFiles reports a repository above the supported ceiling.
type ErrTooManyFiles struct {
	FileCount int
	MaxFiles  int
}

func (tooMany *ErrTooManyFiles) Error() string {
	return fmt.Sprintf("repo: repository has %d analysable files, above the supported ceiling of %d; "+
		"narrow the scan with an include path", tooMany.FileCount, tooMany.MaxFiles)
}

var skippedDirectoryNames = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".venv": true,
	"__pycache__": true, "dist": true, "build": true, ".idea": true, ".vscode": true,
}

// Tree walks the directory, applying the same ignore rules the MCP-backed
// source applies server-side.
func (localSource *LocalSource) Tree(ctx context.Context) ([]Entry, error) {
	if localSource.cachedEntries != nil {
		return localSource.cachedEntries, nil
	}

	entries := []Entry{}
	walkErr := filepath.WalkDir(localSource.RootDirectory, func(absolutePath string, directoryEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if directoryEntry.IsDir() {
			if skippedDirectoryNames[directoryEntry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		relativePath, relErr := filepath.Rel(localSource.RootDirectory, absolutePath)
		if relErr != nil {
			return relErr
		}
		fileInfo, infoErr := directoryEntry.Info()
		if infoErr != nil {
			return infoErr
		}
		contentBytes, readErr := os.ReadFile(absolutePath)
		if readErr != nil {
			return readErr
		}
		entries = append(entries, Entry{
			Path:      filepath.ToSlash(relativePath),
			BlobSHA:   GitBlobSHA(contentBytes),
			SizeBytes: fileInfo.Size(),
		})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("repo: walk %s: %w", localSource.RootDirectory, walkErr)
	}

	sort.Slice(entries, func(leftIndex, rightIndex int) bool {
		return entries[leftIndex].Path < entries[rightIndex].Path
	})
	if localSource.MaxFiles > 0 && len(entries) > localSource.MaxFiles {
		return nil, &ErrTooManyFiles{FileCount: len(entries), MaxFiles: localSource.MaxFiles}
	}
	localSource.cachedEntries = entries
	return entries, nil
}

// ReadFile returns one file's contents.
func (localSource *LocalSource) ReadFile(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cleanedPath := filepath.Clean(path)
	if strings.HasPrefix(cleanedPath, "..") || filepath.IsAbs(cleanedPath) {
		return "", &ErrNotFound{Path: path}
	}
	contentBytes, err := os.ReadFile(filepath.Join(localSource.RootDirectory, cleanedPath))
	if err != nil {
		if os.IsNotExist(err) {
			return "", &ErrNotFound{Path: path}
		}
		return "", fmt.Errorf("repo: read %s: %w", path, err)
	}
	return string(contentBytes), nil
}

// GitBlobSHA computes git's own object hash for content, so cache keys match
// what GitHub reports and a file that has not changed is a guaranteed hit.
func GitBlobSHA(content []byte) string {
	hasher := sha1.New()
	fmt.Fprintf(hasher, "blob %d\x00", len(content))
	hasher.Write(content)
	return hex.EncodeToString(hasher.Sum(nil))
}
