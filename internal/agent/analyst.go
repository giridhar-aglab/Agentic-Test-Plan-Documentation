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

Read the file with code_parse_go (or repo_read_file for a language it cannot parse).
Then call analysis_emit_component exactly once and stop.

Ground every claim in code you have actually read. If the file does not show you
something, leave that field empty rather than guessing. Pay particular attention
to error paths: every distinct way the component can fail is a scenario someone
will need to write.

Do not analyse other files. Do not suggest tests. Do not explain yourself in prose.

You have exactly three tools: code_parse_go, repo_read_file and
analysis_emit_component. There is no tool for listing the repository, and you do
not need one — your task names the only file you should open.

One read is enough. If a file comes back truncated, analyse what you were shown
and emit; a partial reading recorded is worth far more than a complete one you
never got to. Emit before you run out of turns.`

// AnalystReadMaxBytes bounds one analyst file read. It must stay below the
// loop's spill threshold; TestTheAnalystsOwnFileReadCanNeverBeSpilled enforces
// that, and enforces it exactly rather than by estimating bytes per line.
const AnalystReadMaxBytes = 40000

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

	// Failure reasons are collected so that a phase which fails on everything
	// can say *why* rather than only that it did. The reason is almost always
	// the same for every component — an unreachable endpoint, a bad key — and
	// making the user open the report to find that out is a poor trade.
	var reasonMutex sync.Mutex
	failureReasons := []string{}

	for workerNumber := 0; workerNumber < concurrency; workerNumber++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for sourceFile := range workQueue {
				if reason := analyst.analyseOne(ctx, blackboard, sourceFile); reason != "" {
					reasonMutex.Lock()
					failureReasons = append(failureReasons, reason)
					reasonMutex.Unlock()
				}
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
		if commonReason := mostCommonReason(failureReasons); commonReason != "" {
			return fmt.Errorf("analyst: no component was analysed successfully. Every attempt failed with: %s",
				commonReason)
		}
		return fmt.Errorf("analyst: no component was analysed successfully")
	}
	return nil
}

// mostCommonReason picks the failure that explains the most components, so the
// message names the actual cause instead of an arbitrary one.
func mostCommonReason(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	countByReason := map[string]int{}
	for _, reason := range reasons {
		countByReason[summariseReason(reason)]++
	}
	bestReason, bestCount := "", 0
	for reason, count := range countByReason {
		if count > bestCount || (count == bestCount && reason < bestReason) {
			bestReason, bestCount = reason, count
		}
	}
	return bestReason
}

// summariseReason strips the per-component prefix so identical causes group
// together rather than looking like distinct failures.
func summariseReason(reason string) string {
	if _, afterCompletion, found := strings.Cut(reason, "completion: "); found {
		return afterCompletion
	}
	return reason
}

// analyseOne returns a failure reason, or "" when the component was recorded.
func (analyst *Analyst) analyseOne(ctx context.Context, blackboard *memory.Blackboard, sourceFile model.SourceFile) string {
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
	componentRegistry := analyst.registryFor(blackboard, baseModel)

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
			"Analyse %s (%s, %d bytes). Read it once, then call analysis_emit_component.",
			sourceFile.Path, sourceFile.Language, sourceFile.SizeBytes),
	})

	if err != nil {
		blackboard.NoteGap(analyst.Name(), sourceFile.Path, "analysis failed: "+err.Error())
		analyst.logf("analyst: %s failed: %v", sourceFile.Path, err)
		return err.Error()
	}
	if outcome.StoppedBy != nil {
		blackboard.NoteGap(analyst.Name(), sourceFile.Path,
			"analysis stopped early ("+string(outcome.StoppedBy.Reason)+"): "+outcome.StoppedBy.Detail)
		analyst.logf("analyst: %s stopped: %s [called: %s]",
			sourceFile.Path, outcome.StoppedBy.Detail, outcome.CallSummary())
	}

	// A loop that ended without emitting is a silent failure unless it is
	// recorded. Checking the blackboard rather than trusting the outcome is the
	// same principle as a phase postcondition, one level down.
	if !analyst.componentWasRecorded(blackboard, sourceFile.Path) {
		// The model never emitted — but the deterministic pass already ran, and
		// its symbols, complexity and error paths are facts regardless of what
		// the model did with its turns. Throwing them away would drop the
		// component out of the risk register entirely, which is a worse answer
		// than an honestly shallow one.
		if baseModel.Depth == model.DepthTypeResolved {
			salvaged := baseModel
			salvaged.Responsibility = ""
			salvaged.Confidence = 0.3
			salvaged.AnalysedAt = time.Now().UTC()
			blackboard.AddComponentModel(salvaged)
			blackboard.NoteGap(analyst.Name(), sourceFile.Path,
				"the model did not record an analysis; structure was taken from the parser alone")
			analyst.logf("analyst: %s fell back to structural analysis only", sourceFile.Path)
			return ""
		}
		const reason = "the analyst finished without recording an analysis " +
			"(the model may not support tool calling)"
		blackboard.NoteGap(analyst.Name(), sourceFile.Path, reason)
		return reason
	}
	return ""
}

// registryFor builds the analyst's tool set. It is deliberately tiny: three
// tools, none of which can reach outside the one file under analysis.
func (analyst *Analyst) registryFor(
	blackboard *memory.Blackboard, baseModel model.ComponentModel,
) *tool.Registry {
	componentRegistry := tool.NewRegistry()
	// 1500 lines covers all but a handful of source files whole. The previous
	// 400 turned every large file into a paging session that ate the iteration
	// budget before the model ever reached the emit call.
	// MaxBytes sits below the loop's spill threshold on purpose: an analyst
	// read that spilled would be digested and then fetched back in pieces,
	// costing several times what sending it once costs. AnalystReadMaxBytes is
	// asserted against that threshold in the tests.
	componentRegistry.Register(&repo.ReadFileTool{
		Source: analyst.Source, MaxLines: 2000, MaxBytes: AnalystReadMaxBytes,
		PathGuidance: "Read only the file named in your task, using exactly that path.",
	}, tool.NetworkPolicy())
	componentRegistry.Register(&code.ParseGoTool{Source: analyst.Source}, tool.LocalPolicy())
	componentRegistry.Register(&emit.ComponentTool{Blackboard: blackboard, Base: baseModel}, tool.LocalPolicy())
	return componentRegistry
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
