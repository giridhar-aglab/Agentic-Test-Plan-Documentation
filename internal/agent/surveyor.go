package agent

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool/code"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// Surveyor builds the RepoMap: what this repository is made of, before anyone
// reasons about behaviour.
//
// Most of its work is deterministic, and deliberately so. Classifying files by
// extension, spotting test files and parsing manifests are things a model
// should never be asked to do: they are cheap, exact, and reproducible in Go,
// and a model would make them expensive, approximate and different on every
// run. The model is used only for the one genuinely interpretive question —
// what this repository is for — and even that is optional.
type Surveyor struct {
	Source         repo.Source
	RepositoryName string
	CommitSHA      string
	// Loop is optional. Without it the survey is entirely deterministic, which
	// is how the fixture tests run.
	Loop *Loop
}

// Name identifies the agent in logs, guards and stops.
func (surveyor *Surveyor) Name() string { return "surveyor" }

// Survey produces the RepoMap and writes it to the blackboard.
func (surveyor *Surveyor) Survey(ctx context.Context, blackboard *memory.Blackboard) error {
	treeEntries, err := surveyor.Source.Tree(ctx)
	if err != nil {
		return fmt.Errorf("surveyor: %w", err)
	}

	repoMap := model.RepoMap{
		RepositoryName: surveyor.RepositoryName,
		CommitSHA:      surveyor.CommitSHA,
		SurveyedAt:     time.Now().UTC(),
	}

	languagesSeen := map[model.Language]bool{}
	for _, treeEntry := range treeEntries {
		sourceFile := classifyFile(treeEntry)
		sourceFile.SelectedForScan = sourceFile.Analysable()
		if sourceFile.Language != model.LanguageUnknown {
			languagesSeen[sourceFile.Language] = true
		}
		repoMap.Files = append(repoMap.Files, sourceFile)
	}
	for language := range languagesSeen {
		repoMap.Languages = append(repoMap.Languages, language)
	}
	sort.Slice(repoMap.Languages, func(leftIndex, rightIndex int) bool {
		return repoMap.Languages[leftIndex] < repoMap.Languages[rightIndex]
	})

	surveyor.collectDependencies(ctx, &repoMap)
	surveyor.collectExistingTests(ctx, &repoMap, blackboard)
	surveyor.collectEntrypoints(ctx, &repoMap)
	repoMap.TestFrameworkName = inferTestFramework(repoMap)

	if surveyor.Loop != nil {
		if err := surveyor.describeRepository(ctx, &repoMap); err != nil {
			// The narrative is a nicety. Losing it costs a sentence in the
			// report, not the survey.
			blackboard.NoteGap(surveyor.Name(), "repository description", err.Error())
		}
	}

	blackboard.SetRepoMap(repoMap)
	return nil
}

// classifyFile applies the deterministic rules: language, test, generated,
// vendored.
func classifyFile(treeEntry repo.Entry) model.SourceFile {
	sourceFile := model.SourceFile{
		Path:      treeEntry.Path,
		BlobSHA:   treeEntry.BlobSHA,
		SizeBytes: treeEntry.SizeBytes,
		Language:  languageForPath(treeEntry.Path),
	}
	baseName := path.Base(treeEntry.Path)

	switch sourceFile.Language {
	case model.LanguageGo:
		sourceFile.IsTest = strings.HasSuffix(baseName, "_test.go")
		sourceFile.IsGenerated = strings.HasSuffix(baseName, ".pb.go") ||
			strings.HasSuffix(baseName, "_generated.go") ||
			strings.Contains(baseName, "mock_")
	case model.LanguagePython:
		sourceFile.IsTest = strings.HasPrefix(baseName, "test_") ||
			strings.HasSuffix(baseName, "_test.py") ||
			strings.Contains(treeEntry.Path, "/tests/")
		sourceFile.IsGenerated = strings.HasSuffix(baseName, "_pb2.py")
	}

	sourceFile.IsVendored = strings.HasPrefix(treeEntry.Path, "vendor/") ||
		strings.Contains(treeEntry.Path, "/vendor/") ||
		strings.Contains(treeEntry.Path, "third_party/")

	if !sourceFile.Analysable() {
		sourceFile.ExcludedReason = exclusionReason(sourceFile)
	}
	return sourceFile
}

func exclusionReason(sourceFile model.SourceFile) string {
	switch {
	case sourceFile.IsTest:
		return "test file"
	case sourceFile.IsGenerated:
		return "generated code"
	case sourceFile.IsVendored:
		return "vendored dependency"
	case sourceFile.Language == model.LanguageUnknown:
		return "unsupported language"
	default:
		return ""
	}
}

func languageForPath(filePath string) model.Language {
	switch strings.ToLower(path.Ext(filePath)) {
	case ".go":
		return model.LanguageGo
	case ".py":
		return model.LanguagePython
	default:
		return model.LanguageUnknown
	}
}

// collectDependencies parses the manifests the repository actually has.
func (surveyor *Surveyor) collectDependencies(ctx context.Context, repoMap *model.RepoMap) {
	for _, sourceFile := range repoMap.Files {
		manifestContent, err := surveyor.readIfManifest(ctx, sourceFile.Path)
		if err != nil || manifestContent == "" {
			continue
		}
		switch path.Base(sourceFile.Path) {
		case "go.mod":
			repoMap.Dependencies = append(repoMap.Dependencies, parseGoMod(manifestContent)...)
			repoMap.BuildSystemNotes = "Go modules"
		case "requirements.txt":
			repoMap.Dependencies = append(repoMap.Dependencies, parseRequirementsTxt(manifestContent)...)
			if repoMap.BuildSystemNotes == "" {
				repoMap.BuildSystemNotes = "pip requirements"
			}
		case "pyproject.toml":
			if repoMap.BuildSystemNotes == "" {
				repoMap.BuildSystemNotes = "pyproject"
			}
		}
	}
}

func (surveyor *Surveyor) readIfManifest(ctx context.Context, filePath string) (string, error) {
	switch path.Base(filePath) {
	case "go.mod", "requirements.txt", "pyproject.toml":
		return surveyor.Source.ReadFile(ctx, filePath)
	default:
		return "", nil
	}
}

func parseGoMod(manifestContent string) []model.Dependency {
	dependencies := []model.Dependency{}
	insideRequireBlock := false
	for _, rawLine := range strings.Split(manifestContent, "\n") {
		trimmedLine := strings.TrimSpace(rawLine)
		switch {
		case trimmedLine == "require (":
			insideRequireBlock = true
			continue
		case insideRequireBlock && trimmedLine == ")":
			insideRequireBlock = false
			continue
		}

		lineFields := strings.Fields(trimmedLine)
		if insideRequireBlock && len(lineFields) >= 2 {
			dependencies = append(dependencies, model.Dependency{
				Name: lineFields[0], Version: lineFields[1],
				Language: model.LanguageGo, Direct: !strings.Contains(trimmedLine, "// indirect"),
			})
			continue
		}
		if len(lineFields) >= 3 && lineFields[0] == "require" {
			dependencies = append(dependencies, model.Dependency{
				Name: lineFields[1], Version: lineFields[2],
				Language: model.LanguageGo, Direct: true,
			})
		}
	}
	return dependencies
}

func parseRequirementsTxt(manifestContent string) []model.Dependency {
	dependencies := []model.Dependency{}
	for _, rawLine := range strings.Split(manifestContent, "\n") {
		trimmedLine := strings.TrimSpace(rawLine)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "-") {
			continue
		}
		packageName, version, _ := strings.Cut(trimmedLine, "==")
		dependencies = append(dependencies, model.Dependency{
			Name: strings.TrimSpace(packageName), Version: strings.TrimSpace(version),
			Language: model.LanguagePython, Direct: true,
		})
	}
	return dependencies
}

// collectExistingTests maps test functions back to the code they cover, so the
// risk register can tell an untested hotspot from a covered one.
func (surveyor *Surveyor) collectExistingTests(ctx context.Context, repoMap *model.RepoMap, blackboard *memory.Blackboard) {
	for _, sourceFile := range repoMap.Files {
		if !sourceFile.IsTest || sourceFile.Language != model.LanguageGo {
			continue
		}
		sourceText, err := surveyor.Source.ReadFile(ctx, sourceFile.Path)
		if err != nil {
			blackboard.NoteGap(surveyor.Name(), sourceFile.Path, "test file unreadable: "+err.Error())
			continue
		}
		analysis, err := code.AnalyseGoSource(sourceFile.Path, sourceText)
		if err != nil {
			blackboard.NoteGap(surveyor.Name(), sourceFile.Path, "test file did not parse: "+err.Error())
			continue
		}
		for _, symbol := range analysis.Symbols {
			if !strings.HasPrefix(symbol.Name, "Test") {
				continue
			}
			repoMap.ExistingTests = append(repoMap.ExistingTests, model.ExistingTest{
				Path:     sourceFile.Path,
				TestName: symbol.Name,
				CoversRef: model.SourceRef{
					Path:   strings.TrimSuffix(sourceFile.Path, "_test.go") + ".go",
					Symbol: strings.TrimPrefix(symbol.Name, "Test"),
				},
				MatchedByAST: true,
			})
		}
	}
}

func (surveyor *Surveyor) collectEntrypoints(ctx context.Context, repoMap *model.RepoMap) {
	for _, sourceFile := range repoMap.Files {
		if !sourceFile.SelectedForScan || sourceFile.Language != model.LanguageGo {
			continue
		}
		sourceText, err := surveyor.Source.ReadFile(ctx, sourceFile.Path)
		if err != nil {
			continue
		}
		analysis, err := code.AnalyseGoSource(sourceFile.Path, sourceText)
		if err != nil {
			continue
		}
		if analysis.PackageName != "main" {
			continue
		}
		for _, symbol := range analysis.Symbols {
			if symbol.Name == "main" && symbol.Kind == "func" {
				repoMap.Entrypoints = append(repoMap.Entrypoints, symbol.Ref)
			}
		}
	}
}

func inferTestFramework(repoMap model.RepoMap) string {
	for _, dependency := range repoMap.Dependencies {
		switch {
		case strings.Contains(dependency.Name, "testify"):
			return "testify"
		case strings.Contains(dependency.Name, "ginkgo"):
			return "ginkgo"
		case dependency.Name == "pytest":
			return "pytest"
		}
	}
	for _, language := range repoMap.Languages {
		if language == model.LanguageGo {
			return "go test"
		}
	}
	return ""
}

// describeRepository asks the model the one question the deterministic pass
// cannot answer.
func (surveyor *Surveyor) describeRepository(ctx context.Context, repoMap *model.RepoMap) error {
	outcome, err := surveyor.Loop.Run(ctx, Task{
		AgentName: surveyor.Name(),
		Tier:      llm.TierFast,
		SystemPrompt: "You are surveying a code repository to prepare a test plan. " +
			"Answer in at most three sentences. State what the system does and where its " +
			"riskiest behaviour is likely to live. Do not speculate beyond the evidence.",
		Instruction: buildSurveyInstruction(*repoMap),
	})
	if err != nil {
		return err
	}
	if outcome.StoppedBy != nil {
		return fmt.Errorf("survey narrative stopped: %s", outcome.StoppedBy.Detail)
	}
	if summaryText := strings.TrimSpace(outcome.FinalText); summaryText != "" {
		repoMap.BuildSystemNotes = strings.TrimSpace(repoMap.BuildSystemNotes + " — " + summaryText)
	}
	return nil
}

func buildSurveyInstruction(repoMap model.RepoMap) string {
	var instructionBuilder strings.Builder
	fmt.Fprintf(&instructionBuilder, "Repository %s at commit %s.\n", repoMap.RepositoryName, repoMap.CommitSHA)
	fmt.Fprintf(&instructionBuilder, "Languages: %v\n", repoMap.Languages)
	fmt.Fprintf(&instructionBuilder, "%d files, %d selected for analysis, %d existing tests.\n",
		len(repoMap.Files), len(repoMap.SelectedFiles()), len(repoMap.ExistingTests))
	instructionBuilder.WriteString("Files selected for analysis:\n")
	for _, sourceFile := range repoMap.SelectedFiles() {
		fmt.Fprintf(&instructionBuilder, "  %s\n", sourceFile.Path)
	}
	instructionBuilder.WriteString("\nUse repo.read_file or code.parse_go if you need detail before answering.")
	return instructionBuilder.String()
}
