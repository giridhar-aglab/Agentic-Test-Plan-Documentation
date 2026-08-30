package pipeline

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/approval"
	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/render"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

const fixtureDirectory = "../../testdata/fixtures/paymentsvc"

func fixtureSource() repo.Source { return repo.NewLocalSource(fixtureDirectory, 400) }

func baseConfig() Config {
	return Config{
		Source:             fixtureSource(),
		RepositoryName:     "paymentsvc",
		CommitSHA:          "0123456789abcdef",
		AnalystConcurrency: 2,
		MaxRevisionRounds:  2,
		RunBudget:          guard.Budget{MaxWallClock: 2 * time.Minute},
	}
}

func TestPipelineRunsEveryPhaseWithoutAProvider(t *testing.T) {
	// The deterministic fallbacks exist so the whole pipeline shape is
	// demonstrable with no network and no API key. If this breaks, nothing else
	// in the suite can be trusted either.
	blackboard := memory.NewBlackboard("no-provider")
	runReport, err := Build(baseConfig(), blackboard).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runReport.Aborted {
		t.Fatalf("run aborted: %s", runReport.AbortReason)
	}

	expectedPhases := []string{"survey", "analyse", "risk", "author", "validate", "approval"}
	observedPhases := map[string]bool{}
	for _, phaseResult := range runReport.PhaseResults {
		observedPhases[phaseResult.Name] = true
	}
	for _, phaseName := range expectedPhases {
		if !observedPhases[phaseName] {
			t.Errorf("phase %s never ran", phaseName)
		}
	}

	if blackboard.RepoMap() == nil {
		t.Error("no repository map")
	}
	if len(blackboard.ComponentModels()) == 0 {
		t.Error("no components analysed")
	}
	if blackboard.RiskRegister() == nil {
		t.Error("no risk register")
	}
	if len(blackboard.Scenarios()) == 0 {
		t.Error("no scenarios")
	}
}

func TestFallbackScenariosAreHonestlyMarked(t *testing.T) {
	// Placeholder scenarios must never be mistaken for real authoring, or the
	// report quietly overstates what the system did.
	blackboard := memory.NewBlackboard("fallback-honesty")
	if _, err := Build(baseConfig(), blackboard).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, scenario := range blackboard.Scenarios() {
		if scenario.Confidence >= 0.6 {
			t.Errorf("%s claims confidence %.1f without a model", scenario.ID, scenario.Confidence)
		}
		if scenario.Automatable {
			t.Errorf("%s claims to be automatable but was generated structurally", scenario.ID)
		}
	}
	foundNotice := false
	for _, gap := range blackboard.Gaps() {
		if strings.Contains(gap.Reason, "no model provider") {
			foundNotice = true
		}
	}
	if !foundNotice {
		t.Error("the report must say that scenarios were generated without a model")
	}
}

// scriptedProvider drives the full model path deterministically: it answers
// each agent by name, so the Analyst, Author and Critic can all be exercised
// without a network.
func scriptedProvider(t *testing.T) llm.Provider {
	t.Helper()
	return &routingProvider{t: t}
}

type routingProvider struct {
	t *testing.T
}

func (provider *routingProvider) Name() string { return "scripted" }

func (provider *routingProvider) Limits(tier llm.Tier) llm.Limits {
	return llm.Limits{ContextWindowTokens: 40000, MaxOutputTokens: 4000}
}

func (provider *routingProvider) CountTokens(ctx context.Context, request llm.Request) (int, error) {
	return llm.EstimateTokens(request), nil
}

// Complete answers based on which emit tool is in scope, which is a reliable
// proxy for which agent is asking. Each agent gets exactly one turn: emit, then
// the loop ends because no further tool call is requested.
func (provider *routingProvider) Complete(ctx context.Context, request llm.Request) (*llm.Response, error) {
	availableTools := map[string]bool{}
	for _, toolSchema := range request.Tools {
		availableTools[toolSchema.Name] = true
	}

	// A second call for the same agent means the emit already happened, so the
	// turn ends. Without this the loop would spin until a guard stopped it.
	if alreadyEmitted(request) {
		return &llm.Response{Text: "done", StopReason: llm.StopEndTurn}, nil
	}

	switch {
	case availableTools["analysis.emit_component"]:
		return toolCallResponse("analysis.emit_component", map[string]any{
			"responsibility": "Applies entries to accounts and enforces balance and currency rules.",
			"errorPaths": []string{
				"entry does not belong to account", "account is frozen",
				"currency mismatch", "zero amount", "insufficient funds", "balance overflow",
			},
			"sideEffects": []string{"mutates the returned account balance"},
			"confidence":  0.9,
		}), nil

	case availableTools["scenario.emit"]:
		// Use the anchor symbol the instruction actually names. A real model
		// that ignored it would be caught by report.validate, which is exactly
		// what TestValidationCatchesAHallucinatedSymbol demonstrates.
		anchorSymbol := anchorSymbolFrom(request)
		return toolCallResponse("scenario.emit", map[string]any{
			"scenarios": []map[string]any{
				{
					"title": "Reject an operation that violates the component's precondition",
					"type":  "negative", "symbol": anchorSymbol,
					"preconditions": []string{"State that violates one documented precondition"},
					"steps": []string{
						"Invoke " + anchorSymbol + " with an argument that breaches the precondition",
						"Inspect the returned error and the returned state",
					},
					"expectedResult": "The call returns a named error and leaves the input state unchanged",
					"automatable":    true,
				},
				{
					"title": "Reject an operation against state that forbids it",
					"type":  "negative", "symbol": anchorSymbol,
					"steps":          []string{"Invoke " + anchorSymbol + " against state that forbids the operation"},
					"expectedResult": "The call returns a distinct error and no state is mutated",
					"automatable":    true,
				},
			},
		}), nil

	case availableTools["review.emit"]:
		return toolCallResponse("review.emit", map[string]any{
			"findings": []map[string]any{},
		}), nil
	}
	return &llm.Response{Text: "nothing to do", StopReason: llm.StopEndTurn}, nil
}

// anchorSymbolFrom reads the symbol the Author instruction points at. Real
// agents get this from the risk anchor in their prompt; the script does the
// same so it exercises the same path.
func anchorSymbolFrom(request llm.Request) string {
	for _, message := range request.Messages {
		_, afterAnchor, found := strings.Cut(message.Text, "Anchor: ")
		if !found {
			continue
		}
		_, afterAt, found := strings.Cut(afterAnchor, " at ")
		if !found {
			continue
		}
		symbolName, _, _ := strings.Cut(afterAt, " ")
		if symbolName != "" {
			return strings.TrimSpace(symbolName)
		}
	}
	return "Run"
}

func alreadyEmitted(request llm.Request) bool {
	for _, message := range request.Messages {
		for _, toolResult := range message.ToolResults {
			if strings.Contains(toolResult.Content, "You are done; stop now.") {
				return true
			}
		}
	}
	return false
}

func toolCallResponse(toolName string, arguments any) *llm.Response {
	encodedArguments, err := json.Marshal(arguments)
	if err != nil {
		panic(err)
	}
	return &llm.Response{
		ToolCalls: []llm.ToolCall{
			{ID: "call-1", ToolName: toolName, Arguments: encodedArguments},
		},
		StopReason: llm.StopToolUse,
		Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
	}
}

func TestPipelineProducesAValidPlanWithAProvider(t *testing.T) {
	configuration := baseConfig()
	configuration.Provider = scriptedProvider(t)

	blackboard := memory.NewBlackboard("scripted")
	runReport, err := Build(configuration, blackboard).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runReport.Aborted {
		t.Fatalf("run aborted: %s", runReport.AbortReason)
	}

	scenarios := blackboard.Scenarios()
	if len(scenarios) == 0 {
		t.Fatal("no scenarios were produced")
	}

	// The model-supplied error paths must reach the component model, which is
	// what drives risk scoring upward for the ledger.
	foundRichAnalysis := false
	for _, componentModel := range blackboard.ComponentModels() {
		if len(componentModel.ErrorPaths) >= 5 {
			foundRichAnalysis = true
		}
	}
	if !foundRichAnalysis {
		t.Error("the model's error-path analysis did not reach the blackboard")
	}

	// The whole point of the run: what it produced must survive validation.
	validationReport := approval.Validate(blackboard)
	if !validationReport.OK() {
		t.Fatalf("a scripted good run must pass validation, got %v", validationReport.Blocking())
	}
}

func TestRenderedPlanContainsEverySection(t *testing.T) {
	configuration := baseConfig()
	configuration.Provider = scriptedProvider(t)

	blackboard := memory.NewBlackboard("render")
	if _, err := Build(configuration, blackboard).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	validationReport := approval.Validate(blackboard)
	renderedPlan := render.PlanMarkdown(blackboard, render.Options{
		SourceLinkBase: "https://github.com/owner/repo/blob/main",
		Validation:     &validationReport,
	})

	for _, expectedSection := range []string{
		"# Test Plan — paymentsvc",
		"## Summary", "## Run metadata", "## Scope",
		"## Test considerations", "### Risk register",
		"## Scenario catalogue", "## Traceability",
	} {
		if !strings.Contains(renderedPlan, expectedSection) {
			t.Errorf("the plan is missing %q", expectedSection)
		}
	}

	// Source links are what make a scenario checkable in one click rather than
	// a search.
	if !strings.Contains(renderedPlan, "https://github.com/owner/repo/blob/main/internal/ledger/ledger.go#L") {
		t.Error("scenarios must link to the exact lines they test")
	}
	if !strings.Contains(renderedPlan, "Reject an operation that violates") {
		t.Error("the scenario catalogue is missing its scenarios")
	}
}

func TestRenderedPlanIsByteIdenticalAcrossRenders(t *testing.T) {
	configuration := baseConfig()
	configuration.Provider = scriptedProvider(t)

	blackboard := memory.NewBlackboard("determinism")
	if _, err := Build(configuration, blackboard).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first := render.PlanMarkdown(blackboard, render.Options{})
	second := render.PlanMarkdown(blackboard, render.Options{})
	if first != second {
		t.Fatal("two renders of the same blackboard differed, so runs cannot be diffed")
	}
}

func TestPhaseCallbacksFireInOrder(t *testing.T) {
	// An MCP caller polling tasks/get sees these; without them a long run is a
	// black box.
	observedPhases := []string{}
	configuration := baseConfig()
	configuration.OnPhase = func(phaseName string) {
		observedPhases = append(observedPhases, phaseName)
	}

	if _, err := Build(configuration, memory.NewBlackboard("phases")).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedOrder := "survey,analyse,risk,author,validate,approval"
	if strings.Join(observedPhases, ",") != expectedOrder {
		t.Fatalf("expected %s, got %s", expectedOrder, strings.Join(observedPhases, ","))
	}
}

func TestCancelledRunStillProducesAPartialPlan(t *testing.T) {
	// A cancelled run is a stop, not a crash. Whatever was learned before the
	// cancellation is still worth rendering.
	cancellableContext, cancelRun := context.WithCancel(context.Background())
	configuration := baseConfig()
	configuration.OnPhase = func(phaseName string) {
		if phaseName == "risk" {
			cancelRun()
		}
	}

	blackboard := memory.NewBlackboard("cancelled")
	runReport, err := Build(configuration, blackboard).Run(cancellableContext)
	if err != nil {
		t.Fatalf("cancellation must not be returned as an error: %v", err)
	}
	if !runReport.Aborted {
		t.Log("run completed before cancellation took effect; nothing to assert")
		return
	}
	renderedPlan := render.PlanMarkdown(blackboard, render.Options{})
	if !strings.Contains(renderedPlan, "# Test Plan") {
		t.Error("even an aborted run must render a document")
	}
	if !strings.Contains(renderedPlan, "## Gaps") {
		t.Error("an aborted run must say what it did not do")
	}
}

func TestApprovalPhaseBlocksPublication(t *testing.T) {
	// Phase 7 is a hard stop. Nothing may be published until a person has read
	// the report, and the plan must say so plainly.
	blackboard := memory.NewBlackboard("approval")
	if _, err := Build(baseConfig(), blackboard).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blackboard.Verdicts()) != 0 {
		t.Fatal("no verdict may exist before a human has reviewed the plan")
	}
	foundAwaitingNotice := false
	for _, gap := range blackboard.Gaps() {
		if strings.Contains(gap.Reason, "awaiting human approval") {
			foundAwaitingNotice = true
		}
	}
	if !foundAwaitingNotice {
		t.Error("the plan must record that it is awaiting approval and unpublished")
	}
}

// hallucinatingProvider names a symbol that does not exist in the file it was
// pointed at, which is the single most common way a generated plan is wrong in
// a way a reader would not notice.
type hallucinatingProvider struct{ routingProvider }

func (provider *hallucinatingProvider) Complete(ctx context.Context, request llm.Request) (*llm.Response, error) {
	availableTools := map[string]bool{}
	for _, toolSchema := range request.Tools {
		availableTools[toolSchema.Name] = true
	}
	if availableTools["scenario.emit"] && !alreadyEmitted(request) {
		return toolCallResponse("scenario.emit", map[string]any{
			"scenarios": []map[string]any{{
				"title": "Validate the transaction reconciliation ledger",
				"type":  "unit", "symbol": "ReconcileTransactions",
				"steps":          []string{"Call ReconcileTransactions with two opposing entries"},
				"expectedResult": "The ledger nets to zero",
			}},
		}), nil
	}
	return provider.routingProvider.Complete(ctx, request)
}

func TestValidationCatchesAHallucinatedSymbol(t *testing.T) {
	// A scenario anchored to a symbol that does not exist is worse than no
	// scenario: it looks authoritative and wastes a reviewer's trust. Catching
	// it by machine is the whole reason report.validate exists.
	configuration := baseConfig()
	configuration.Provider = &hallucinatingProvider{routingProvider{t: t}}

	blackboard := memory.NewBlackboard("hallucination")
	if _, err := Build(configuration, blackboard).Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	validationReport := approval.Validate(blackboard)
	if validationReport.OK() {
		t.Fatal("a scenario naming a non-existent symbol must not pass validation")
	}
	foundUnresolvableSymbol := false
	for _, defect := range validationReport.Blocking() {
		if defect.Rule == "unresolvable-symbol" {
			foundUnresolvableSymbol = true
		}
	}
	if !foundUnresolvableSymbol {
		t.Fatalf("expected an unresolvable-symbol defect, got %v", validationReport.Blocking())
	}

	// And it must reach the reader rather than being silently dropped.
	renderedPlan := render.PlanMarkdown(blackboard, render.Options{Validation: &validationReport})
	if !strings.Contains(renderedPlan, "structural defects") {
		t.Error("unresolved structural defects must be visible in the report")
	}
}
