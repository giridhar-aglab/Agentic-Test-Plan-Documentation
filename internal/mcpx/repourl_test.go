package mcpx

import "testing"

func TestParseRepositoryReferenceAcceptsEveryCommonForm(t *testing.T) {
	// These are the forms someone actually has in their clipboard. Refusing any
	// of them means they have to retype a URL by hand.
	testCases := []struct {
		name          string
		input         string
		expectedOwner string
		expectedRepo  string
		expectedRef   string
	}{
		{"bare slug", "giri-ms19/testplan-agent", "giri-ms19", "testplan-agent", ""},
		{"browser URL", "https://github.com/giri-ms19/testplan-agent", "giri-ms19", "testplan-agent", ""},
		{"browser URL with trailing slash", "https://github.com/giri-ms19/testplan-agent/", "giri-ms19", "testplan-agent", ""},
		{"clone URL", "https://github.com/giri-ms19/testplan-agent.git", "giri-ms19", "testplan-agent", ""},
		{"ssh remote", "git@github.com:giri-ms19/testplan-agent.git", "giri-ms19", "testplan-agent", ""},
		{"www host", "https://www.github.com/giri-ms19/testplan-agent", "giri-ms19", "testplan-agent", ""},
		{"no scheme", "github.com/giri-ms19/testplan-agent", "giri-ms19", "testplan-agent", ""},
		{"http", "http://github.com/giri-ms19/testplan-agent", "giri-ms19", "testplan-agent", ""},
		{"surrounding whitespace", "  https://github.com/giri-ms19/testplan-agent  ", "giri-ms19", "testplan-agent", ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reference, err := ParseRepositoryReference(testCase.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if reference.Owner != testCase.expectedOwner || reference.Repository != testCase.expectedRepo {
				t.Fatalf("expected %s/%s, got %s", testCase.expectedOwner, testCase.expectedRepo, reference.Slug())
			}
			if reference.Ref != testCase.expectedRef {
				t.Fatalf("expected ref %q, got %q", testCase.expectedRef, reference.Ref)
			}
		})
	}
}

func TestParseRepositoryReferenceKeepsTheBranchFromABrowserURL(t *testing.T) {
	// A link copied from a feature branch must analyse that branch. Silently
	// analysing main instead would produce a plan for code the person is not
	// looking at.
	testCases := []struct {
		input       string
		expectedRef string
	}{
		{"https://github.com/owner/repo/tree/feature-x", "feature-x"},
		{"https://github.com/owner/repo/tree/main/internal/ledger", "main"},
		{"https://github.com/owner/repo/blob/v1.2.0/main.go", "v1.2.0"},
		{"https://github.com/owner/repo/commit/0123456789abcdef", "0123456789abcdef"},
	}
	for _, testCase := range testCases {
		reference, err := ParseRepositoryReference(testCase.input)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", testCase.input, err)
		}
		if reference.Ref != testCase.expectedRef {
			t.Errorf("%s: expected ref %q, got %q", testCase.input, testCase.expectedRef, reference.Ref)
		}
		if reference.Slug() != "owner/repo" {
			t.Errorf("%s: expected owner/repo, got %s", testCase.input, reference.Slug())
		}
	}
}

func TestParseRepositoryReferenceRejectsWhatItCannotHandle(t *testing.T) {
	for _, badInput := range []string{
		"", "   ",
		"just-a-name",
		"https://gitlab.com/owner/repo",
		"https://bitbucket.org/owner/repo",
		"owner/",
		"/repo",
	} {
		if reference, err := ParseRepositoryReference(badInput); err == nil {
			t.Errorf("%q should have been rejected, got %+v", badInput, reference)
		}
	}
}

func TestNonGitHubHostIsNamedInTheError(t *testing.T) {
	// A confusing parse failure wastes someone's time; naming the actual
	// limitation does not.
	_, err := ParseRepositoryReference("https://gitlab.com/owner/repo")
	if err == nil {
		t.Fatal("expected a rejection")
	}
	if !contains(err.Error(), "github.com") {
		t.Fatalf("the error should say only GitHub is supported, got %v", err)
	}
}

func TestSourceLinkBaseIsDerivedFromTheReference(t *testing.T) {
	// Deriving this means the common case needs no extra flag for source links
	// to work.
	reference, err := ParseRepositoryReference("https://github.com/owner/repo/tree/develop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := reference.SourceLinkBase(); got != "https://github.com/owner/repo/blob/develop" {
		t.Fatalf("unexpected link base %q", got)
	}

	withoutRef, _ := ParseRepositoryReference("owner/repo")
	if got := withoutRef.SourceLinkBase(); got != "https://github.com/owner/repo/blob/HEAD" {
		t.Fatalf("a reference without a branch should fall back to HEAD, got %q", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for startIndex := 0; startIndex+len(needle) <= len(haystack); startIndex++ {
		if haystack[startIndex:startIndex+len(needle)] == needle {
			return startIndex
		}
	}
	return -1
}
