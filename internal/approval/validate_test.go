package approval

import (
	"strings"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

func blackboardWith(scenarios ...model.TestScenario) *memory.Blackboard {
	blackboard := memory.NewBlackboard("validate-test")
	blackboard.SetRepoMap(model.RepoMap{
		RepositoryName: "paymentsvc",
		Files: []model.SourceFile{
			{Path: "internal/ledger/ledger.go", Language: model.LanguageGo, SelectedForScan: true},
		},
	})
	blackboard.AddComponentModel(model.ComponentModel{
		ComponentName: "ledger/ledger",
		Path:          "internal/ledger/ledger.go",
		PublicSymbols: []model.Symbol{
			{Name: "Post", Exported: true}, {Name: "Balance", Exported: true},
		},
	})
	blackboard.AddScenarios(scenarios...)
	return blackboard
}

func goodScenario() model.TestScenario {
	return model.TestScenario{
		ID:             "TS-ledger-001",
		Title:          "Post rejects a debit that exceeds the balance",
		ComponentName:  "ledger/ledger",
		SourceRef:      model.SourceRef{Path: "internal/ledger/ledger.go", Symbol: "Post"},
		Type:           model.ScenarioNegative,
		Priority:       model.PriorityP0,
		RiskRef:        "RISK-ledger",
		Steps:          []model.Step{{Ordinal: 1, Action: "Post a debit of 200 against a balance of 100"}},
		ExpectedResult: "Post returns ErrInsufficientFunds and the balance is unchanged at 100",
	}
}

func rulesIn(report Report) map[string]bool {
	rules := map[string]bool{}
	for _, defect := range report.Defects {
		rules[defect.Rule] = true
	}
	return rules
}

func TestValidateAcceptsAWellFormedScenario(t *testing.T) {
	report := Validate(blackboardWith(goodScenario()))
	if !report.OK() {
		t.Fatalf("a well-formed scenario must pass, got %v", report.Defects)
	}
}

func TestValidateBlocksAnUnresolvablePath(t *testing.T) {
	// The claim most likely to be hallucinated, and the cheapest to check.
	scenario := goodScenario()
	scenario.SourceRef.Path = "internal/ledger/does_not_exist.go"

	report := Validate(blackboardWith(scenario))
	if report.OK() {
		t.Fatal("a path that is not in the commit must block")
	}
	if !rulesIn(report)["unresolvable-path"] {
		t.Fatalf("expected an unresolvable-path defect, got %v", report.Defects)
	}
}

func TestValidateBlocksAnUnresolvableSymbol(t *testing.T) {
	scenario := goodScenario()
	scenario.SourceRef.Symbol = "PostTransaction" // close, but not a real symbol

	report := Validate(blackboardWith(scenario))
	if !rulesIn(report)["unresolvable-symbol"] {
		t.Fatalf("expected an unresolvable-symbol defect, got %v", report.Defects)
	}
}

func TestValidateAcceptsAQualifiedMethodName(t *testing.T) {
	// Account.Balance should resolve against the bare symbol Balance.
	scenario := goodScenario()
	scenario.SourceRef.Symbol = "Account.Balance"

	if report := Validate(blackboardWith(scenario)); !report.OK() {
		t.Fatalf("a receiver-qualified symbol must resolve, got %v", report.Defects)
	}
}

func TestValidateBlocksMissingStepsAndExpectedResult(t *testing.T) {
	withoutSteps := goodScenario()
	withoutSteps.Steps = nil
	withoutExpected := goodScenario()
	withoutExpected.ID = "TS-ledger-002"
	withoutExpected.ExpectedResult = ""

	report := Validate(blackboardWith(withoutSteps, withoutExpected))
	rules := rulesIn(report)
	if !rules["missing-steps"] || !rules["missing-expected-result"] {
		t.Fatalf("both defects should be reported, got %v", report.Defects)
	}
	if report.OK() {
		t.Fatal("an unexecutable scenario must block")
	}
}

func TestValidateFlagsVagueAssertionsWithoutBlocking(t *testing.T) {
	// Whether an assertion is specific enough is a human call, so this is
	// surfaced to the reviewer rather than bounced back to the author.
	scenario := goodScenario()
	scenario.ExpectedResult = "The function works correctly"

	report := Validate(blackboardWith(scenario))
	if !rulesIn(report)["vague-expected-result"] {
		t.Fatalf("expected a vague-expected-result finding, got %v", report.Defects)
	}
	if !report.OK() {
		t.Fatal("a vague assertion should be flagged for a human, not treated as blocking")
	}
}

func TestValidateFindsNearDuplicates(t *testing.T) {
	first := goodScenario()
	second := goodScenario()
	second.ID = "TS-ledger-002"
	second.Title = "post rejects a debit that exceeds the balance." // reworded punctuation only

	report := Validate(blackboardWith(first, second))
	if !rulesIn(report)["near-duplicate"] {
		t.Fatalf("expected a near-duplicate finding, got %v", report.Defects)
	}
}

func TestValidateBlocksDuplicateIDs(t *testing.T) {
	// Duplicate IDs would break the Jira ledger, which is what stops a re-run
	// creating duplicate tickets.
	first := goodScenario()
	second := goodScenario()
	second.Title = "A different scenario that reuses an ID"
	second.ExpectedResult = "Something else entirely happens"

	blackboard := memory.NewBlackboard("dupe-id")
	blackboard.SetRepoMap(model.RepoMap{Files: []model.SourceFile{
		{Path: "internal/ledger/ledger.go"},
	}})
	// AddScenarios de-duplicates by ID, so the collision is constructed
	// directly to exercise the validator rather than the blackboard.
	report := Report{}
	seenIDs := map[string]bool{first.ID: true}
	report.Defects = validateOneScenario(second,
		map[string]bool{"internal/ledger/ledger.go": true}, nil, seenIDs)

	if !rulesIn(report)["duplicate-id"] {
		t.Fatalf("expected a duplicate-id defect, got %v", report.Defects)
	}
}

func TestValidateBlocksAnUncoveredPrioritisedRisk(t *testing.T) {
	// The most consequential thing validation catches: a gap that is invisible
	// in a document which otherwise looks complete.
	blackboard := blackboardWith(goodScenario())
	blackboard.SetRiskRegister(model.RiskRegister{Risks: []model.Risk{
		{ID: "RISK-ledger", ComponentName: "ledger/ledger", Level: model.RiskCritical},
		{ID: "RISK-gateway", ComponentName: "gateway/gateway", Level: model.RiskHigh},
	}})

	report := Validate(blackboard)
	if report.OK() {
		t.Fatal("a prioritised risk with no scenario must block")
	}
	foundUncovered := false
	for _, defect := range report.Defects {
		if defect.Rule == "uncovered-risk" && defect.RiskRef == "RISK-gateway" {
			foundUncovered = true
		}
	}
	if !foundUncovered {
		t.Fatalf("expected RISK-gateway to be reported uncovered, got %v", report.Defects)
	}
}

func TestValidateIgnoresLowAndMediumRiskCoverage(t *testing.T) {
	// Only prioritised risks are required to have scenarios; demanding coverage
	// of every low risk would make the check impossible to satisfy.
	blackboard := blackboardWith(goodScenario())
	blackboard.SetRiskRegister(model.RiskRegister{Risks: []model.Risk{
		{ID: "RISK-ledger", ComponentName: "ledger/ledger", Level: model.RiskCritical},
		{ID: "RISK-main", ComponentName: "paysvc/main", Level: model.RiskLow},
	}})

	if report := Validate(blackboard); !report.OK() {
		t.Fatalf("a low risk without a scenario must not block, got %v", report.Defects)
	}
}

func TestDefectStringNamesItsSubject(t *testing.T) {
	defect := Defect{ScenarioID: "TS-1", Rule: "missing-steps", Detail: "no steps"}
	if !strings.Contains(defect.String(), "TS-1") || !strings.Contains(defect.String(), "missing-steps") {
		t.Fatalf("a defect must identify itself, got %q", defect.String())
	}
}
