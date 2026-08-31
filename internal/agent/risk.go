package agent

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

// RiskWeights are the scoring coefficients. They are named and exported so the
// scoring is arguable rather than mysterious: a reviewer who disagrees with the
// prioritisation can point at a number.
type RiskWeights struct {
	PerBranch           float64
	PerErrorPath        float64
	PerExternalCall     float64
	PerMockedDependency float64
	PerSideEffect       float64
	PerExportedSymbol   float64
	// UntestedMultiplier scales a component with no existing coverage. Untested
	// complexity is the whole point of the exercise, so it is a multiplier
	// rather than another additive term.
	UntestedMultiplier float64
	// ShallowAnalysisPenalty reduces the score when the analysis was syntactic
	// rather than type-resolved, so a guess does not outrank a fact.
	ShallowAnalysisPenalty float64
}

// DefaultRiskWeights is the shipped scoring.
func DefaultRiskWeights() RiskWeights {
	return RiskWeights{
		PerBranch: 1.0, PerErrorPath: 2.5, PerExternalCall: 2.0,
		PerMockedDependency: 1.5, PerSideEffect: 2.0, PerExportedSymbol: 0.5,
		UntestedMultiplier: 1.8, ShallowAnalysisPenalty: 0.75,
	}
}

// RiskThresholds map a score onto a level.
type RiskThresholds struct {
	Critical float64
	High     float64
	Medium   float64
}

// DefaultRiskThresholds is the shipped banding, used only for repositories too
// small for percentile banding to mean anything.
func DefaultRiskThresholds() RiskThresholds {
	return RiskThresholds{Critical: 30, High: 18, Medium: 8}
}

// RiskBands set how much of a repository lands in each level.
//
// Absolute thresholds were tuned on a three-file fixture, and on a real
// repository they called 42 of 58 components critical or high. Prioritisation
// that selects 72% of a codebase is not prioritisation — it is a list. Bands
// are therefore relative to this repository's own distribution: the score
// stays absolute and comparable between runs, while the level answers "how
// does this component rank against its neighbours", which is the question a
// reviewer deciding where to start is actually asking.
type RiskBands struct {
	CriticalFraction float64
	HighFraction     float64
	MediumFraction   float64
	// MinimumComponents is the size below which percentiles are meaningless
	// and the absolute thresholds are used instead.
	MinimumComponents int
}

// DefaultRiskBands puts the top tenth in critical and the next fifth in high.
func DefaultRiskBands() RiskBands {
	return RiskBands{
		CriticalFraction: 0.10, HighFraction: 0.20, MediumFraction: 0.35,
		MinimumComponents: 8,
	}
}

// RiskScorer builds the prioritised register.
//
// It uses no model at all, and that is a deliberate architectural choice rather
// than a shortcut. Prioritisation must be reproducible: if a re-run silently
// reshuffles which components are P0, the plan cannot be diffed and a reviewer
// cannot tell a real change from sampling noise.
type RiskScorer struct {
	Weights    RiskWeights
	Thresholds RiskThresholds
	Bands      RiskBands
}

// NewRiskScorer builds a scorer with the shipped defaults.
func NewRiskScorer() *RiskScorer {
	return &RiskScorer{
		Weights: DefaultRiskWeights(), Thresholds: DefaultRiskThresholds(),
		Bands: DefaultRiskBands(),
	}
}

func (riskScorer *RiskScorer) Name() string { return "risk" }

// Score builds the register from the blackboard and writes it back.
func (riskScorer *RiskScorer) Score(blackboard *memory.Blackboard) error {
	componentModels := blackboard.ComponentModels()
	if len(componentModels) == 0 {
		return fmt.Errorf("risk: no components have been analysed")
	}
	repoMap := blackboard.RepoMap()

	risks := make([]model.Risk, 0, len(componentModels))
	for _, componentModel := range componentModels {
		risks = append(risks, riskScorer.scoreComponent(componentModel, repoMap))
	}

	// Highest score first, then by ID so equal scores order deterministically.
	sort.Slice(risks, func(leftIndex, rightIndex int) bool {
		if risks[leftIndex].Score != risks[rightIndex].Score {
			return risks[leftIndex].Score > risks[rightIndex].Score
		}
		return risks[leftIndex].ID < risks[rightIndex].ID
	})
	riskScorer.assignLevels(risks)

	blackboard.SetRiskRegister(model.RiskRegister{
		Risks:    risks,
		ScoredAt: time.Now().UTC(),
		ScoringNote: fmt.Sprintf(
			"Deterministic scoring: branches ×%.1f, error paths ×%.1f, external calls ×%.1f, "+
				"side effects ×%.1f, exported symbols ×%.1f; ×%.1f when untested, ×%.2f when analysis was syntactic only. "+
				"Scores are absolute and comparable between runs; levels are relative to this repository "+
				"(top 10%% critical, next 20%% high, next 35%% medium).",
			riskScorer.Weights.PerBranch, riskScorer.Weights.PerErrorPath,
			riskScorer.Weights.PerExternalCall, riskScorer.Weights.PerSideEffect,
			riskScorer.Weights.PerExportedSymbol,
			riskScorer.Weights.UntestedMultiplier, riskScorer.Weights.ShallowAnalysisPenalty),
	})
	return nil
}

func (riskScorer *RiskScorer) scoreComponent(componentModel model.ComponentModel, repoMap *model.RepoMap) model.Risk {
	weights := riskScorer.Weights
	signals := []string{}
	score := 0.0

	if componentModel.ComplexityScore > 0 {
		score += float64(componentModel.ComplexityScore) * weights.PerBranch
		signals = append(signals, fmt.Sprintf("%d branches", componentModel.ComplexityScore))
	}
	if errorPathCount := len(componentModel.ErrorPaths); errorPathCount > 0 {
		score += float64(errorPathCount) * weights.PerErrorPath
		signals = append(signals, fmt.Sprintf("%d error paths", errorPathCount))
	}
	if externalCallCount := len(componentModel.ExternalCalls); externalCallCount > 0 {
		score += float64(externalCallCount) * weights.PerExternalCall
		signals = append(signals, fmt.Sprintf("%d external dependencies", externalCallCount))
	}
	mockedCount := 0
	for _, externalCall := range componentModel.ExternalCalls {
		if externalCall.RequiresMock {
			mockedCount++
		}
	}
	if mockedCount > 0 {
		score += float64(mockedCount) * weights.PerMockedDependency
		signals = append(signals, fmt.Sprintf("%d dependencies needing stubs", mockedCount))
	}
	if sideEffectCount := len(componentModel.SideEffects); sideEffectCount > 0 {
		score += float64(sideEffectCount) * weights.PerSideEffect
		signals = append(signals, fmt.Sprintf("%d side effects", sideEffectCount))
	}
	exportedCount := 0
	for _, symbol := range componentModel.PublicSymbols {
		if symbol.Exported {
			exportedCount++
		}
	}
	if exportedCount > 0 {
		score += float64(exportedCount) * weights.PerExportedSymbol
		signals = append(signals, fmt.Sprintf("%d exported symbols", exportedCount))
	}

	hasExistingTest := componentHasTest(componentModel, repoMap)
	if !hasExistingTest {
		score *= weights.UntestedMultiplier
		signals = append(signals, "no existing tests")
	} else {
		signals = append(signals, "some existing coverage")
	}
	if componentModel.Depth != model.DepthTypeResolved {
		score *= weights.ShallowAnalysisPenalty
		signals = append(signals, "analysis was "+string(componentModel.Depth))
	}

	return model.Risk{
		ID:            riskIDFor(componentModel),
		ComponentName: componentModel.ComponentName,
		Ref:           riskAnchorFor(componentModel),
		// Level is assigned once the whole distribution is known; see
		// assignLevels.
		Level:           model.RiskLow,
		Score:           roundToOneDecimal(score),
		Rationale:       rationaleFor(componentModel, hasExistingTest),
		Signals:         signals,
		HasExistingTest: hasExistingTest,
	}
}

// riskAnchorFor points the risk at the most complex exported symbol in the
// component rather than the file as a whole, so the scenario that follows has
// something specific to test.
func riskAnchorFor(componentModel model.ComponentModel) model.SourceRef {
	anchor := model.SourceRef{Path: componentModel.Path}
	highestBranchCount := -1
	for _, symbol := range componentModel.PublicSymbols {
		if !symbol.Exported {
			continue
		}
		if symbol.BranchCount > highestBranchCount {
			highestBranchCount = symbol.BranchCount
			anchor = symbol.Ref
		}
	}
	return anchor
}

func componentHasTest(componentModel model.ComponentModel, repoMap *model.RepoMap) bool {
	if repoMap == nil {
		return false
	}
	componentDirectory := directoryOf(componentModel.Path)
	for _, existingTest := range repoMap.ExistingTests {
		if directoryOf(existingTest.Path) == componentDirectory {
			return true
		}
	}
	return false
}

func directoryOf(filePath string) string {
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash < 0 {
		return "."
	}
	return filePath[:lastSlash]
}

func rationaleFor(componentModel model.ComponentModel, hasExistingTest bool) string {
	var rationaleBuilder strings.Builder
	fmt.Fprintf(&rationaleBuilder, "%s has %d branches across %d exported symbols",
		componentModel.ComponentName, componentModel.ComplexityScore, len(componentModel.PublicSymbols))
	if errorPathCount := len(componentModel.ErrorPaths); errorPathCount > 0 {
		fmt.Fprintf(&rationaleBuilder, " and %d distinct failure paths", errorPathCount)
	}
	if !hasExistingTest {
		rationaleBuilder.WriteString(", with no tests in its package")
	}
	rationaleBuilder.WriteString(".")
	return rationaleBuilder.String()
}

// assignLevels bands an already-sorted register by rank.
//
// Ties are honoured: two components with the same score always get the same
// level, so a band boundary never splits equal risks arbitrarily.
func (riskScorer *RiskScorer) assignLevels(risks []model.Risk) {
	bands := riskScorer.Bands
	if bands.MinimumComponents <= 0 {
		bands = DefaultRiskBands()
	}
	if len(risks) < bands.MinimumComponents {
		// Too few components for a percentile to mean anything. Fall back to
		// the absolute thresholds rather than calling the top file of four
		// "critical" purely because something has to be.
		for index := range risks {
			risks[index].Level = riskScorer.levelFor(risks[index].Score)
		}
		return
	}

	criticalCount := atLeastOne(float64(len(risks)) * bands.CriticalFraction)
	highCount := criticalCount + atLeastOne(float64(len(risks))*bands.HighFraction)
	mediumCount := highCount + atLeastOne(float64(len(risks))*bands.MediumFraction)

	levelForRank := func(rank int) model.RiskLevel {
		switch {
		case rank < criticalCount:
			return model.RiskCritical
		case rank < highCount:
			return model.RiskHigh
		case rank < mediumCount:
			return model.RiskMedium
		default:
			return model.RiskLow
		}
	}

	for index := range risks {
		level := levelForRank(index)
		// Equal scores must not straddle a boundary: promote to whatever the
		// first component with this score received.
		if index > 0 && risks[index].Score == risks[index-1].Score {
			level = risks[index-1].Level
		}
		risks[index].Level = level
	}
}

func atLeastOne(value float64) int {
	rounded := int(value + 0.5)
	if rounded < 1 {
		return 1
	}
	return rounded
}

func (riskScorer *RiskScorer) levelFor(score float64) model.RiskLevel {
	switch {
	case score >= riskScorer.Thresholds.Critical:
		return model.RiskCritical
	case score >= riskScorer.Thresholds.High:
		return model.RiskHigh
	case score >= riskScorer.Thresholds.Medium:
		return model.RiskMedium
	default:
		return model.RiskLow
	}
}

func riskIDFor(componentModel model.ComponentModel) string {
	slug := strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(componentModel.ComponentName)
	return "RISK-" + strings.ToLower(slug)
}

func roundToOneDecimal(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}
