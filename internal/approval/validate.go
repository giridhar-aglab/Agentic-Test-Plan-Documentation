// Package approval holds the gate between a generated plan and a human, and
// the guarded write path beyond it.
package approval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

// Defect is one structural problem found by validation.
type Defect struct {
	ScenarioID string `json:"scenarioId,omitempty"`
	RiskRef    string `json:"riskRef,omitempty"`
	Rule       string `json:"rule"`
	Detail     string `json:"detail"`
	// Blocking defects must be repaired before a reviewer sees the report.
	Blocking bool `json:"blocking"`
}

func (defect Defect) String() string {
	subject := defect.ScenarioID
	if subject == "" {
		subject = defect.RiskRef
	}
	return fmt.Sprintf("[%s] %s: %s", defect.Rule, subject, defect.Detail)
}

// Report is the outcome of validation.
type Report struct {
	Defects []Defect `json:"defects"`
}

// Blocking returns only the defects that must be fixed.
func (report Report) Blocking() []Defect {
	blocking := []Defect{}
	for _, defect := range report.Defects {
		if defect.Blocking {
			blocking = append(blocking, defect)
		}
	}
	return blocking
}

// OK reports whether the catalogue may be shown to a reviewer.
func (report Report) OK() bool { return len(report.Blocking()) == 0 }

// Validate asserts everything about a plan that does not require judgement.
//
// The point is to spend a reviewer's attention on the questions only a person
// can answer. Catching a hallucinated file path or a scenario with no expected
// result is not one of those questions, and a human who finds one loses trust
// in the whole document — so these are caught by machine, repaired by
// re-entering the Author phase, and never surfaced.
func Validate(blackboard *memory.Blackboard) Report {
	report := Report{Defects: []Defect{}}
	scenarios := blackboard.Scenarios()
	repoMap := blackboard.RepoMap()
	riskRegister := blackboard.RiskRegister()

	knownSymbolsByPath := buildSymbolIndex(blackboard.ComponentModels())
	knownPaths := map[string]bool{}
	if repoMap != nil {
		for _, sourceFile := range repoMap.Files {
			knownPaths[sourceFile.Path] = true
		}
	}

	seenIDs := map[string]bool{}
	for _, scenario := range scenarios {
		report.Defects = append(report.Defects,
			validateOneScenario(scenario, knownPaths, knownSymbolsByPath, seenIDs)...)
		seenIDs[scenario.ID] = true
	}

	report.Defects = append(report.Defects, findNearDuplicates(scenarios)...)
	report.Defects = append(report.Defects, findUncoveredRisks(riskRegister, scenarios)...)

	sort.SliceStable(report.Defects, func(leftIndex, rightIndex int) bool {
		return report.Defects[leftIndex].Blocking && !report.Defects[rightIndex].Blocking
	})
	return report
}

func validateOneScenario(
	scenario model.TestScenario,
	knownPaths map[string]bool,
	knownSymbolsByPath map[string]map[string]bool,
	seenIDs map[string]bool,
) []Defect {
	defects := []Defect{}

	if seenIDs[scenario.ID] {
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "duplicate-id",
			Detail: "two scenarios share this ID; IDs must be unique for the Jira ledger to work", Blocking: true})
	}
	if strings.TrimSpace(scenario.Title) == "" {
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "missing-title",
			Detail: "a scenario without a title cannot be reviewed", Blocking: true})
	}
	if len(scenario.Steps) == 0 {
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "missing-steps",
			Detail: "a scenario with no steps is not executable", Blocking: true})
	}
	if strings.TrimSpace(scenario.ExpectedResult) == "" {
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "missing-expected-result",
			Detail: "a scenario with no expected result cannot pass or fail", Blocking: true})
	} else if isVagueAssertion(scenario.ExpectedResult) {
		// Not blocking: judging whether an assertion is specific enough is a
		// human call. Flagging it puts it in front of the reviewer instead.
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "vague-expected-result",
			Detail: fmt.Sprintf("%q does not name something a person could check", scenario.ExpectedResult)})
	}

	// The traceability anchor is the claim most likely to be hallucinated, and
	// the cheapest to verify.
	if scenario.SourceRef.Path == "" {
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "missing-source-ref",
			Detail: "a scenario must name the code it tests", Blocking: true})
	} else if len(knownPaths) > 0 && !knownPaths[scenario.SourceRef.Path] {
		defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "unresolvable-path",
			Detail: fmt.Sprintf("%q is not a file in the analysed commit", scenario.SourceRef.Path), Blocking: true})
	} else if scenario.SourceRef.Symbol != "" {
		if symbolsInFile, known := knownSymbolsByPath[scenario.SourceRef.Path]; known && len(symbolsInFile) > 0 {
			if !symbolsInFile[bareSymbolName(scenario.SourceRef.Symbol)] {
				defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "unresolvable-symbol",
					Detail: fmt.Sprintf("%q was not found in %s",
						scenario.SourceRef.Symbol, scenario.SourceRef.Path), Blocking: true})
			}
		}
	}
	return defects
}

// vagueAssertions are phrasings that look like an expected result but give a
// reviewer nothing to check.
var vagueAssertions = []string{
	"works correctly", "works as expected", "behaves correctly", "behaves as expected",
	"is correct", "succeeds as expected", "no issues", "functions properly",
	"the test passes", "everything works",
}

func isVagueAssertion(expectedResult string) bool {
	lowered := strings.ToLower(expectedResult)
	for _, vaguePhrase := range vagueAssertions {
		if strings.Contains(lowered, vaguePhrase) {
			return true
		}
	}
	return false
}

func bareSymbolName(qualifiedSymbol string) string {
	if lastDot := strings.LastIndex(qualifiedSymbol, "."); lastDot >= 0 {
		return qualifiedSymbol[lastDot+1:]
	}
	return qualifiedSymbol
}

func buildSymbolIndex(componentModels []model.ComponentModel) map[string]map[string]bool {
	symbolIndex := map[string]map[string]bool{}
	for _, componentModel := range componentModels {
		if symbolIndex[componentModel.Path] == nil {
			symbolIndex[componentModel.Path] = map[string]bool{}
		}
		for _, symbol := range componentModel.PublicSymbols {
			symbolIndex[componentModel.Path][symbol.Name] = true
		}
	}
	return symbolIndex
}

// findNearDuplicates flags scenarios that test the same thing.
//
// The comparison is on the normalised title plus the expected result, which
// catches the common failure — the same assertion reworded — without the false
// positives a fuzzier match would produce. It is non-blocking: two similar
// scenarios may both be worth keeping, and that is a reviewer's call.
func findNearDuplicates(scenarios []model.TestScenario) []Defect {
	defects := []Defect{}
	seenBySignature := map[string]string{}
	for _, scenario := range scenarios {
		signature := normaliseForComparison(scenario.Title) + "|" + normaliseForComparison(scenario.ExpectedResult)
		if firstScenarioID, alreadySeen := seenBySignature[signature]; alreadySeen {
			defects = append(defects, Defect{ScenarioID: scenario.ID, Rule: "near-duplicate",
				Detail: fmt.Sprintf("this appears to test the same thing as %s", firstScenarioID)})
			continue
		}
		seenBySignature[signature] = scenario.ID
	}
	return defects
}

var comparisonNoise = strings.NewReplacer(
	",", " ", ".", " ", "\"", " ", "'", " ", "-", " ", "_", " ", "(", " ", ")", " ",
)

func normaliseForComparison(text string) string {
	lowered := strings.ToLower(comparisonNoise.Replace(text))
	return strings.Join(strings.Fields(lowered), " ")
}

// findUncoveredRisks is the coverage check: a prioritised risk with no scenario
// is the single most consequential thing validation can catch, because it is
// invisible in a document that otherwise looks complete.
func findUncoveredRisks(riskRegister *model.RiskRegister, scenarios []model.TestScenario) []Defect {
	if riskRegister == nil {
		return nil
	}
	coveredRiskIDs := map[string]bool{}
	for _, scenario := range scenarios {
		coveredRiskIDs[scenario.RiskRef] = true
	}

	defects := []Defect{}
	for _, prioritisedRisk := range riskRegister.Prioritised() {
		if coveredRiskIDs[prioritisedRisk.ID] {
			continue
		}
		defects = append(defects, Defect{
			RiskRef: prioritisedRisk.ID, Rule: "uncovered-risk",
			Detail: fmt.Sprintf("%s risk %s (%s) has no scenario",
				prioritisedRisk.Level, prioritisedRisk.ID, prioritisedRisk.ComponentName),
			Blocking: true,
		})
	}
	return defects
}
