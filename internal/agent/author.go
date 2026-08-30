package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool"
	"github.com/giri-ms19/testplan-agent/internal/tool/code"
	"github.com/giri-ms19/testplan-agent/internal/tool/emit"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// AuthorSystemPrompt asks for scenarios a QA engineer could run without reading
// the source. That standard is the whole product: a scenario that only makes
// sense to someone who has already read the code has saved nobody any work.
const AuthorSystemPrompt = `You are writing test scenarios for one specific risk in a codebase.

Read the code the risk points at, then call scenario.emit once with every scenario
you intend to write, and stop.

Standards:
- A scenario must be executable by a QA engineer who has not read the source.
- Every scenario needs exactly one observable, checkable expected result. If you
  cannot state what a person would look at to decide pass or fail, the scenario
  is not ready.
- Cover the failure paths, not just the happy path. Boundary and negative cases
  are where defects actually live.
- Do not restate the same scenario with different words. Fewer, sharper scenarios
  beat a long list.
- Do not write scenarios for code the risk does not cover.`

// CriticSystemPrompt asks for findings the critic would defend. An unbounded
// critic finds infinite problems, so the instruction to raise only defensible
// findings is a cost control as much as a quality one.
const CriticSystemPrompt = `You are reviewing a catalogue of test scenarios before a human reads it.

Call review.emit exactly once and stop. Pass an empty findings array to approve.

Raise a finding only if you would defend it to the author. Look for:
- A scenario whose expected result is not observable or not checkable.
- Two scenarios that test the same thing.
- A prioritised risk with no scenario covering it.
- A scenario that could not be executed without reading the source.
- Missing negative or boundary coverage on a failure path the analysis identified.

Severity: "blocking" means a human should not see the catalogue until it is fixed.
"major" is worth the author's time. "minor" is a nitpick — raise few of these.

Do not rewrite scenarios yourself. Do not praise. Report only what is wrong.`

// Author writes scenarios for each prioritised risk.
type Author struct {
	Source        repo.Source
	Provider      llm.Provider
	PerRiskBudget guard.Budget
	Logf          func(format string, arguments ...any)
}

func (author *Author) Name() string { return "author" }

func (author *Author) logf(format string, arguments ...any) {
	if author.Logf != nil {
		author.Logf(format, arguments...)
	}
}

// Write produces scenarios for every prioritised risk.
func (author *Author) Write(ctx context.Context, blackboard *memory.Blackboard) error {
	riskRegister := blackboard.RiskRegister()
	if riskRegister == nil {
		return fmt.Errorf("author: no risk register; the risk phase must run first")
	}
	targetRisks := riskRegister.Prioritised()
	if len(targetRisks) == 0 {
		// Nothing critical or high is a legitimate finding about a codebase,
		// not a failure. Fall back to the top medium risks so the plan is not
		// empty, and say so.
		targetRisks = topRisks(riskRegister.Risks, 3)
		blackboard.NoteGap(author.Name(), "prioritisation",
			"no critical or high risks were found; scenarios were written for the highest-scoring components instead")
	}

	for _, targetRisk := range targetRisks {
		if err := ctx.Err(); err != nil {
			blackboard.NoteGap(author.Name(), targetRisk.ID, "run cancelled before authoring")
			break
		}
		author.writeForRisk(ctx, blackboard, targetRisk)
	}

	if len(blackboard.Scenarios()) == 0 {
		return fmt.Errorf("author: no scenarios were produced")
	}
	return nil
}

func (author *Author) writeForRisk(ctx context.Context, blackboard *memory.Blackboard, targetRisk model.Risk) {
	riskRegistry := tool.NewRegistry()
	riskRegistry.Register(&repo.ReadFileTool{Source: author.Source, MaxLines: 400}, tool.NetworkPolicy())
	riskRegistry.Register(&code.ParseGoTool{Source: author.Source}, tool.LocalPolicy())
	riskRegistry.Register(&emit.ScenarioTool{
		Blackboard: blackboard,
		Risk:       targetRisk,
		IDPrefix:   scenarioPrefixFor(targetRisk),
	}, tool.LocalPolicy())

	repeatGuard := guard.DefaultRepeatCallGuard()
	riskGuards := guard.Chain{
		guard.NewBudgetGuard(author.PerRiskBudget),
		repeatGuard,
		guard.NewProgressGuard(3),
		guard.NewScopeGuard(riskRegistry),
	}

	authorLoop := NewLoop(author.Provider, riskRegistry, blackboard, riskGuards)
	authorLoop.RepeatGuard = repeatGuard

	outcome, err := authorLoop.Run(ctx, Task{
		AgentName:    "author:" + targetRisk.ID,
		Tier:         llm.TierStrong,
		SystemPrompt: AuthorSystemPrompt,
		Instruction:  buildAuthorInstruction(targetRisk, blackboard),
	})
	if err != nil {
		blackboard.NoteGap(author.Name(), targetRisk.ID, "authoring failed: "+err.Error())
		author.logf("author: %s failed: %v", targetRisk.ID, err)
		return
	}
	if outcome.StoppedBy != nil {
		blackboard.NoteGap(author.Name(), targetRisk.ID,
			"authoring stopped early ("+string(outcome.StoppedBy.Reason)+"): "+outcome.StoppedBy.Detail)
	}
}

func buildAuthorInstruction(targetRisk model.Risk, blackboard *memory.Blackboard) string {
	var instructionBuilder strings.Builder
	fmt.Fprintf(&instructionBuilder, "Risk %s (%s, score %.1f) in component %s.\n",
		targetRisk.ID, targetRisk.Level, targetRisk.Score, targetRisk.ComponentName)
	fmt.Fprintf(&instructionBuilder, "Rationale: %s\n", targetRisk.Rationale)
	fmt.Fprintf(&instructionBuilder, "Signals: %s\n", strings.Join(targetRisk.Signals, "; "))
	fmt.Fprintf(&instructionBuilder, "Anchor: %s", targetRisk.Ref.Path)
	if targetRisk.Ref.Symbol != "" {
		fmt.Fprintf(&instructionBuilder, " at %s (lines %d-%d)",
			targetRisk.Ref.Symbol, targetRisk.Ref.StartLine, targetRisk.Ref.EndLine)
	}
	instructionBuilder.WriteString("\n\n")

	for _, componentModel := range blackboard.ComponentModels() {
		if componentModel.ComponentName != targetRisk.ComponentName {
			continue
		}
		fmt.Fprintf(&instructionBuilder, "What the analyst found:\n  %s\n", componentModel.Responsibility)
		if len(componentModel.ErrorPaths) > 0 {
			fmt.Fprintf(&instructionBuilder, "  Failure paths: %s\n", strings.Join(componentModel.ErrorPaths, "; "))
		}
		if len(componentModel.SideEffects) > 0 {
			fmt.Fprintf(&instructionBuilder, "  Side effects: %s\n", strings.Join(componentModel.SideEffects, "; "))
		}
		if len(componentModel.PublicSymbols) > 0 {
			instructionBuilder.WriteString("  Exported surface:\n")
			for _, symbol := range componentModel.PublicSymbols {
				if symbol.Exported && symbol.Signature != "" {
					fmt.Fprintf(&instructionBuilder, "    %s\n", symbol.Signature)
				}
			}
		}
		break
	}

	// Outstanding critic findings for this risk go into the next round's
	// instruction. This is what makes the revision cycle converge rather than
	// repeat.
	outstandingFindings := findingsForRisk(blackboard.RevisionRequests(), targetRisk.ID)
	if len(outstandingFindings) > 0 {
		instructionBuilder.WriteString("\nA reviewer raised these problems with your previous scenarios. Fix them:\n")
		for _, finding := range outstandingFindings {
			fmt.Fprintf(&instructionBuilder, "  - [%s] %s", finding.Severity, finding.Issue)
			if finding.Suggestion != "" {
				fmt.Fprintf(&instructionBuilder, " (suggestion: %s)", finding.Suggestion)
			}
			instructionBuilder.WriteString("\n")
		}
	}

	instructionBuilder.WriteString("\nRead the anchor, then call scenario.emit.")
	return instructionBuilder.String()
}

func findingsForRisk(findings []model.RevisionRequest, riskID string) []model.RevisionRequest {
	matching := []model.RevisionRequest{}
	for _, finding := range findings {
		if finding.RiskRef == riskID || finding.RiskRef == "" {
			matching = append(matching, finding)
		}
	}
	return matching
}

func topRisks(risks []model.Risk, count int) []model.Risk {
	if len(risks) < count {
		count = len(risks)
	}
	return risks[:count]
}

func scenarioPrefixFor(targetRisk model.Risk) string {
	slug := strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(targetRisk.ComponentName)
	return "TS-" + strings.ToLower(slug)
}

// Critic reviews the catalogue.
type Critic struct {
	Provider llm.Provider
	Budget   guard.Budget
}

func (critic *Critic) Name() string { return "critic" }

// Review records findings on the blackboard.
func (critic *Critic) Review(ctx context.Context, blackboard *memory.Blackboard) error {
	scenarios := blackboard.Scenarios()
	if len(scenarios) == 0 {
		return fmt.Errorf("critic: there are no scenarios to review")
	}

	criticRegistry := tool.NewRegistry()
	criticRegistry.Register(&emit.ReviewTool{Blackboard: blackboard}, tool.LocalPolicy())

	repeatGuard := guard.DefaultRepeatCallGuard()
	criticGuards := guard.Chain{
		guard.NewBudgetGuard(critic.Budget),
		repeatGuard,
		guard.NewProgressGuard(3),
		guard.NewScopeGuard(criticRegistry),
	}

	criticLoop := NewLoop(critic.Provider, criticRegistry, blackboard, criticGuards)
	criticLoop.RepeatGuard = repeatGuard

	outcome, err := criticLoop.Run(ctx, Task{
		AgentName:    critic.Name(),
		Tier:         llm.TierStrong,
		SystemPrompt: CriticSystemPrompt,
		Instruction:  buildCriticInstruction(blackboard),
	})
	if err != nil {
		// A critic that cannot run is a degraded review, not a failed plan. The
		// gap is recorded so a reviewer knows the catalogue was unreviewed.
		blackboard.NoteGap(critic.Name(), "review", "the critic could not run: "+err.Error())
		return nil
	}
	if outcome.StoppedBy != nil {
		blackboard.NoteGap(critic.Name(), "review",
			"review stopped early ("+string(outcome.StoppedBy.Reason)+"): "+outcome.StoppedBy.Detail)
	}
	return nil
}

func buildCriticInstruction(blackboard *memory.Blackboard) string {
	var instructionBuilder strings.Builder
	riskRegister := blackboard.RiskRegister()

	instructionBuilder.WriteString("Prioritised risks that must each have at least one scenario:\n")
	if riskRegister != nil {
		for _, prioritisedRisk := range riskRegister.Prioritised() {
			fmt.Fprintf(&instructionBuilder, "  %s (%s) — %s\n",
				prioritisedRisk.ID, prioritisedRisk.Level, prioritisedRisk.Rationale)
		}
	}

	instructionBuilder.WriteString("\nScenario catalogue:\n")
	for _, scenario := range blackboard.Scenarios() {
		fmt.Fprintf(&instructionBuilder, "\n%s [%s, %s] %s\n", scenario.ID, scenario.Priority, scenario.Type, scenario.Title)
		fmt.Fprintf(&instructionBuilder, "  covers: %s at %s\n", scenario.RiskRef, scenario.SourceRef.Symbol)
		for _, step := range scenario.Steps {
			fmt.Fprintf(&instructionBuilder, "  %d. %s\n", step.Ordinal, step.Action)
		}
		fmt.Fprintf(&instructionBuilder, "  expected: %s\n", scenario.ExpectedResult)
	}
	instructionBuilder.WriteString("\nCall review.emit with your findings.")
	return instructionBuilder.String()
}

// RevisionCycle runs Author and Critic to convergence, bounded on two axes: a
// hard round cap, and a requirement that the blocking-issue count strictly
// decrease. Either one alone would be defeatable — a critic that finds one new
// problem per round would run forever under the count rule, and the cap alone
// would let a converged catalogue burn its full budget.
type RevisionCycle struct {
	Author    *Author
	Critic    *Critic
	MaxRounds int
	Logf      func(format string, arguments ...any)
}

func (revisionCycle *RevisionCycle) logf(format string, arguments ...any) {
	if revisionCycle.Logf != nil {
		revisionCycle.Logf(format, arguments...)
	}
}

// Run performs authoring, then up to MaxRounds of review and revision.
func (revisionCycle *RevisionCycle) Run(ctx context.Context, blackboard *memory.Blackboard) error {
	if err := revisionCycle.Author.Write(ctx, blackboard); err != nil {
		return err
	}

	maxRounds := revisionCycle.MaxRounds
	if maxRounds < 1 {
		maxRounds = 2
	}

	previousBlockingCount := -1
	for roundNumber := 1; roundNumber <= maxRounds; roundNumber++ {
		if err := revisionCycle.Critic.Review(ctx, blackboard); err != nil {
			return err
		}
		findings := blackboard.RevisionRequests()
		blockingCount := countBlocking(findings)
		revisionCycle.logf("revision round %d: %d findings, %d blocking", roundNumber, len(findings), blockingCount)

		if blockingCount == 0 {
			return nil
		}
		if previousBlockingCount >= 0 && blockingCount >= previousBlockingCount {
			// Not converging. Accept what exists and let the reviewer see the
			// outstanding findings rather than spending another round.
			blackboard.NoteGap("critic", "revision",
				fmt.Sprintf("stopped after round %d: blocking findings did not decrease (%d then %d)",
					roundNumber, previousBlockingCount, blockingCount))
			return nil
		}
		previousBlockingCount = blockingCount

		if roundNumber == maxRounds {
			blackboard.NoteGap("critic", "revision",
				fmt.Sprintf("%d blocking findings remain after %d rounds and are listed in the report",
					blockingCount, maxRounds))
			return nil
		}
		if err := revisionCycle.Author.Write(ctx, blackboard); err != nil {
			return err
		}
	}
	return nil
}

func countBlocking(findings []model.RevisionRequest) int {
	blockingCount := 0
	for _, finding := range findings {
		if finding.Blocking() {
			blockingCount++
		}
	}
	return blockingCount
}

// ScenarioFallback writes a minimal, honest scenario per prioritised risk when
// no provider is available. It exists so the pipeline shape is demonstrable
// end to end without a model, and every scenario it emits is marked low
// confidence and non-automatable so nobody mistakes it for real authoring.
func ScenarioFallback(blackboard *memory.Blackboard) error {
	riskRegister := blackboard.RiskRegister()
	if riskRegister == nil {
		return fmt.Errorf("author: no risk register")
	}
	targetRisks := riskRegister.Prioritised()
	if len(targetRisks) == 0 {
		targetRisks = topRisks(riskRegister.Risks, 3)
	}

	for _, targetRisk := range targetRisks {
		symbolName := targetRisk.Ref.Symbol
		if symbolName == "" {
			symbolName = targetRisk.ComponentName
		}
		blackboard.AddScenarios(model.TestScenario{
			ID:            scenarioPrefixFor(targetRisk) + "-001",
			Title:         fmt.Sprintf("Exercise every failure path of %s", symbolName),
			ComponentName: targetRisk.ComponentName,
			SourceRef:     targetRisk.Ref,
			Type:          model.ScenarioNegative,
			Priority:      model.PriorityForRisk(targetRisk.Level),
			RiskRef:       targetRisk.ID,
			Steps: []model.Step{
				{Ordinal: 1, Action: fmt.Sprintf("Call %s once for each failure path listed in the analysis", symbolName)},
				{Ordinal: 2, Action: "Record the returned error for each"},
			},
			ExpectedResult: "Each failure path returns its own distinct, identifiable error.",
			Automatable:    false,
			Confidence:     0.3,
			Notes:          "Generated without a model. This is a placeholder for a real scenario, not a substitute for one.",
		})
	}
	blackboard.NoteGap("author", "scenarios",
		"no model provider was configured; scenarios are structural placeholders only")
	return nil
}
