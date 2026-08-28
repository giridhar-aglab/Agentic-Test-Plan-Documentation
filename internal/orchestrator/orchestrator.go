// Package orchestrator owns the run. It advances phases, enforces budgets and
// decides termination. It never touches repository tools itself.
package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/memory"
)

// PhaseFunc is the work of one phase. It receives the blackboard and writes its
// findings there; it returns an error only for genuine faults, because a guard
// stop is an orderly end that the orchestrator handles itself.
type PhaseFunc func(ctx context.Context, blackboard *memory.Blackboard) error

// Phase is one node in the pipeline DAG.
type Phase struct {
	Name      string
	DependsOn []string
	Run       PhaseFunc
	// Postcondition is the explicit termination condition. A phase is complete
	// when this returns nil — not when a model stopped asking for tools. Leaving
	// it nil means the phase is complete as soon as Run returns.
	Postcondition func(blackboard *memory.Blackboard) error
	// Optional phases record a gap and let the run continue when they fail.
	Optional bool
	Budget   guard.Budget
}

// PhaseResult records what happened in one phase.
type PhaseResult struct {
	Name       string
	Started    time.Time
	Duration   time.Duration
	Err        error
	Skipped    bool
	SkipReason string
}

// Succeeded reports whether the phase ran and met its postcondition.
func (phaseResult PhaseResult) Succeeded() bool {
	return phaseResult.Err == nil && !phaseResult.Skipped
}

// CheckpointWriter persists a blackboard snapshot after each phase.
type CheckpointWriter interface {
	Write(runID, phaseName string, snapshot []byte) error
}

// Orchestrator executes phases in dependency order.
type Orchestrator struct {
	Blackboard *memory.Blackboard
	Checkpoint CheckpointWriter
	// RunBudget bounds the whole run, independently of per-phase budgets.
	RunBudget guard.Budget
	// Logf receives progress lines. Nil means silent.
	Logf func(format string, arguments ...any)

	phases  []Phase
	nowFunc func() time.Time
}

// New builds an orchestrator over a blackboard.
func New(blackboard *memory.Blackboard) *Orchestrator {
	return &Orchestrator{Blackboard: blackboard, nowFunc: time.Now}
}

// AddPhase registers a phase.
func (orchestrator *Orchestrator) AddPhase(phase Phase) {
	orchestrator.phases = append(orchestrator.phases, phase)
}

func (orchestrator *Orchestrator) logf(format string, arguments ...any) {
	if orchestrator.Logf != nil {
		orchestrator.Logf(format, arguments...)
	}
}

// RunReport summarises a whole run.
type RunReport struct {
	RunID         string
	PhaseResults  []PhaseResult
	TotalDuration time.Duration
	Aborted       bool
	AbortReason   string
}

// Run executes every phase in topological order.
//
// Failure of a required phase aborts the run but still returns everything
// gathered so far: a partial plan with an honest gaps section beats nothing.
func (orchestrator *Orchestrator) Run(ctx context.Context) (RunReport, error) {
	orderedPhases, err := topologicalOrder(orchestrator.phases)
	if err != nil {
		return RunReport{}, err
	}

	runStartedAt := orchestrator.nowFunc()
	runReport := RunReport{RunID: orchestrator.Blackboard.RunID}

	if orchestrator.RunBudget.MaxWallClock > 0 {
		var cancelRun context.CancelFunc
		ctx, cancelRun = context.WithTimeout(ctx, orchestrator.RunBudget.MaxWallClock)
		defer cancelRun()
	}

	completedPhases := map[string]bool{}

	for _, phase := range orderedPhases {
		if missingDependency, ok := firstUnmetDependency(phase, completedPhases); ok {
			phaseResult := PhaseResult{
				Name: phase.Name, Skipped: true,
				SkipReason: fmt.Sprintf("dependency %q did not complete", missingDependency),
			}
			runReport.PhaseResults = append(runReport.PhaseResults, phaseResult)
			orchestrator.Blackboard.NoteGap(phase.Name, "phase", phaseResult.SkipReason)
			orchestrator.logf("phase %s skipped: %s", phase.Name, phaseResult.SkipReason)
			continue
		}

		phaseResult := orchestrator.runOnePhase(ctx, phase)
		runReport.PhaseResults = append(runReport.PhaseResults, phaseResult)

		if phaseResult.Succeeded() {
			completedPhases[phase.Name] = true
			continue
		}
		if phase.Optional {
			orchestrator.Blackboard.NoteGap(phase.Name, "phase", phaseResult.Err.Error())
			orchestrator.logf("optional phase %s failed, continuing: %v", phase.Name, phaseResult.Err)
			continue
		}

		runReport.Aborted = true
		runReport.AbortReason = fmt.Sprintf("required phase %q failed: %v", phase.Name, phaseResult.Err)
		orchestrator.Blackboard.NoteGap(phase.Name, "phase", runReport.AbortReason)
		orchestrator.logf("%s", runReport.AbortReason)
		break
	}

	runReport.TotalDuration = orchestrator.nowFunc().Sub(runStartedAt)
	return runReport, nil
}

func (orchestrator *Orchestrator) runOnePhase(ctx context.Context, phase Phase) PhaseResult {
	phaseResult := PhaseResult{Name: phase.Name, Started: orchestrator.nowFunc()}
	orchestrator.logf("phase %s starting", phase.Name)

	phaseCtx := ctx
	if phase.Budget.MaxWallClock > 0 {
		var cancelPhase context.CancelFunc
		phaseCtx, cancelPhase = context.WithTimeout(ctx, phase.Budget.MaxWallClock)
		defer cancelPhase()
	}

	if err := phase.Run(phaseCtx, orchestrator.Blackboard); err != nil {
		phaseResult.Err = err
	} else if phase.Postcondition != nil {
		// The explicit termination condition. Reaching the end of Run is not
		// the same as having done the work.
		if err := phase.Postcondition(orchestrator.Blackboard); err != nil {
			phaseResult.Err = fmt.Errorf("postcondition not met: %w", err)
		}
	}

	phaseResult.Duration = orchestrator.nowFunc().Sub(phaseResult.Started)
	orchestrator.writeCheckpoint(phase.Name)

	if phaseResult.Err != nil {
		orchestrator.logf("phase %s failed after %s: %v", phase.Name, phaseResult.Duration, phaseResult.Err)
	} else {
		orchestrator.logf("phase %s completed in %s", phase.Name, phaseResult.Duration)
	}
	return phaseResult
}

func (orchestrator *Orchestrator) writeCheckpoint(phaseName string) {
	if orchestrator.Checkpoint == nil {
		return
	}
	snapshot, err := orchestrator.Blackboard.Checkpoint()
	if err != nil {
		orchestrator.logf("checkpoint after %s failed to serialise: %v", phaseName, err)
		return
	}
	if err := orchestrator.Checkpoint.Write(orchestrator.Blackboard.RunID, phaseName, snapshot); err != nil {
		// A checkpoint failure costs resumability, not the run.
		orchestrator.logf("checkpoint after %s not written: %v", phaseName, err)
	}
}

func firstUnmetDependency(phase Phase, completedPhases map[string]bool) (string, bool) {
	for _, dependencyName := range phase.DependsOn {
		if !completedPhases[dependencyName] {
			return dependencyName, true
		}
	}
	return "", false
}

// topologicalOrder sorts phases so dependencies run first, and refuses a cycle.
//
// This is the strongest loop guard in the system, and it is a data structure
// rather than a detector: a cyclic pipeline cannot be constructed, so it cannot
// be entered.
func topologicalOrder(phases []Phase) ([]Phase, error) {
	phasesByName := make(map[string]Phase, len(phases))
	for _, phase := range phases {
		if _, duplicate := phasesByName[phase.Name]; duplicate {
			return nil, fmt.Errorf("orchestrator: duplicate phase %q", phase.Name)
		}
		phasesByName[phase.Name] = phase
	}
	for _, phase := range phases {
		for _, dependencyName := range phase.DependsOn {
			if _, known := phasesByName[dependencyName]; !known {
				return nil, fmt.Errorf("orchestrator: phase %q depends on unknown phase %q", phase.Name, dependencyName)
			}
		}
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	visitState := make(map[string]int, len(phases))
	ordered := make([]Phase, 0, len(phases))

	var visit func(phaseName string, path []string) error
	visit = func(phaseName string, path []string) error {
		switch visitState[phaseName] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("orchestrator: dependency cycle: %v -> %s", path, phaseName)
		}
		visitState[phaseName] = visiting
		phase := phasesByName[phaseName]

		dependencyNames := append([]string{}, phase.DependsOn...)
		sort.Strings(dependencyNames)
		for _, dependencyName := range dependencyNames {
			if err := visit(dependencyName, append(path, phaseName)); err != nil {
				return err
			}
		}
		visitState[phaseName] = visited
		ordered = append(ordered, phase)
		return nil
	}

	// Preserve declaration order among independent phases so runs are
	// reproducible.
	for _, phase := range phases {
		if err := visit(phase.Name, nil); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}
