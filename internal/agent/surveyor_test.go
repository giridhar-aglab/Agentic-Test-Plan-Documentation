package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/render"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

const fixtureDirectory = "../../testdata/fixtures/paymentsvc"

func surveyFixture(t *testing.T) (*memory.Blackboard, model.RepoMap) {
	t.Helper()
	blackboard := memory.NewBlackboard("fixture-run")
	surveyor := &Surveyor{
		Source:         repo.NewLocalSource(fixtureDirectory, 400),
		RepositoryName: "paymentsvc",
		CommitSHA:      "0123456789abcdef",
	}
	if err := surveyor.Survey(context.Background(), blackboard); err != nil {
		t.Fatalf("survey failed: %v", err)
	}
	repoMap := blackboard.RepoMap()
	if repoMap == nil {
		t.Fatal("survey produced no repository map")
	}
	return blackboard, *repoMap
}

func TestSurveyorClassifiesFixtureRepository(t *testing.T) {
	_, repoMap := surveyFixture(t)

	selectedPaths := map[string]bool{}
	for _, sourceFile := range repoMap.SelectedFiles() {
		selectedPaths[sourceFile.Path] = true
	}
	for _, expectedPath := range []string{
		"cmd/paysvc/main.go", "internal/ledger/ledger.go", "internal/gateway/gateway.go",
	} {
		if !selectedPaths[expectedPath] {
			t.Errorf("expected %s to be selected for analysis", expectedPath)
		}
	}
	if selectedPaths["internal/ledger/ledger_test.go"] {
		t.Error("a test file must not be selected as a subject of analysis")
	}
	if len(repoMap.Languages) != 1 || repoMap.Languages[0] != model.LanguageGo {
		t.Errorf("expected Go only, got %v", repoMap.Languages)
	}
}

func TestSurveyorUsesGitBlobSHAsAsCacheKeys(t *testing.T) {
	// Blob SHAs are the analysis cache key, so they must be git's own hash and
	// must be stable across surveys of unchanged content.
	_, firstSurvey := surveyFixture(t)
	_, secondSurvey := surveyFixture(t)

	shaByPath := map[string]string{}
	for _, sourceFile := range firstSurvey.Files {
		if len(sourceFile.BlobSHA) != 40 {
			t.Fatalf("%s: expected a 40-character git blob SHA, got %q", sourceFile.Path, sourceFile.BlobSHA)
		}
		shaByPath[sourceFile.Path] = sourceFile.BlobSHA
	}
	for _, sourceFile := range secondSurvey.Files {
		if shaByPath[sourceFile.Path] != sourceFile.BlobSHA {
			t.Fatalf("%s: blob SHA changed between surveys of unchanged content", sourceFile.Path)
		}
	}
}

func TestGitBlobSHAMatchesGitObjectFormat(t *testing.T) {
	// Verified against `printf '' | git hash-object --stdin`.
	if got := repo.GitBlobSHA([]byte("")); got != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Fatalf("empty blob hash does not match git's, got %s", got)
	}
	// Verified against `printf 'hello\n' | git hash-object --stdin`.
	if got := repo.GitBlobSHA([]byte("hello\n")); got != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Fatalf("blob hash does not match git's, got %s", got)
	}
}

func TestSurveyorFindsDependenciesAndExistingTests(t *testing.T) {
	_, repoMap := surveyFixture(t)

	foundIndirect := false
	for _, dependency := range repoMap.Dependencies {
		if dependency.Name == "github.com/shopspring/decimal" {
			foundIndirect = true
			if dependency.Direct {
				t.Error("an // indirect dependency must not be reported as direct")
			}
		}
	}
	if !foundIndirect {
		t.Error("expected the indirect dependency to be parsed from go.mod")
	}
	if repoMap.TestFrameworkName != "testify" {
		t.Errorf("expected testify to be inferred, got %q", repoMap.TestFrameworkName)
	}
	if len(repoMap.ExistingTests) != 1 || repoMap.ExistingTests[0].TestName != "TestBalance" {
		t.Errorf("expected the existing test to be found, got %v", repoMap.ExistingTests)
	}
	if len(repoMap.Entrypoints) != 1 || repoMap.Entrypoints[0].Symbol != "main" {
		t.Errorf("expected one main entrypoint, got %v", repoMap.Entrypoints)
	}
}

func TestSurveyorRefusesRepositoryAboveCeiling(t *testing.T) {
	// The stated ceiling refuses loudly rather than truncating silently, which
	// would produce a plan that looks complete and is not.
	blackboard := memory.NewBlackboard("over-ceiling")
	surveyor := &Surveyor{
		Source:         repo.NewLocalSource(fixtureDirectory, 2),
		RepositoryName: "paymentsvc",
	}
	err := surveyor.Survey(context.Background(), blackboard)
	if err == nil {
		t.Fatal("expected a repository above the ceiling to be refused")
	}
	if !strings.Contains(err.Error(), "ceiling") {
		t.Fatalf("the refusal must explain itself, got %v", err)
	}
}

func TestRenderedReportIsDeterministicAndNamesGaps(t *testing.T) {
	blackboard, _ := surveyFixture(t)
	blackboard.NoteGap("analyse", "internal/gateway/gateway.go", "model unavailable")

	firstRender := render.SurveyMarkdown(blackboard)
	secondRender := render.SurveyMarkdown(blackboard)
	if firstRender != secondRender {
		t.Fatal("the same blackboard must always render the same report, or runs cannot be diffed")
	}
	for _, expectedFragment := range []string{
		"# Test Plan — paymentsvc",
		"internal/ledger/ledger.go",
		"## Gaps",
		"model unavailable",
	} {
		if !strings.Contains(firstRender, expectedFragment) {
			t.Errorf("report is missing %q", expectedFragment)
		}
	}
}
