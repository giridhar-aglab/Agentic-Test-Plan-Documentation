package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/approval"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

// Options control how the plan is rendered.
type Options struct {
	// SourceLinkBase, when set, turns every SourceRef into a link to the exact
	// lines. Judging whether a scenario is worth writing usually means looking
	// at the code that motivated it, and making that one click is most of the
	// difference between a report that gets read and one that gets skimmed.
	// Example: https://github.com/owner/repo/blob/<sha>
	SourceLinkBase string
	// Validation is folded into the summary so a reviewer sees what the machine
	// already checked and does not re-check it by hand.
	Validation *approval.Report
}

// PlanMarkdown renders the full test plan.
//
// The ordering is chosen for a reviewer rather than for completeness: what they
// need in order to decide where to spend attention comes before the material
// they are deciding about.
func PlanMarkdown(blackboard *memory.Blackboard, options Options) string {
	var reportBuilder strings.Builder
	repoMap := blackboard.RepoMap()

	repositoryName := "repository"
	if repoMap != nil && repoMap.RepositoryName != "" {
		repositoryName = repoMap.RepositoryName
	}
	fmt.Fprintf(&reportBuilder, "# Test Plan — %s\n\n", repositoryName)

	writeVerdictSummary(&reportBuilder, blackboard, options)
	writeRunMetadata(&reportBuilder, blackboard)
	if repoMap != nil {
		writeScope(&reportBuilder, *repoMap)
	}
	writeTestConsiderations(&reportBuilder, blackboard, options)
	writeScenarioCatalogue(&reportBuilder, blackboard, options)
	writeTraceability(&reportBuilder, blackboard)
	writeOutstandingFindings(&reportBuilder, blackboard)
	writeGaps(&reportBuilder, blackboard)
	return reportBuilder.String()
}

func writeVerdictSummary(reportBuilder *strings.Builder, blackboard *memory.Blackboard, options Options) {
	scenarios := blackboard.Scenarios()
	riskRegister := blackboard.RiskRegister()

	reportBuilder.WriteString("## Summary\n\n")

	coveredRiskIDs := map[string]bool{}
	for _, scenario := range scenarios {
		coveredRiskIDs[scenario.RiskRef] = true
	}
	prioritisedCount, coveredCount := 0, 0
	if riskRegister != nil {
		for _, prioritisedRisk := range riskRegister.Prioritised() {
			prioritisedCount++
			if coveredRiskIDs[prioritisedRisk.ID] {
				coveredCount++
			}
		}
	}

	if prioritisedCount > 0 {
		fmt.Fprintf(reportBuilder, "- **%d scenarios** covering **%d of %d** prioritised risks\n",
			len(scenarios), coveredCount, prioritisedCount)
	} else {
		// No critical or high risk is a finding about the codebase, not an
		// empty result, and the summary should not read like a failure.
		fmt.Fprintf(reportBuilder,
			"- **%d scenarios**. No risk scored critical or high; scenarios cover the highest-scoring components.\n",
			len(scenarios))
	}
	fmt.Fprintf(reportBuilder, "- Priority mix: %s\n", priorityMix(scenarios))

	if options.Validation != nil {
		blockingDefects := options.Validation.Blocking()
		if len(blockingDefects) == 0 {
			fmt.Fprintf(reportBuilder,
				"- Structural validation passed: every scenario has steps, a checkable expected "+
					"result, and a source reference that resolves in this commit\n")
		} else {
			fmt.Fprintf(reportBuilder, "- **%d structural defects remain** and are listed below\n",
				len(blockingDefects))
		}
	}

	// Low-confidence scenarios are surfaced first because that is where review
	// effort actually pays.
	lowConfidence := []model.TestScenario{}
	for _, scenario := range scenarios {
		if scenario.Confidence < 0.6 {
			lowConfidence = append(lowConfidence, scenario)
		}
	}
	if len(lowConfidence) > 0 {
		fmt.Fprintf(reportBuilder, "- **Review these first** — %d scenarios the system is least confident about: %s\n",
			len(lowConfidence), joinScenarioIDs(lowConfidence, 8))
	}
	if gapCount := len(blackboard.Gaps()); gapCount > 0 {
		fmt.Fprintf(reportBuilder, "- %d gaps recorded; see the end of this document\n", gapCount)
	}
	reportBuilder.WriteString("\n")

	if options.Validation != nil && len(options.Validation.Blocking()) > 0 {
		reportBuilder.WriteString("### Structural defects\n\n")
		for _, defect := range options.Validation.Blocking() {
			fmt.Fprintf(reportBuilder, "- %s\n", defect)
		}
		reportBuilder.WriteString("\n")
	}
}

func writeRunMetadata(reportBuilder *strings.Builder, blackboard *memory.Blackboard) {
	repoMap := blackboard.RepoMap()
	reportBuilder.WriteString("## Run metadata\n\n")
	fmt.Fprintf(reportBuilder, "- Run: `%s`\n", blackboard.RunID)
	if repoMap != nil {
		fmt.Fprintf(reportBuilder, "- Commit: `%s`\n", shortSHA(repoMap.CommitSHA))
		fmt.Fprintf(reportBuilder, "- Languages: %s\n", joinLanguages(repoMap.Languages))
		if repoMap.TestFrameworkName != "" {
			fmt.Fprintf(reportBuilder, "- Test framework: %s\n", repoMap.TestFrameworkName)
		}
	}

	// Analysis depth is reported honestly. A syntactic reading is not the same
	// thing as understanding, and a reviewer weighing a scenario deserves to
	// know which they are looking at.
	depthCounts := map[model.AnalysisDepth]int{}
	for _, componentModel := range blackboard.ComponentModels() {
		depthCounts[componentModel.Depth]++
	}
	if len(depthCounts) > 0 {
		depthParts := []string{}
		for _, depth := range []model.AnalysisDepth{
			model.DepthTypeResolved, model.DepthSyntactic, model.DepthInferred,
		} {
			if count := depthCounts[depth]; count > 0 {
				depthParts = append(depthParts, fmt.Sprintf("%d %s", count, depth))
			}
		}
		fmt.Fprintf(reportBuilder, "- Analysis depth: %s\n", strings.Join(depthParts, ", "))
	}
	if degradedTools := blackboard.DegradedTools(); len(degradedTools) > 0 {
		toolNames := sortedKeys(degradedTools)
		fmt.Fprintf(reportBuilder, "- Degraded tools: `%s`\n", strings.Join(toolNames, "`, `"))
	}
	reportBuilder.WriteString("\n")
}

func writeTestConsiderations(reportBuilder *strings.Builder, blackboard *memory.Blackboard, options Options) {
	riskRegister := blackboard.RiskRegister()
	if riskRegister == nil || len(riskRegister.Risks) == 0 {
		return
	}

	reportBuilder.WriteString("## Test considerations\n\n")
	reportBuilder.WriteString("### Risk register\n\n")
	reportBuilder.WriteString("| Risk | Component | Level | Score | Existing tests | Why |\n")
	reportBuilder.WriteString("|---|---|---|---|---|---|\n")
	for _, risk := range riskRegister.Risks {
		existingTestLabel := "none"
		if risk.HasExistingTest {
			existingTestLabel = "some"
		}
		fmt.Fprintf(reportBuilder, "| `%s` | %s | **%s** | %.1f | %s | %s |\n",
			risk.ID, risk.ComponentName, risk.Level, risk.Score, existingTestLabel, risk.Rationale)
	}
	reportBuilder.WriteString("\n")
	if riskRegister.ScoringNote != "" {
		fmt.Fprintf(reportBuilder, "_%s_\n\n", riskRegister.ScoringNote)
	}

	writeTestabilityObstacles(reportBuilder, blackboard)
}

// writeTestabilityObstacles lists what will make these tests hard to write,
// which is the part of a plan a QA lead actually schedules against.
func writeTestabilityObstacles(reportBuilder *strings.Builder, blackboard *memory.Blackboard) {
	dependenciesToStub := map[string]string{}
	sideEffectsByComponent := map[string][]string{}
	for _, componentModel := range blackboard.ComponentModels() {
		for _, externalCall := range componentModel.ExternalCalls {
			if externalCall.RequiresMock {
				reason := externalCall.Reason
				if reason == "" {
					reason = "external dependency"
				}
				dependenciesToStub[externalCall.Name] = reason
			}
		}
		if len(componentModel.SideEffects) > 0 {
			sideEffectsByComponent[componentModel.ComponentName] = componentModel.SideEffects
		}
	}
	if len(dependenciesToStub) == 0 && len(sideEffectsByComponent) == 0 {
		return
	}

	reportBuilder.WriteString("### Testability obstacles\n\n")
	if len(dependenciesToStub) > 0 {
		reportBuilder.WriteString("Dependencies a test must stub or cross:\n\n")
		for _, dependencyName := range sortedKeys(dependenciesToStub) {
			fmt.Fprintf(reportBuilder, "- `%s` — %s\n", dependencyName, dependenciesToStub[dependencyName])
		}
		reportBuilder.WriteString("\n")
	}
	if len(sideEffectsByComponent) > 0 {
		reportBuilder.WriteString("Side effects that need isolation:\n\n")
		for _, componentName := range sortedKeys(sideEffectsByComponent) {
			fmt.Fprintf(reportBuilder, "- **%s** — %s\n",
				componentName, strings.Join(sideEffectsByComponent[componentName], "; "))
		}
		reportBuilder.WriteString("\n")
	}
}

func writeScenarioCatalogue(reportBuilder *strings.Builder, blackboard *memory.Blackboard, options Options) {
	scenarios := blackboard.Scenarios()
	if len(scenarios) == 0 {
		return
	}

	reportBuilder.WriteString("## Scenario catalogue\n\n")

	scenariosByComponent := map[string][]model.TestScenario{}
	for _, scenario := range scenarios {
		scenariosByComponent[scenario.ComponentName] = append(scenariosByComponent[scenario.ComponentName], scenario)
	}

	for _, componentName := range sortedKeys(scenariosByComponent) {
		componentScenarios := scenariosByComponent[componentName]
		sort.SliceStable(componentScenarios, func(leftIndex, rightIndex int) bool {
			return componentScenarios[leftIndex].Priority < componentScenarios[rightIndex].Priority
		})

		fmt.Fprintf(reportBuilder, "### %s\n\n", componentName)
		for _, scenario := range componentScenarios {
			writeOneScenario(reportBuilder, scenario, options)
		}
	}
}

func writeOneScenario(reportBuilder *strings.Builder, scenario model.TestScenario, options Options) {
	fmt.Fprintf(reportBuilder, "#### `%s` — %s\n\n", scenario.ID, scenario.Title)
	fmt.Fprintf(reportBuilder, "**%s** · %s · covers `%s` · %s\n\n",
		scenario.Priority, scenario.Type, scenario.RiskRef, sourceLink(scenario.SourceRef, options))

	if len(scenario.Preconditions) > 0 {
		reportBuilder.WriteString("*Preconditions:*\n\n")
		for _, precondition := range scenario.Preconditions {
			fmt.Fprintf(reportBuilder, "- %s\n", precondition)
		}
		reportBuilder.WriteString("\n")
	}

	reportBuilder.WriteString("*Steps:*\n\n")
	for _, step := range scenario.Steps {
		fmt.Fprintf(reportBuilder, "%d. %s\n", step.Ordinal, step.Action)
	}
	reportBuilder.WriteString("\n")

	fmt.Fprintf(reportBuilder, "*Expected:* %s\n\n", scenario.ExpectedResult)

	if len(scenario.TestData) > 0 {
		dataDescriptions := make([]string, 0, len(scenario.TestData))
		for _, dataRequirement := range scenario.TestData {
			dataDescriptions = append(dataDescriptions, dataRequirement.Description)
		}
		fmt.Fprintf(reportBuilder, "*Test data:* %s\n\n", strings.Join(dataDescriptions, "; "))
	}
	if len(scenario.Mocks) > 0 {
		mockNames := make([]string, 0, len(scenario.Mocks))
		for _, mock := range scenario.Mocks {
			mockNames = append(mockNames, "`"+mock.Name+"`")
		}
		fmt.Fprintf(reportBuilder, "*Stub:* %s\n\n", strings.Join(mockNames, ", "))
	}
	if scenario.Notes != "" {
		fmt.Fprintf(reportBuilder, "*Note:* %s\n\n", scenario.Notes)
	}
	if scenario.Confidence < 0.6 {
		fmt.Fprintf(reportBuilder, "> Low confidence (%.1f) — worth a close read.\n\n", scenario.Confidence)
	}
}

func writeTraceability(reportBuilder *strings.Builder, blackboard *memory.Blackboard) {
	scenarios := blackboard.Scenarios()
	if len(scenarios) == 0 {
		return
	}
	reportBuilder.WriteString("## Traceability\n\n")
	reportBuilder.WriteString("| Scenario | Symbol | Risk | Priority |\n|---|---|---|---|\n")

	sortedScenarios := append([]model.TestScenario{}, scenarios...)
	sort.Slice(sortedScenarios, func(leftIndex, rightIndex int) bool {
		return sortedScenarios[leftIndex].ID < sortedScenarios[rightIndex].ID
	})
	for _, scenario := range sortedScenarios {
		symbolName := scenario.SourceRef.Symbol
		if symbolName == "" {
			symbolName = scenario.SourceRef.Path
		}
		fmt.Fprintf(reportBuilder, "| `%s` | `%s` | `%s` | %s |\n",
			scenario.ID, symbolName, scenario.RiskRef, scenario.Priority)
	}
	reportBuilder.WriteString("\n")
}

// writeOutstandingFindings shows what the critic raised and the author did not
// resolve. Hiding these would make the plan look finished when it is not.
func writeOutstandingFindings(reportBuilder *strings.Builder, blackboard *memory.Blackboard) {
	findings := blackboard.RevisionRequests()
	if len(findings) == 0 {
		return
	}
	reportBuilder.WriteString("## Outstanding review findings\n\n")
	reportBuilder.WriteString("The automated reviewer raised these and they were not resolved:\n\n")
	for _, finding := range findings {
		subject := finding.ScenarioID
		if subject == "" {
			subject = finding.RiskRef
		}
		fmt.Fprintf(reportBuilder, "- **%s**", finding.Severity)
		if subject != "" {
			fmt.Fprintf(reportBuilder, " `%s`", subject)
		}
		fmt.Fprintf(reportBuilder, " — %s", finding.Issue)
		if finding.Suggestion != "" {
			fmt.Fprintf(reportBuilder, " _(%s)_", finding.Suggestion)
		}
		reportBuilder.WriteString("\n")
	}
	reportBuilder.WriteString("\n")
}

func sourceLink(sourceRef model.SourceRef, options Options) string {
	label := sourceRef.Path
	if sourceRef.Symbol != "" {
		label = fmt.Sprintf("`%s` in %s", sourceRef.Symbol, sourceRef.Path)
	} else {
		label = "`" + label + "`"
	}
	if options.SourceLinkBase == "" || sourceRef.Path == "" {
		return label
	}
	linkTarget := fmt.Sprintf("%s/%s", strings.TrimSuffix(options.SourceLinkBase, "/"), sourceRef.Path)
	if sourceRef.StartLine > 0 {
		linkTarget += fmt.Sprintf("#L%d", sourceRef.StartLine)
		if sourceRef.EndLine > sourceRef.StartLine {
			linkTarget += fmt.Sprintf("-L%d", sourceRef.EndLine)
		}
	}
	return fmt.Sprintf("[%s](%s)", label, linkTarget)
}

func priorityMix(scenarios []model.TestScenario) string {
	counts := map[model.Priority]int{}
	for _, scenario := range scenarios {
		counts[scenario.Priority]++
	}
	parts := []string{}
	for _, priority := range []model.Priority{
		model.PriorityP0, model.PriorityP1, model.PriorityP2, model.PriorityP3,
	} {
		if count := counts[priority]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, priority))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func joinScenarioIDs(scenarios []model.TestScenario, maximumShown int) string {
	identifiers := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		identifiers = append(identifiers, "`"+scenario.ID+"`")
	}
	if len(identifiers) > maximumShown {
		return strings.Join(identifiers[:maximumShown], ", ") +
			fmt.Sprintf(" and %d more", len(identifiers)-maximumShown)
	}
	return strings.Join(identifiers, ", ")
}

func sortedKeys[ValueType any](valuesByKey map[string]ValueType) []string {
	keys := make([]string, 0, len(valuesByKey))
	for key := range valuesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
