// Command testplan runs the test-planning pipeline against a repository.
//
// The source is either a local directory or a GitHub repository reached through
// an MCP server. Both satisfy repo.Source, so nothing downstream changes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/approval"
	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/llm/anthropic"
	"github.com/giri-ms19/testplan-agent/internal/mcpx"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/pipeline"
	"github.com/giri-ms19/testplan-agent/internal/render"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "testplan: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	sourceDirectory := flag.String("source", ".", "local directory to analyse")
	githubRepository := flag.String("github", "",
		"analyse a GitHub repository: a full URL, an SSH remote, or owner/repo")
	githubRef := flag.String("ref", "",
		"branch, tag or commit for -github (overrides a branch found in the URL; default: the default branch)")
	mcpCommand := flag.String("mcp-command", defaultMCPCommand(),
		"command that starts the GitHub MCP server (or set GITHUB_MCP_COMMAND)")
	repositoryName := flag.String("name", "", "repository name for the report")
	outputPath := flag.String("out", "", "write the report here instead of stdout")
	checkpointDirectory := flag.String("checkpoints", "", "directory for per-phase blackboard checkpoints")
	maxFiles := flag.Int("max-files", 400, "refuse repositories larger than this")
	concurrency := flag.Int("concurrency", 4, "analyst fan-out width")
	revisionRounds := flag.Int("revision-rounds", 2, "maximum author/critic rounds")
	sourceLinkBase := flag.String("link-base", "", "base URL for source links, e.g. https://github.com/owner/repo/blob/main")
	mcpCheck := flag.Bool("mcp-check", false,
		"connect to the GitHub MCP server, report what it offers, and exit without running the pipeline")
	mcpToolTree := flag.String("mcp-tool-tree", "", "override the remote tool used to list the repository tree")
	mcpToolFile := flag.String("mcp-tool-file", "", "override the remote tool used to read a file")
	verbose := flag.Bool("v", false, "log phase progress")
	flag.Parse()

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	logf := func(format string, arguments ...any) {}
	if *verbose {
		logf = func(format string, arguments ...any) {
			fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, arguments...))
		}
	}

	sourceOptions := sourceOptions{
		LocalDirectory: *sourceDirectory,
		GitHubTarget:   *githubRepository,
		Ref:            *githubRef,
		MCPCommand:     *mcpCommand,
		MaxFiles:       *maxFiles,
		DisplayName:    *repositoryName,
		ToolOverrides:  mcpx.GitHubToolNames{Tree: *mcpToolTree, GetFile: *mcpToolFile},
		CheckOnly:      *mcpCheck,
		Logf:           logf,
	}
	if *mcpCheck {
		return checkMCPServer(ctx, sourceOptions)
	}

	source, displayName, commitSHA, closeSource, err := buildSource(ctx, sourceOptions)
	if err != nil {
		return err
	}
	defer closeSource()

	// Source links make a scenario checkable in one click. For a GitHub run the
	// base is derivable, so not asking for it is one less flag to get wrong.
	resolvedLinkBase := *sourceLinkBase
	if resolvedLinkBase == "" && *githubRepository != "" {
		if reference, parseErr := mcpx.ParseRepositoryReference(*githubRepository); parseErr == nil {
			if *githubRef != "" {
				reference.Ref = *githubRef
			}
			resolvedLinkBase = reference.SourceLinkBase()
		}
	}

	provider := buildProvider(logf)

	runID := fmt.Sprintf("run-%d", time.Now().UTC().Unix())
	blackboard := memory.NewBlackboard(runID)

	configuration := pipeline.Config{
		Source: source, Provider: provider,
		RepositoryName: displayName, CommitSHA: commitSHA,
		AnalystConcurrency: *concurrency,
		MaxRevisionRounds:  *revisionRounds,
		RunBudget:          guard.Budget{MaxWallClock: 45 * time.Minute},
		Logf:               logf,
	}
	if *checkpointDirectory != "" {
		configuration.Checkpoint = &fileCheckpointWriter{directory: *checkpointDirectory}
	}

	runReport, err := pipeline.Build(configuration, blackboard).Run(ctx)
	if err != nil {
		return err
	}

	validationReport := approval.Validate(blackboard)
	reportMarkdown := render.PlanMarkdown(blackboard, render.Options{
		SourceLinkBase: resolvedLinkBase,
		Validation:     &validationReport,
	})

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, []byte(reportMarkdown), 0o644); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "report written to %s\n", *outputPath)
	} else {
		fmt.Print(reportMarkdown)
	}

	for _, phaseResult := range runReport.PhaseResults {
		if phaseResult.Succeeded() {
			continue
		}
		if phaseResult.Skipped {
			fmt.Fprintf(os.Stderr, "phase %s skipped: %s\n", phaseResult.Name, phaseResult.SkipReason)
			continue
		}
		fmt.Fprintf(os.Stderr, "phase %s did not succeed: %v\n", phaseResult.Name, phaseResult.Err)
	}
	if runReport.Aborted {
		return errors.New(runReport.AbortReason)
	}
	return nil
}

// sourceOptions carries everything the source constructors need, so adding a
// knob does not mean threading another positional argument through.
type sourceOptions struct {
	LocalDirectory string
	GitHubTarget   string
	Ref            string
	MCPCommand     string
	MaxFiles       int
	DisplayName    string
	ToolOverrides  mcpx.GitHubToolNames
	CheckOnly      bool
	Logf           func(format string, arguments ...any)
}

// buildSource picks between a local directory and GitHub over MCP. Both satisfy
// repo.Source, so this function is the only place that knows the difference.
func buildSource(ctx context.Context, options sourceOptions) (repo.Source, string, string, func(), error) {
	noop := func() {}

	if options.GitHubTarget == "" {
		absoluteSource, err := filepath.Abs(options.LocalDirectory)
		if err != nil {
			return nil, "", "", noop, fmt.Errorf("resolve source: %w", err)
		}
		displayName := options.DisplayName
		if displayName == "" {
			displayName = filepath.Base(absoluteSource)
		}
		return repo.NewLocalSource(absoluteSource, options.MaxFiles), displayName,
			currentCommitSHA(absoluteSource), noop, nil
	}

	reference, client, closeClient, err := connectToGitHub(ctx, options)
	if err != nil {
		return nil, "", "", noop, err
	}

	// Ask the server what it actually offers rather than assuming a naming
	// convention. Explicit overrides still win.
	resolvedToolNames, resolveErr := mcpx.ResolveToolNames(ctx, client)
	if options.ToolOverrides.Tree != "" {
		resolvedToolNames.Tree = options.ToolOverrides.Tree
	}
	if options.ToolOverrides.GetFile != "" {
		resolvedToolNames.GetFile = options.ToolOverrides.GetFile
	}
	if resolvedToolNames.Tree == "" || resolvedToolNames.GetFile == "" {
		closeClient()
		return nil, "", "", noop, resolveErr
	}
	options.Logf("mcp: using %s", resolvedToolNames.Describe())

	displayName := options.DisplayName
	if displayName == "" {
		displayName = reference.Slug()
	}
	githubSource := mcpx.NewGitHubSource(client, reference.Owner, reference.Repository,
		reference.Ref, options.MaxFiles)
	githubSource.ToolNames = resolvedToolNames

	return githubSource, displayName, reference.Ref, closeClient, nil
}

// connectToGitHub starts the server subprocess and negotiates the protocol.
func connectToGitHub(ctx context.Context, options sourceOptions) (
	mcpx.RepositoryReference, *mcpx.Client, func(), error,
) {
	noop := func() {}

	reference, err := mcpx.ParseRepositoryReference(options.GitHubTarget)
	if err != nil {
		return reference, nil, noop, fmt.Errorf("-github: %w", err)
	}
	// An explicit -ref wins over a branch carried in the URL, so a pasted link
	// can still be redirected without editing it.
	if options.Ref != "" {
		reference.Ref = options.Ref
	}
	if options.MCPCommand == "" {
		return reference, nil, noop, errors.New(
			"-github needs a GitHub MCP server to talk to. Set -mcp-command or GITHUB_MCP_COMMAND, " +
				"for example: -mcp-command \"npx -y @modelcontextprotocol/server-github\". " +
				"The server reads GITHUB_TOKEN for private repositories")
	}

	commandFields := strings.Fields(options.MCPCommand)
	transport, err := mcpx.StartCommand(ctx, commandFields[0], commandFields[1:], nil)
	if err != nil {
		return reference, nil, noop, fmt.Errorf("could not start the MCP server %q: %w", options.MCPCommand, err)
	}
	client := mcpx.NewClient("github", transport)
	closeClient := func() { _ = client.Close() }

	// Negotiate before doing anything else: under the stateless revision this
	// is the only way to learn whether the server can talk to us at all.
	if err := client.Discover(ctx); err != nil {
		closeClient()
		return reference, nil, noop, err
	}
	return reference, client, closeClient, nil
}

// checkMCPServer is the pre-flight diagnostic: connect, negotiate, list the
// catalogue, and report which tools would be used. Rehearsing an integration
// should not require running a whole pipeline.
func checkMCPServer(ctx context.Context, options sourceOptions) error {
	if options.GitHubTarget == "" {
		return errors.New("-mcp-check needs -github naming the repository to check against")
	}

	reference, client, closeClient, err := connectToGitHub(ctx, options)
	if err != nil {
		return err
	}
	defer closeClient()

	fmt.Printf("connected     server responded to server/discover on protocol %s\n", mcpx.ProtocolVersion)
	fmt.Printf("repository    %s", reference.Slug())
	if reference.Ref != "" {
		fmt.Printf(" @ %s", reference.Ref)
	}
	fmt.Println()

	descriptors, err := client.ListTools(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("catalogue     %d tools\n", len(descriptors))
	for _, descriptor := range descriptors {
		fmt.Printf("              - %s\n", descriptor.Name)
	}

	resolvedToolNames, resolveErr := mcpx.ResolveToolNames(ctx, client)
	if options.ToolOverrides.Tree != "" {
		resolvedToolNames.Tree = options.ToolOverrides.Tree
	}
	if options.ToolOverrides.GetFile != "" {
		resolvedToolNames.GetFile = options.ToolOverrides.GetFile
	}
	if resolvedToolNames.Tree == "" || resolvedToolNames.GetFile == "" {
		return resolveErr
	}
	fmt.Printf("resolved      %s\n", resolvedToolNames.Describe())

	// A live fetch is the only thing that proves credentials and arguments are
	// right. Everything above can pass against a server that cannot see the repo.
	githubSource := mcpx.NewGitHubSource(client, reference.Owner, reference.Repository,
		reference.Ref, options.MaxFiles)
	githubSource.ToolNames = resolvedToolNames

	entries, err := githubSource.Tree(ctx)
	if err != nil {
		return fmt.Errorf("the tree fetch failed, so the pipeline would fail too: %w", err)
	}
	fmt.Printf("tree          %d files\n", len(entries))
	if len(entries) > 0 {
		content, err := githubSource.ReadFile(ctx, entries[0].Path)
		if err != nil {
			return fmt.Errorf("reading %s failed, so the pipeline would fail too: %w", entries[0].Path, err)
		}
		fmt.Printf("read          %s (%d bytes)\n", entries[0].Path, len(content))
	}
	fmt.Println("\nok — this server can drive the pipeline.")
	return nil
}

// defaultMCPCommand lets the server be configured once in the environment
// rather than repeated on every invocation.
func defaultMCPCommand() string { return os.Getenv("GITHUB_MCP_COMMAND") }

// buildProvider returns nil when no credentials are configured. A nil provider
// is a supported mode, not an error: the pipeline runs every phase with its
// deterministic fallback and says so in the report.
func buildProvider(logf func(format string, arguments ...any)) llm.Provider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		logf("no ANTHROPIC_API_KEY set; running with deterministic fallbacks only")
		return nil
	}
	provider := anthropic.New(apiKey)
	if endpointOverride := os.Getenv("ANTHROPIC_BASE_URL"); endpointOverride != "" {
		provider.Endpoint = strings.TrimSuffix(endpointOverride, "/") + "/v1/messages"
	}
	return provider
}

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
	headText := strings.TrimSpace(string(headContent))
	if reference, isSymbolic := strings.CutPrefix(headText, "ref: "); isSymbolic {
		referenceContent, err := os.ReadFile(
			filepath.Join(repositoryDirectory, ".git", filepath.FromSlash(reference)))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(referenceContent))
	}
	return headText
}
