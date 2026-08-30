package mcpx

import (
	"fmt"
	"strings"
)

// RepositoryReference is a parsed pointer to a GitHub repository.
type RepositoryReference struct {
	Owner      string
	Repository string
	// Ref is a branch, tag or commit when the input carried one. Empty means
	// the server's default branch.
	Ref string
}

// Slug renders the canonical owner/repo form.
func (reference RepositoryReference) Slug() string {
	return reference.Owner + "/" + reference.Repository
}

// SourceLinkBase builds the blob URL that scenario source links hang off, so a
// reviewer can click through to the exact lines. Deriving it from the reference
// means the common case needs no extra flag.
func (reference RepositoryReference) SourceLinkBase() string {
	ref := reference.Ref
	if ref == "" {
		ref = "HEAD"
	}
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s", reference.Owner, reference.Repository, ref)
}

// ParseRepositoryReference accepts the forms a person actually has to hand:
// a browser URL, a clone URL, an SSH remote, or a bare owner/repo slug.
//
// Requiring one canonical form would be a small piece of rudeness with a real
// cost — the thing someone has in their clipboard is almost always the browser
// URL, and refusing it means they have to retype it.
func ParseRepositoryReference(input string) (RepositoryReference, error) {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return RepositoryReference{}, fmt.Errorf("no repository was given")
	}

	remainder := trimmedInput

	// git@github.com:owner/repo.git
	if afterSSH, isSSH := strings.CutPrefix(remainder, "git@"); isSSH {
		_, afterHost, found := strings.Cut(afterSSH, ":")
		if !found {
			return RepositoryReference{}, fmt.Errorf("could not parse the SSH remote %q", input)
		}
		remainder = afterHost
	} else {
		// Strip a scheme and any host, so both browser and clone URLs work.
		for _, schemePrefix := range []string{"https://", "http://", "ssh://", "git://"} {
			if afterScheme, hadScheme := strings.CutPrefix(remainder, schemePrefix); hadScheme {
				remainder = afterScheme
				break
			}
		}
		if afterHost, hadHost := strings.CutPrefix(remainder, "www.github.com/"); hadHost {
			remainder = afterHost
		} else if afterHost, hadHost := strings.CutPrefix(remainder, "github.com/"); hadHost {
			remainder = afterHost
		} else if strings.Contains(remainder, "://") || looksLikeAnotherHost(remainder) {
			return RepositoryReference{}, fmt.Errorf(
				"only github.com repositories are supported, got %q", input)
		}
	}

	remainder = strings.TrimSuffix(strings.Trim(remainder, "/"), ".git")

	pathSegments := strings.Split(remainder, "/")
	if len(pathSegments) < 2 || pathSegments[0] == "" || pathSegments[1] == "" {
		return RepositoryReference{}, fmt.Errorf(
			"expected owner/repo or a github.com URL, got %q", input)
	}

	reference := RepositoryReference{
		Owner:      pathSegments[0],
		Repository: strings.TrimSuffix(pathSegments[1], ".git"),
	}

	// A browser URL often already names the branch the person is looking at:
	//   /tree/<ref>/…   /blob/<ref>/…   /commit/<sha>
	// Honouring it means a link copied from a feature branch analyses that
	// branch rather than silently analysing main.
	if len(pathSegments) >= 4 {
		switch pathSegments[2] {
		case "tree", "blob", "commit":
			reference.Ref = pathSegments[3]
		}
	}
	return reference, nil
}

// looksLikeAnotherHost spots a non-GitHub host so the error can say so plainly
// rather than failing later with a confusing parse.
func looksLikeAnotherHost(remainder string) bool {
	firstSegment, _, found := strings.Cut(remainder, "/")
	if !found {
		return false
	}
	return strings.Contains(firstSegment, ".")
}
