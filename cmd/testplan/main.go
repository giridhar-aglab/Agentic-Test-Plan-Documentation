// Command testplan runs the test-planning pipeline against a repository.
//
// This slice runs the survey phase against a local directory with a fake
// provider, which is enough to exercise the whole control layer: registry,
// policy, guards, blackboard, orchestrator and checkpointing. The GitHub MCP
// source and the later phases drop in behind the same interfaces.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/agent"
	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/orchestrator"
	"github.com/giri-ms19/testplan-agent/internal/render"
	"github.com/giri-ms19/testplan-agent/internal/tool"
	"github.com/giri-ms19/testplan-agent/internal/tool/code"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "testplan: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	sourceDirectory := flag.String("source", ".", "directory to analyse")
	repositoryName := flag.String("name", "", "repository name for the report (defaults to the directory name)")
	outputPath := flag.String("out", "", "write the report here instead of stdout")
	checkpointDirectory := flag.String("checkpoints", "", "directory for per-phase blackboard checkpoints")
	maxFiles := flag.Int("max-files", 400, "refuse repositories larger than this")
	verbose := flag.Bool("v", false, "log phase progress")
	flag.Parse()

	absoluteSource, err := filepath.Abs(*sourceDirectory)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	displayName := *repositoryName
	if displayName == "" {
		displayName = filepath.Base(absoluteSource)
	}

	// Ctrl-C is a clean cancellation, not a kill: the run stops at the next
	// guard check and still emits whatever it has.
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	runID := fmt.Sprintf("run-%d", time.Now().UTC().Unix())
	blackboard := memory.NewBlackboard(runID)

	source := repo.NewLocalSource(absoluteSource, *maxFiles)

	fullRegistry := tool.NewRegistry()
	fullRegistry.Register(&repo.TreeTool{Source: source}, tool.NetworkPolicy())
	fullRegistry.Register(&repo.ReadFileTool{Source: source, MaxLines: 400}, tool.NetworkPolicy())
	fullRegistry.Register(&code.ParseGoTool{Source: source}, tool.LocalPolicy())

	// Scoped registry: the Surveyor gets exactly these three tools and nothing
	// else. It cannot reach a renderer or a publisher, which is a loop guard as
	// much as a safety measure.
	surveyorRegistry, err := fullRegistry.Scoped("repo.tree", "repo.read_file", "code.parse_go")
	if err != nil {
		return err
	}

	repeatGuard := guard.DefaultRepeatCallGuard()
	surveyorGuards := guard.Chain{
		guard.NewBudgetGuard(guard.Budget{
			MaxIterations: 12, MaxToolCalls: 40, MaxTokens: 60000, MaxWallClock: 3 * time.Minute,
		}),
		repeatGuard,
		guard.NewProgressGuard(3),
		guard.NewScopeGuard(surveyorRegistry),
	}

	// Until an adapter is wired, the fake provider stands in. The survey is
	// deterministic without it; only the narrative sentence is lost.
	fakeProvider := llm.NewFakeProvider()
	fakeProvider.DefaultResponse = &llm.Response{
		Text:       "",
		StopReason: llm.StopEndTurn,
	}

	surveyorLoop := agent.NewLoop(fakeProvider, surveyorRegistry, blackboard, surveyorGuards)
	surveyorLoop.RepeatGuard = repeatGuard

	surveyor := &agent.Surveyor{
		Source: source, RepositoryName: displayName, CommitSHA: currentCommitSHA(absoluteSource),
	}

	pipeline := orchestrator.New(blackboard)
	pipeline.RunBudget = guard.Budget{MaxWallClock: 15 * time.Minute}
	if *verbose {
		pipeline.Logf = func(format string, arguments ...any) {
			fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, arguments...))
		}
	}
	if *checkpointDirectory != "" {
		pipeline.Checkpoint = &fileCheckpointWriter{directory: *checkpointDirectory}
	}

	pipeline.AddPhase(orchestrator.Phase{
		Name: "survey",
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			return surveyor.Survey(ctx, blackboard)
		},
		Budget: guard.Budget{MaxWallClock: 3 * time.Minute},
		// The explicit termination condition: reaching the end of Survey is not
		// the same as having surveyed anything.
		Postcondition: func(blackboard *memory.Blackboard) error {
			repoMap := blackboard.RepoMap()
			if repoMap == nil {
				return errors.New("no repository map was produced")
			}
			if len(repoMap.SelectedFiles()) == 0 {
				return errors.New("no analysable source files were selected")
			}
			return nil
		},
	})

	runReport, err := pipeline.Run(ctx)
	if err != nil {
		return err
	}

	reportMarkdown := render.SurveyMarkdown(blackboard)
	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, []byte(reportMarkdown), 0o644); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "report written to %s\n", *outputPath)
	} else {
		fmt.Print(reportMarkdown)
	}

	for _, phaseResult := range runReport.PhaseResults {
		if !phaseResult.Succeeded() {
			fmt.Fprintf(os.Stderr, "phase %s did not succeed: %v%s\n",
				phaseResult.Name, phaseResult.Err, phaseResult.SkipReason)
		}
	}
	if runReport.Aborted {
		return errors.New(runReport.AbortReason)
	}
	return nil
}

// fileCheckpointWriter persists blackboard snapshots so a crashed run resumes
// from the last completed phase.
type fileCheckpointWriter struct{ directory string }

func (writer *fileCheckpointWriter) Write(runID, phaseName string, snapshot []byte) error {
	runDirectory := filepath.Join(writer.directory, runID)
	if err := os.MkdirAll(runDirectory, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDirectory, phaseName+".json"), snapshot, 0o644)
}

// currentCommitSHA reads .git/HEAD without shelling out to git, so the binary
// has no external command dependency.
func currentCommitSHA(repositoryDirectory string) string {
	headContent, err := os.ReadFile(filepath.Join(repositoryDirectory, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	headText := string(headContent)
	if len(headText) > 5 && headText[:5] == "ref: " {
		referencePath := filepath.Join(repositoryDirectory, ".git",
			filepath.FromSlash(trimNewline(headText[5:])))
		referenceContent, err := os.ReadFile(referencePath)
		if err != nil {
			return ""
		}
		return trimNewline(string(referenceContent))
	}
	return trimNewline(headText)
}

func trimNewline(value string) string {
	for len(value) > 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == '\r') {
		value = value[:len(value)-1]
	}
	return value
}
