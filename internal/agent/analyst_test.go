package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

func TestMostCommonReasonGroupsIdenticalCauses(t *testing.T) {
	// Every component usually fails for the same reason — an unreachable
	// endpoint, a bad key. The message must name that cause rather than an
	// arbitrary one, and the per-component prefix must not split it into
	// several apparently distinct failures.
	reasons := []string{
		`agent analyst:a.go: completion: openai-compatible: request failed: dial tcp: connection refused`,
		`agent analyst:b.go: completion: openai-compatible: request failed: dial tcp: connection refused`,
		`agent analyst:c.go: completion: openai-compatible: request failed: dial tcp: connection refused`,
	}
	got := mostCommonReason(reasons)
	if got != `openai-compatible: request failed: dial tcp: connection refused` {
		t.Fatalf("the shared cause should survive without its per-component prefix, got %q", got)
	}
}

func TestMostCommonReasonPicksTheMajority(t *testing.T) {
	reasons := []string{
		"completion: rate limited",
		"completion: rate limited",
		"completion: something else entirely",
	}
	if got := mostCommonReason(reasons); got != "rate limited" {
		t.Fatalf("expected the majority cause, got %q", got)
	}
}

func TestMostCommonReasonIsEmptyWhenNothingFailed(t *testing.T) {
	if got := mostCommonReason(nil); got != "" {
		t.Fatalf("expected no reason, got %q", got)
	}
}

// pagingProvider imitates a small model that treats every hint as an
// instruction: it reads, and keeps reading, and never emits.
type pagingProvider struct{ readCount int }

func (provider *pagingProvider) Name() string { return "paging" }
func (provider *pagingProvider) Limits(llm.Tier) llm.Limits {
	return llm.Limits{ContextWindowTokens: 100000, MaxOutputTokens: 2000}
}
func (provider *pagingProvider) CountTokens(context.Context, llm.Request) (int, error) { return 0, nil }
func (provider *pagingProvider) Complete(_ context.Context, request llm.Request) (*llm.Response, error) {
	provider.readCount++
	startLine := 1 + provider.readCount*100
	return &llm.Response{
		StopReason: llm.StopToolUse,
		ToolCalls: []llm.ToolCall{{
			ID:       fmt.Sprintf("c%d", provider.readCount),
			ToolName: "repo_read_file",
			Arguments: json.RawMessage(fmt.Sprintf(
				`{"path":"internal/ledger/ledger.go","startLine":%d}`, startLine)),
		}},
		Usage: llm.Usage{InputTokens: 10, OutputTokens: 5},
	}, nil
}

func TestAComponentIsStillRecordedWhenTheModelNeverEmits(t *testing.T) {
	// A model that burns its whole budget reading is not a reason to drop the
	// component. The parser already established its symbols, complexity and
	// error paths; those are facts whatever the model did with its turns, and
	// discarding them would take the file out of the risk register entirely.
	blackboard := memory.NewBlackboard("run")
	analyst := &Analyst{
		Source:             repo.NewLocalSource("../../testdata/fixtures/paymentsvc", 400),
		Provider:           &pagingProvider{},
		Blackboard:         blackboard,
		PerComponentBudget: guard.Budget{MaxIterations: 8, MaxToolCalls: 20},
	}

	reason := analyst.analyseOne(context.Background(), blackboard, model.SourceFile{
		Path: "internal/ledger/ledger.go", Language: model.LanguageGo, SizeBytes: 1000,
	})
	if reason != "" {
		t.Fatalf("a salvageable component must not count as a failure, got %q", reason)
	}

	models := blackboard.ComponentModels()
	if len(models) != 1 {
		t.Fatalf("expected the component to survive, got %d", len(models))
	}
	if len(models[0].PublicSymbols) == 0 {
		t.Error("the parser's symbols should have been kept")
	}
	// Honesty about what it is: no responsibility was ever written, and the
	// confidence has to say so or the report would overstate itself.
	if models[0].Responsibility != "" {
		t.Errorf("no responsibility was recorded, so none should be claimed: %q", models[0].Responsibility)
	}
	if models[0].Confidence > 0.5 {
		t.Errorf("a structural-only analysis must not claim high confidence, got %v", models[0].Confidence)
	}
	if len(blackboard.Gaps()) == 0 {
		t.Error("the shortfall must be recorded as a gap so the report can show it")
	}
}

func TestTheAnalystNeverMentionsAToolItDoesNotHave(t *testing.T) {
	// This is what turned a helpful sentence into a loop: repo_read_file told
	// the model to "call repo_tree to list valid paths", and no agent registers
	// repo_tree. The model spent its budget reaching for a tool that was never
	// there.
	blackboard := memory.NewBlackboard("run")
	analyst := &Analyst{
		Source:     repo.NewLocalSource("../../testdata/fixtures/paymentsvc", 400),
		Blackboard: blackboard,
	}
	registry := analyst.registryFor(blackboard, model.ComponentModel{})

	offered := map[string]bool{}
	visibleText := AnalystSystemPrompt
	for _, schema := range registry.Schemas() {
		offered[schema.Name] = true
		visibleText += " " + schema.Description
	}
	for _, toolName := range []string{"repo_tree", "repo_commits", "repo_walk"} {
		if offered[toolName] {
			continue
		}
		if strings.Contains(visibleText, toolName) {
			t.Errorf("the analyst is told about %q but cannot call it", toolName)
		}
	}
}

func TestNoToolPointsAtSomethingThatDoesNotExistAtAll(t *testing.T) {
	// The earlier check covered prompts and tool descriptions. It missed the
	// correctable error strings *inside* tools, where code_parse_go was still
	// telling the model to "use code.parse_python" — a tool that has never
	// existed — and to "call repo_tree", which no agent registers. A model
	// following either one has no legal move.
	sourceFiles, err := filepath.Glob("../tool/*/*.go")
	if err != nil {
		t.Fatal(err)
	}
	nested, _ := filepath.Glob("../tool/*/*/*.go")
	sourceFiles = append(sourceFiles, nested...)

	// Every tool this system actually implements.
	implemented := map[string]bool{
		"repo_tree": true, "repo_read_file": true, "repo_commits": true, "repo_walk": true,
		"code_parse_go": true, "analysis_emit_component": true, "scenario_emit": true,
		"review_emit": true, "jira_push": true, "report_validate": true, "context_fetch": true,
	}
	mentioned := regexp.MustCompile(`\b(repo|code|analysis|scenario|review|jira|report|context)[._][a-z_]+\b`)

	for _, sourceFile := range sourceFiles {
		if strings.HasSuffix(sourceFile, "_test.go") {
			continue
		}
		body, err := os.ReadFile(sourceFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range mentioned.FindAllString(string(body), -1) {
			if !implemented[name] {
				t.Errorf("%s names %q, which is not a tool in this system",
					filepath.Base(sourceFile), name)
			}
		}
	}
}

func TestALargeFileReachesTheAnalystInOneRead(t *testing.T) {
	// Paging a source file one 400-line window at a time spent the iteration
	// budget before the model ever reached the emit call — and every page
	// carried different arguments, so the repeat guard never saw a repeat.
	sourceLines := make([]string, 900)
	for index := range sourceLines {
		sourceLines[index] = fmt.Sprintf("// line %d", index+1)
	}
	readTool := &repo.ReadFileTool{
		Source:   staticSource{"big.go": strings.Join(sourceLines, "\n")},
		MaxLines: 1500,
	}
	result, err := readTool.Invoke(context.Background(), json.RawMessage(`{"path":"big.go"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Content, "further lines were not shown") {
		t.Fatal("a 900-line file must arrive whole at the analyst's read limit")
	}
}

// staticSource serves fixed content, so a read limit can be exercised without
// a fixture file of a particular length.
type staticSource map[string]string

func (source staticSource) Name() string { return "static" }
func (source staticSource) Tree(context.Context) ([]repo.Entry, error) {
	return nil, errors.New("not needed")
}
func (source staticSource) ReadFile(_ context.Context, path string) (string, error) {
	content, known := source[path]
	if !known {
		return "", &repo.ErrNotFound{Path: path}
	}
	return content, nil
}

// promptsAndRegistries pairs each agent's prompt text with the tools it will
// actually be offered.
func promptsAndRegistries(t *testing.T) map[string][]string {
	t.Helper()
	blackboard := memory.NewBlackboard("run")
	source := repo.NewLocalSource("../../testdata/fixtures/paymentsvc", 400)

	analyst := &Analyst{Source: source, Blackboard: blackboard}
	analystTools := []string{}
	for _, schema := range analyst.registryFor(blackboard, model.ComponentModel{}).Schemas() {
		analystTools = append(analystTools, schema.Name)
	}
	return map[string][]string{AnalystSystemPrompt: analystTools}
}

func TestEveryToolNamedInAPromptIsOfferedToTheModel(t *testing.T) {
	// The failure this guards against is silent and total: a prompt that says
	// "call analysis_emit_component" while the model's tool list holds some
	// other spelling produces eight wasted iterations per file and an empty
	// plan, with no error anywhere. The prompt and the tool list are one
	// contract and nothing else checks it.
	toolNamePattern := regexp.MustCompile(`\b(repo|code|analysis|scenario|review|jira|report)[._][a-z_]+\b`)

	for promptText, offeredTools := range promptsAndRegistries(t) {
		offered := map[string]bool{}
		for _, toolName := range offeredTools {
			offered[toolName] = true
		}
		for _, mentioned := range toolNamePattern.FindAllString(promptText, -1) {
			if !offered[mentioned] {
				t.Errorf("the prompt names %q but the model is offered only %v", mentioned, offeredTools)
			}
		}
	}
}

func TestNoToolNameNeedsRewritingForAnyBackend(t *testing.T) {
	// Tool names must be legal on every backend as written. OpenAI enforces
	// ^[a-zA-Z0-9_-]+$; a name needing translation there would stop matching
	// the name the prompts use.
	legal := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	blackboard := memory.NewBlackboard("run")
	analyst := &Analyst{
		Source: repo.NewLocalSource("../../testdata/fixtures/paymentsvc", 400), Blackboard: blackboard,
	}
	for _, schema := range analyst.registryFor(blackboard, model.ComponentModel{}).Schemas() {
		if !legal.MatchString(schema.Name) {
			t.Errorf("tool %q must be renamed at the source, not translated on the wire", schema.Name)
		}
	}
}

func TestTheAnalystsOwnFileReadCanNeverBeSpilled(t *testing.T) {
	// This is the invariant behind the whole failure: the analyst reads the one
	// file it was told to analyse, and that read was being replaced by a
	// thousand-character digest plus a handle nothing could resolve. The
	// model's only remaining move was to read again — eight times, then stop.
	//
	// A read the agent is *designed* to make must fit under the spill
	// threshold, by construction, not by luck.
	loop := NewLoop(llm.NewFakeProvider(), tool.NewRegistry(), memory.NewBlackboard("r"), guard.Chain{})

	// Estimating this from a line count and an assumed bytes-per-line was how
	// it slipped through the first time: 1500 lines of dense Go with
	// line-number prefixes overshot the threshold, spilled, and cost five
	// context_fetch round trips and 81k tokens on one file. The cap is now a
	// byte cap, so the comparison is exact.
	if AnalystReadMaxBytes >= loop.SpillThresholdBytes {
		t.Fatalf("an analyst read can reach %d bytes but results spill at %d; "+
			"its own file would be digested and fetched back in pieces",
			AnalystReadMaxBytes, loop.SpillThresholdBytes)
	}
}

func TestAReadIsTruncatedToItsByteCapNotJustItsLineCap(t *testing.T) {
	// Dense lines are the case that broke the estimate. One very long line must
	// not carry the result past the cap.
	denseLines := make([]string, 2000)
	for index := range denseLines {
		denseLines[index] = strings.Repeat("x", 200)
	}
	readTool := &repo.ReadFileTool{
		Source:   staticSource{"dense.go": strings.Join(denseLines, "\n")},
		MaxLines: 2000, MaxBytes: AnalystReadMaxBytes,
	}
	result, err := readTool.Invoke(context.Background(), json.RawMessage(`{"path":"dense.go"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A little slack for the final line and the trailing notice.
	if len(result.Content) > AnalystReadMaxBytes+1000 {
		t.Fatalf("the read returned %d bytes against a %d cap",
			len(result.Content), AnalystReadMaxBytes)
	}
	if !strings.Contains(result.Content, "further lines were not shown") {
		t.Error("a truncated read must say so")
	}
}

func TestOneComponentsReadsFitInsideItsTokenBudget(t *testing.T) {
	// The spill fix was necessary but not sufficient: a read can be small
	// enough not to spill and still be too expensive, because every turn
	// re-sends the whole history. A 108 KB read cost 81k tokens against a 60k
	// budget — not in one turn, but accumulated across six.
	//
	// The rule that has to hold: the largest read, re-sent on every iteration
	// an agent is allowed, must fit the budget with room for the prompt and the
	// emit call.
	const analystTokenBudget = 60000 // pipeline.Config, analyse phase
	const analystMaxIterations = 8
	const bytesPerToken = 4

	readTokens := AnalystReadMaxBytes / bytesPerToken
	// A read lands in the history and is re-sent on every later turn. Half the
	// iterations is a fair expectation of how many carry it.
	worstCaseTokens := readTokens * (analystMaxIterations / 2)

	if worstCaseTokens >= analystTokenBudget {
		t.Fatalf("a %d-byte read (~%d tokens) re-sent across %d turns needs ~%d tokens, "+
			"but the budget is %d; the agent would be stopped mid-analysis",
			AnalystReadMaxBytes, readTokens, analystMaxIterations/2,
			worstCaseTokens, analystTokenBudget)
	}
}

// countingProvider records how many separate agent invocations it served.
type countingProvider struct {
	mutex sync.Mutex
	calls int
	seen  map[string]bool
}

func (provider *countingProvider) Name() string { return "counting" }
func (provider *countingProvider) Limits(llm.Tier) llm.Limits {
	return llm.Limits{ContextWindowTokens: 100000, MaxOutputTokens: 2000}
}
func (provider *countingProvider) CountTokens(context.Context, llm.Request) (int, error) {
	return 0, nil
}
func (provider *countingProvider) Complete(_ context.Context, request llm.Request) (*llm.Response, error) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	provider.calls++
	if provider.seen == nil {
		provider.seen = map[string]bool{}
	}
	for _, message := range request.Messages {
		for _, riskID := range []string{"RISK-a", "RISK-b", "RISK-c"} {
			if strings.Contains(message.Text, riskID) {
				provider.seen[riskID] = true
			}
		}
	}
	return &llm.Response{StopReason: llm.StopEndTurn, Text: "done"}, nil
}

func TestARevisionTouchesOnlyTheRisksThatWereFlagged(t *testing.T) {
	// One blocking finding on one scenario used to cost a second full pass over
	// every risk, on the strongest tier, with nothing printed. Nothing about
	// the unflagged risks had changed, so nothing about them needed redoing.
	blackboard := memory.NewBlackboard("run")
	blackboard.SetRiskRegister(model.RiskRegister{Risks: []model.Risk{
		{ID: "RISK-a", Level: model.RiskCritical}, {ID: "RISK-b", Level: model.RiskCritical},
		{ID: "RISK-c", Level: model.RiskCritical},
	}})

	provider := &countingProvider{}
	author := &Author{
		Source:   repo.NewLocalSource("../../testdata/fixtures/paymentsvc", 400),
		Provider: provider, Concurrency: 3,
		PerRiskBudget: guard.Budget{MaxIterations: 2, MaxToolCalls: 4},
	}

	err := author.Revise(context.Background(), blackboard, []model.RevisionRequest{
		{RiskRef: "RISK-b", Severity: model.ReviewBlocking, Issue: "expected result is not checkable"},
		{RiskRef: "RISK-c", Severity: model.ReviewMinor, Issue: "could be clearer"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.seen["RISK-a"] || provider.seen["RISK-c"] {
		t.Errorf("only the blocking risk should be revised, but saw %v", provider.seen)
	}
	if !provider.seen["RISK-b"] {
		t.Error("the flagged risk should have been revised")
	}
}
