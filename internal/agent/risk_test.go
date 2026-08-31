package agent

import (
	"fmt"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

func componentWith(name, path string, complexity int, errorPaths []string, externalCalls int) model.ComponentModel {
	dependencies := make([]model.Dependency, 0, externalCalls)
	for callIndex := 0; callIndex < externalCalls; callIndex++ {
		dependencies = append(dependencies, model.Dependency{
			Name: "dep" + string(rune('a'+callIndex)), RequiresMock: true,
		})
	}
	return model.ComponentModel{
		ComponentName:   name,
		Path:            path,
		Depth:           model.DepthTypeResolved,
		ComplexityScore: complexity,
		ErrorPaths:      errorPaths,
		ExternalCalls:   dependencies,
		PublicSymbols: []model.Symbol{
			{Name: "DoWork", Exported: true, BranchCount: complexity,
				Ref: model.SourceRef{Path: path, Symbol: "DoWork", StartLine: 10, EndLine: 40}},
			{Name: "Simple", Exported: true, BranchCount: 1,
				Ref: model.SourceRef{Path: path, Symbol: "Simple", StartLine: 45, EndLine: 47}},
		},
	}
}

func scoreFixture(t *testing.T, existingTests []model.ExistingTest, components ...model.ComponentModel) model.RiskRegister {
	t.Helper()
	blackboard := memory.NewBlackboard("risk-test")
	blackboard.SetRepoMap(model.RepoMap{ExistingTests: existingTests})
	for _, componentModel := range components {
		blackboard.AddComponentModel(componentModel)
	}
	if err := NewRiskScorer().Score(blackboard); err != nil {
		t.Fatalf("scoring failed: %v", err)
	}
	riskRegister := blackboard.RiskRegister()
	if riskRegister == nil {
		t.Fatal("no risk register was produced")
	}
	return *riskRegister
}

func TestRiskScoringIsDeterministic(t *testing.T) {
	// If a re-run silently reshuffled priorities, the plan could not be diffed
	// and a reviewer could not tell a real change from noise. This is the whole
	// reason the scorer uses no model.
	components := []model.ComponentModel{
		componentWith("a/one", "a/one.go", 12, []string{"x", "y"}, 1),
		componentWith("b/two", "b/two.go", 3, nil, 0),
		componentWith("c/three", "c/three.go", 7, []string{"z"}, 2),
	}
	firstRegister := scoreFixture(t, nil, components...)
	secondRegister := scoreFixture(t, nil, components...)

	if len(firstRegister.Risks) != len(secondRegister.Risks) {
		t.Fatal("the register changed size between identical runs")
	}
	for riskIndex := range firstRegister.Risks {
		if firstRegister.Risks[riskIndex].ID != secondRegister.Risks[riskIndex].ID ||
			firstRegister.Risks[riskIndex].Score != secondRegister.Risks[riskIndex].Score {
			t.Fatalf("risk %d differed between identical runs: %+v vs %+v",
				riskIndex, firstRegister.Risks[riskIndex], secondRegister.Risks[riskIndex])
		}
	}
}

func TestRiskRegisterIsOrderedByScore(t *testing.T) {
	riskRegister := scoreFixture(t, nil,
		componentWith("b/simple", "b/simple.go", 2, nil, 0),
		componentWith("a/complex", "a/complex.go", 20, []string{"x", "y", "z"}, 3),
	)
	if riskRegister.Risks[0].ComponentName != "a/complex" {
		t.Fatalf("the riskiest component must sort first, got %s", riskRegister.Risks[0].ComponentName)
	}
	if riskRegister.Risks[0].Score <= riskRegister.Risks[1].Score {
		t.Fatal("scores must strictly order the register")
	}
}

func TestUntestedComponentOutranksAnEquivalentTestedOne(t *testing.T) {
	// Untested complexity is the entire point of the exercise, so it is a
	// multiplier rather than one more additive signal.
	testedRegister := scoreFixture(t,
		[]model.ExistingTest{{Path: "a/one_test.go"}},
		componentWith("a/one", "a/one.go", 10, []string{"x"}, 1))
	untestedRegister := scoreFixture(t, nil,
		componentWith("a/one", "a/one.go", 10, []string{"x"}, 1))

	if untestedRegister.Risks[0].Score <= testedRegister.Risks[0].Score {
		t.Fatalf("an untested component must outrank an identical tested one, got %.1f vs %.1f",
			untestedRegister.Risks[0].Score, testedRegister.Risks[0].Score)
	}
	if testedRegister.Risks[0].HasExistingTest != true {
		t.Error("existing coverage must be recorded on the risk")
	}
}

func TestSyntacticAnalysisIsPenalised(t *testing.T) {
	// A guess must not outrank a fact.
	resolvedComponent := componentWith("a/one", "a/one.go", 10, []string{"x"}, 1)
	syntacticComponent := componentWith("a/one", "a/one.go", 10, []string{"x"}, 1)
	syntacticComponent.Depth = model.DepthSyntactic

	resolvedRegister := scoreFixture(t, nil, resolvedComponent)
	syntacticRegister := scoreFixture(t, nil, syntacticComponent)

	if syntacticRegister.Risks[0].Score >= resolvedRegister.Risks[0].Score {
		t.Fatalf("a shallow analysis must score below an equivalent deep one, got %.1f vs %.1f",
			syntacticRegister.Risks[0].Score, resolvedRegister.Risks[0].Score)
	}
}

func TestRiskAnchorsOnTheMostComplexExportedSymbol(t *testing.T) {
	// Anchoring on the file would give the Author nothing specific to test.
	riskRegister := scoreFixture(t, nil, componentWith("a/one", "a/one.go", 15, []string{"x"}, 0))
	anchor := riskRegister.Risks[0].Ref
	if anchor.Symbol != "DoWork" {
		t.Fatalf("expected the branchiest exported symbol as the anchor, got %q", anchor.Symbol)
	}
	if anchor.StartLine == 0 {
		t.Error("the anchor needs a line range for the source link to be useful")
	}
}

func TestPrioritisedReturnsOnlyCriticalAndHigh(t *testing.T) {
	riskRegister := model.RiskRegister{Risks: []model.Risk{
		{ID: "a", Level: model.RiskCritical},
		{ID: "b", Level: model.RiskHigh},
		{ID: "c", Level: model.RiskMedium},
		{ID: "d", Level: model.RiskLow},
	}}
	prioritised := riskRegister.Prioritised()
	if len(prioritised) != 2 {
		t.Fatalf("expected 2 prioritised risks, got %d", len(prioritised))
	}
}

func TestPriorityMapsFromRiskLevel(t *testing.T) {
	expectations := map[model.RiskLevel]model.Priority{
		model.RiskCritical: model.PriorityP0,
		model.RiskHigh:     model.PriorityP1,
		model.RiskMedium:   model.PriorityP2,
		model.RiskLow:      model.PriorityP3,
	}
	for riskLevel, expectedPriority := range expectations {
		if got := model.PriorityForRisk(riskLevel); got != expectedPriority {
			t.Errorf("%s should map to %s, got %s", riskLevel, expectedPriority, got)
		}
	}
}

func TestScoringFailsWhenNothingWasAnalysed(t *testing.T) {
	blackboard := memory.NewBlackboard("empty")
	if err := NewRiskScorer().Score(blackboard); err == nil {
		t.Fatal("scoring an empty analysis must fail rather than produce an empty register")
	}
}

func TestScoringNoteExplainsItself(t *testing.T) {
	// A reviewer who disagrees with the prioritisation should be able to point
	// at a number rather than argue with a black box.
	riskRegister := scoreFixture(t, nil, componentWith("a/one", "a/one.go", 5, nil, 0))
	if riskRegister.ScoringNote == "" {
		t.Fatal("the register must explain how it was scored")
	}
}

func TestPrioritisationSelectsAMinorityOfARealRepository(t *testing.T) {
	// Absolute thresholds tuned on a three-file fixture called 42 of 58 real
	// components critical or high. A register that prioritises 72% of a
	// codebase tells a reviewer nothing about where to start, and it made the
	// author phase forty-two strong-tier agents instead of a handful.
	componentModels := make([]model.ComponentModel, 0, 58)
	for index := range 58 {
		componentModels = append(componentModels, model.ComponentModel{
			ComponentName: fmt.Sprintf("pkg/component%02d", index),
			Path:          fmt.Sprintf("pkg/component%02d.go", index),
			Language:      model.LanguageGo,
			Depth:         model.DepthTypeResolved,
			// A spread of complexity, the way a real repository has.
			PublicSymbols: makeSymbols(index%9 + 1),
			ErrorPaths:    makeStrings("fails", index%5),
		})
	}

	blackboard := memory.NewBlackboard("run")
	blackboard.SetRepoMap(model.RepoMap{})
	for _, componentModel := range componentModels {
		blackboard.AddComponentModel(componentModel)
	}
	if err := NewRiskScorer().Score(blackboard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	register := blackboard.RiskRegister()
	prioritised := register.Prioritised()
	fraction := float64(len(prioritised)) / float64(len(register.Risks))
	if fraction > 0.40 {
		t.Errorf("prioritisation selected %d of %d components (%.0f%%); that is a list, not a ranking",
			len(prioritised), len(register.Risks), fraction*100)
	}
	if len(prioritised) == 0 {
		t.Error("something must always be prioritised, or there is nothing to author")
	}
}

func TestEqualScoresNeverStraddleABandBoundary(t *testing.T) {
	// A boundary that splits identical scores would make the level arbitrary,
	// and two identical components getting different priorities is the kind of
	// thing that destroys trust in a generated plan.
	blackboard := memory.NewBlackboard("run")
	blackboard.SetRepoMap(model.RepoMap{})
	for index := range 20 {
		blackboard.AddComponentModel(model.ComponentModel{
			ComponentName: fmt.Sprintf("pkg/same%02d", index),
			Path:          fmt.Sprintf("pkg/same%02d.go", index),
			Language:      model.LanguageGo, Depth: model.DepthTypeResolved,
			PublicSymbols: makeSymbols(3), ErrorPaths: makeStrings("fails", 2),
		})
	}
	if err := NewRiskScorer().Score(blackboard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	risks := blackboard.RiskRegister().Risks
	for index := 1; index < len(risks); index++ {
		if risks[index].Score == risks[index-1].Score && risks[index].Level != risks[index-1].Level {
			t.Fatalf("identical scores %.1f got different levels: %s vs %s",
				risks[index].Score, risks[index-1].Level, risks[index].Level)
		}
	}
}

func TestASmallRepositoryDoesNotGetAnInventedCriticalRisk(t *testing.T) {
	// Percentiles are meaningless on four files: the top one would be
	// "critical" purely because something has to be. Below the minimum, the
	// absolute thresholds decide.
	blackboard := memory.NewBlackboard("run")
	blackboard.SetRepoMap(model.RepoMap{})
	for index := range 4 {
		blackboard.AddComponentModel(model.ComponentModel{
			ComponentName: fmt.Sprintf("pkg/tiny%d", index),
			Path:          fmt.Sprintf("pkg/tiny%d.go", index),
			Language:      model.LanguageGo, Depth: model.DepthTypeResolved,
			PublicSymbols: makeSymbols(1),
		})
	}
	if err := NewRiskScorer().Score(blackboard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, risk := range blackboard.RiskRegister().Risks {
		if risk.Level == model.RiskCritical && risk.Score < DefaultRiskThresholds().Critical {
			t.Errorf("%s scored %.1f but was called critical on a four-file repository",
				risk.ID, risk.Score)
		}
	}
}

func makeSymbols(count int) []model.Symbol {
	symbols := make([]model.Symbol, 0, count)
	for index := range count {
		symbols = append(symbols, model.Symbol{
			Name: fmt.Sprintf("Exported%d", index), Exported: true, BranchCount: index + 1,
		})
	}
	return symbols
}

func makeStrings(prefix string, count int) []string {
	values := make([]string, 0, count)
	for index := range count {
		values = append(values, fmt.Sprintf("%s %d", prefix, index))
	}
	return values
}
