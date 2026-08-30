package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool"
	"github.com/giri-ms19/testplan-agent/internal/tool/code"
	"github.com/giri-ms19/testplan-agent/internal/tool/emit"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// AnalystSystemPrompt is deliberately narrow. The Analyst reads one component
// and records what it found; it does not design tests, prioritise, or comment
// on anything outside the file it was given. Keeping each specialist's remit
// small is what keeps their contexts small.
const AnalystSystemPrompt = `You are analysing a single source file to prepare a test plan.

Read the file with code.parse_go (or repo.read_file for a language it cannot parse).
Then call analysis.emit_component exactly once and stop.

Ground every claim in code you have actually read. If the file does not show you
something, leave that field empty rather than guessing. Pay particular attention
to error paths: every distinct way the component can fail is a scenario someone
will need to write.

Do not analyse other files. Do not suggest tests. Do not explain yourself in prose.`

// Analyst produces one ComponentModel per selected file.
//
// The fan-out is bounded and each worker gets a fresh context holding only its
// own task. That is the practical payoff of passing structs instead of
// transcripts: N components analyse in parallel without N contexts growing into
// each other.
type Analyst struct {
	Source      repo.Source
	Provider    llm.Provider
	Blackboard  *memory.Blackboard
	Concurrency int
	// PerComponentBudget bounds one worker. A component that will not analyse
	// costs one budget, not the phase.
	PerComponentBudget guard.Budget
	Logf               func(format string, arguments ...any)
}

func (analyst *Analyst) Name() string { return "analyst" }

func (analyst *Analyst) logf(format string, arguments ...any) {
	if analyst.Logf != nil {
		analyst.Logf(format, arguments...)
	}
}

// Analyse fans out across the selected files.
func (analyst *Analyst) Analyse(ctx context.Context, blackboard *memory.Blackboard) error {
	repoMap := blackboard.RepoMap()
	if repoMap == nil {
		return fmt.Errorf("analyst: no repository map; the survey phase must run first")
	}
	selectedFiles := repoMap.SelectedFiles()
	if len(selectedFiles) == 0 {
		return fmt.Errorf("analyst: no files were selected for analysis")
	}

	concurrency := analyst.Concurrency
	if concurrency < 1 {
		concurrency = 4
	}

	workQueue := make(chan model.SourceFile)
	var waitGroup sync.WaitGroup

	for workerNumber := 0; workerNumber < concurrency; workerNumber++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for sourceFile := range workQueue {
				analyst.analyseOne(ctx, blackboard, sourceFile)
			}
		}()
	}

dispatch:
	for _, sourceFile := range selectedFiles {
		select {
		case <-ctx.Done():
			break dispatch
		case workQueue <- sourceFile:
		}
	}
	close(workQueue)
	waitGroup.Wait()

	// Partial analysis is a legitimate outcome. The phase fails only if nothing
	// at all was learned, because a plan covering most of a repository with an
	// honest gaps section is worth more than no plan.
	if len(blackboard.ComponentModels()) == 0 {
		return fmt.Errorf("analyst: no component was analysed successfully")
	}
	return nil
}

func (analyst *Analyst) analyseOne(ctx context.Context, blackboard *memory.Blackboard, sourceFile model.SourceFile) {
	componentRegistry := tool.NewRegistry()
	componentRegistry.Register(&repo.ReadFileTool{Source: analyst.Source, MaxLines: 400}, tool.NetworkPolicy())
	componentRegistry.Register(&code.ParseGoTool{Source: analyst.Source}, tool.LocalPolicy())
	// The deterministic pass runs first and becomes the base the model enriches.
	// Symbols, complexity and third-party imports are facts, not judgements.
	baseModel := model.ComponentModel{
		ComponentName: componentNameForPath(sourceFile.Path),
		Path:          sourceFile.Path,
		BlobSHA:       sourceFile.BlobSHA,
		Language:      sourceFile.Language,
		Depth:         model.DepthSyntactic,
	}
	if sourceFile.Language == model.LanguageGo {
		if sourceText, err := analyst.Source.ReadFile(ctx, sourceFile.Path); err == nil {
			if astAnalysis, err := code.AnalyseGoSource(sourceFile.Path, sourceText); err == nil {
				baseModel = componentModelFromAST(sourceFile, astAnalysis)
				baseModel.Depth = model.DepthTypeResolved
			}
		}
	}
	componentRegistry.Register(&emit.ComponentTool{Blackboard: blackboard, Base: baseModel}, tool.LocalPolicy())

	repeatGuard := guard.DefaultRepeatCallGuard()
	componentGuards := guard.Chain{
		guard.NewBudgetGuard(analyst.PerComponentBudget),
		repeatGuard,
		guard.NewProgressGuard(3),
		guard.NewScopeGuard(componentRegistry),
	}

	componentLoop := NewLoop(analyst.Provider, componentRegistry, blackboard, componentGuards)
	componentLoop.RepeatGuard = repeatGuard

	outcome, err := componentLoop.Run(ctx, Task{
		AgentName:    "analyst:" + sourceFile.Path,
		Tier:         llm.TierBalanced,
		SystemPrompt: AnalystSystemPrompt,
		Instruction: fmt.Sprintf(
			"Analyse %s (%s, %d bytes). Read it, then call analysis.emit_component.",
			sourceFile.Path, sourceFile.Language, sourceFile.SizeBytes),
	})

	if err != nil {
		blackboard.NoteGap(analyst.Name(), sourceFile.Path, "analysis failed: "+err.Error())
		analyst.logf("analyst: %s failed: %v", sourceFile.Path, err)
		return
	}
	if outcome.StoppedBy != nil {
		blackboard.NoteGap(analyst.Name(), sourceFile.Path,
			"analysis stopped early ("+string(outcome.StoppedBy.Reason)+"): "+outcome.StoppedBy.Detail)
		analyst.logf("analyst: %s stopped: %s", sourceFile.Path, outcome.StoppedBy.Detail)
	}

	// A loop that ended without emitting is a silent failure unless it is
	// recorded. Checking the blackboard rather than trusting the outcome is the
	// same principle as a phase postcondition, one level down.
	if !analyst.componentWasRecorded(blackboard, sourceFile.Path) {
		blackboard.NoteGap(analyst.Name(), sourceFile.Path,
			"the analyst finished without recording an analysis")
	}
}

func (analyst *Analyst) componentWasRecorded(blackboard *memory.Blackboard, filePath string) bool {
	for _, componentModel := range blackboard.ComponentModels() {
		if componentModel.Path == filePath {
			return true
		}
	}
	return false
}

// AnalystFallback fills in a component model without a provider, using only the
// AST. It is what the phase falls back to when the model layer is unavailable,
// and it is what the fixture tests run against.
//
// The result is honestly marked DepthSyntactic and low confidence: a structural
// summary is not the same thing as understanding, and the report says so.
func AnalystFallback(ctx context.Context, source repo.Source, blackboard *memory.Blackboard) error {
	repoMap := blackboard.RepoMap()
	if repoMap == nil {
		return fmt.Errorf("analyst: no repository map")
	}
	for _, sourceFile := range repoMap.SelectedFiles() {
		if sourceFile.Language != model.LanguageGo {
			blackboard.NoteGap("analyst", sourceFile.Path, "no analyser for this language without a model")
			continue
		}
		sourceText, err := source.ReadFile(ctx, sourceFile.Path)
		if err != nil {
			blackboard.NoteGap("analyst", sourceFile.Path, "unreadable: "+err.Error())
			continue
		}
		analysis, err := code.AnalyseGoSource(sourceFile.Path, sourceText)
		if err != nil {
			blackboard.NoteGap("analyst", sourceFile.Path, "did not parse: "+err.Error())
			continue
		}
		blackboard.AddComponentModel(componentModelFromAST(sourceFile, analysis))
	}
	if len(blackboard.ComponentModels()) == 0 {
		return fmt.Errorf("analyst: no component was analysed successfully")
	}
	return nil
}

func componentModelFromAST(sourceFile model.SourceFile, analysis code.FileAnalysis) model.ComponentModel {
	publicSymbols := []model.Symbol{}
	errorPaths := []string{}
	for _, symbol := range analysis.Symbols {
		if symbol.Exported {
			publicSymbols = append(publicSymbols, symbol)
		}
		if symbol.ReturnsError {
			errorPaths = append(errorPaths, symbol.Name+" returns an error")
		}
	}

	externalCalls := []model.Dependency{}
	for _, importPath := range analysis.Imports {
		if !strings.Contains(importPath, ".") {
			continue // standard library
		}
		externalCalls = append(externalCalls, model.Dependency{
			Name: importPath, Language: sourceFile.Language,
			RequiresMock: true, Reason: "third-party import",
		})
	}

	return model.ComponentModel{
		ComponentName:   componentNameForPath(sourceFile.Path),
		Path:            sourceFile.Path,
		BlobSHA:         sourceFile.BlobSHA,
		Language:        sourceFile.Language,
		Depth:           model.DepthSyntactic,
		Responsibility:  fmt.Sprintf("Package %s: %d exported symbols.", analysis.PackageName, len(publicSymbols)),
		PublicSymbols:   publicSymbols,
		ExternalCalls:   externalCalls,
		ErrorPaths:      errorPaths,
		ComplexityScore: analysis.ComplexityScore,
		Confidence:      0.5,
		AnalysedAt:      time.Now().UTC(),
	}
}

func componentNameForPath(filePath string) string {
	trimmedPath := strings.TrimSuffix(strings.TrimSuffix(filePath, ".go"), ".py")
	pathSegments := strings.Split(trimmedPath, "/")
	if len(pathSegments) >= 2 {
		return strings.Join(pathSegments[len(pathSegments)-2:], "/")
	}
	return trimmedPath
}
