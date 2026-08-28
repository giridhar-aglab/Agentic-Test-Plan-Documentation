package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

func recordingPhase(name string, executionOrder *[]string, phaseErr error) Phase {
	return Phase{
		Name: name,
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			*executionOrder = append(*executionOrder, name)
			return phaseErr
		},
	}
}

func TestPhasesRunInDependencyOrder(t *testing.T) {
	executionOrder := []string{}
	pipeline := New(memory.NewBlackboard("r1"))

	// Declared out of order on purpose.
	authorPhase := recordingPhase("author", &executionOrder, nil)
	authorPhase.DependsOn = []string{"risk"}
	riskPhase := recordingPhase("risk", &executionOrder, nil)
	riskPhase.DependsOn = []string{"survey"}

	pipeline.AddPhase(authorPhase)
	pipeline.AddPhase(riskPhase)
	pipeline.AddPhase(recordingPhase("survey", &executionOrder, nil))

	if _, err := pipeline.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedOrder := []string{"survey", "risk", "author"}
	if strings.Join(executionOrder, ",") != strings.Join(expectedOrder, ",") {
		t.Fatalf("expected %v, got %v", expectedOrder, executionOrder)
	}
}

func TestCyclicPipelineIsRefused(t *testing.T) {
	// The strongest loop guard in the system: a cyclic pipeline cannot be
	// constructed, so it can never be entered.
	pipeline := New(memory.NewBlackboard("r1"))
	firstPhase := recordingPhase("a", &[]string{}, nil)
	firstPhase.DependsOn = []string{"b"}
	secondPhase := recordingPhase("b", &[]string{}, nil)
	secondPhase.DependsOn = []string{"a"}
	pipeline.AddPhase(firstPhase)
	pipeline.AddPhase(secondPhase)

	_, err := pipeline.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected a cycle to be refused at wiring time, got %v", err)
	}
}

func TestPostconditionFailureFailsThePhase(t *testing.T) {
	// Reaching the end of Run is not the same as having done the work.
	pipeline := New(memory.NewBlackboard("r1"))
	pipeline.AddPhase(Phase{
		Name: "survey",
		Run:  func(ctx context.Context, blackboard *memory.Blackboard) error { return nil },
		Postcondition: func(blackboard *memory.Blackboard) error {
			if blackboard.RepoMap() == nil {
				return errors.New("no repository map was produced")
			}
			return nil
		},
	})

	runReport, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runReport.PhaseResults[0].Succeeded() {
		t.Fatal("a phase whose postcondition fails must not be reported as succeeded")
	}
	if !runReport.Aborted {
		t.Fatal("a failed required phase must abort the run")
	}
}

func TestDependentPhaseIsSkippedNotRun(t *testing.T) {
	executionOrder := []string{}
	pipeline := New(memory.NewBlackboard("r1"))

	optionalFailing := recordingPhase("risk", &executionOrder, errors.New("scoring unavailable"))
	optionalFailing.Optional = true
	dependent := recordingPhase("author", &executionOrder, nil)
	dependent.DependsOn = []string{"risk"}

	pipeline.AddPhase(optionalFailing)
	pipeline.AddPhase(dependent)

	runReport, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("an optional phase failing must not error the run: %v", err)
	}
	if runReport.Aborted {
		t.Fatal("an optional phase must not abort the run")
	}
	for _, phaseName := range executionOrder {
		if phaseName == "author" {
			t.Fatal("a phase whose dependency did not complete must be skipped, not run")
		}
	}
	// The skip has to be visible in the report, or the plan silently omits work.
	if len(pipeline.Blackboard.Gaps()) == 0 {
		t.Fatal("skipped and failed phases must be recorded as gaps")
	}
}

// countingCheckpointWriter records how many snapshots were written.
type countingCheckpointWriter struct {
	writeCount int
	lastErr    error
}

func (writer *countingCheckpointWriter) Write(runID, phaseName string, snapshot []byte) error {
	writer.writeCount++
	return writer.lastErr
}

func TestCheckpointWrittenAfterEveryPhase(t *testing.T) {
	writer := &countingCheckpointWriter{}
	executionOrder := []string{}
	pipeline := New(memory.NewBlackboard("r1"))
	pipeline.Checkpoint = writer
	pipeline.AddPhase(recordingPhase("survey", &executionOrder, nil))
	pipeline.AddPhase(recordingPhase("analyse", &executionOrder, nil))

	if _, err := pipeline.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer.writeCount != 2 {
		t.Fatalf("expected a checkpoint after each phase, got %d", writer.writeCount)
	}
}

func TestCheckpointFailureDoesNotFailTheRun(t *testing.T) {
	// Losing a checkpoint costs resumability, not the run.
	writer := &countingCheckpointWriter{lastErr: errors.New("disk full")}
	executionOrder := []string{}
	pipeline := New(memory.NewBlackboard("r1"))
	pipeline.Checkpoint = writer
	pipeline.AddPhase(recordingPhase("survey", &executionOrder, nil))

	runReport, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("a checkpoint failure must not fail the run: %v", err)
	}
	if runReport.Aborted {
		t.Fatal("a checkpoint failure must not abort the run")
	}
}

func TestBlackboardCheckpointRoundTrips(t *testing.T) {
	blackboard := memory.NewBlackboard("r1")
	blackboard.SetRepoMap(model.RepoMap{RepositoryName: "paymentsvc", CommitSHA: "abc123"})
	blackboard.AddScenarios(model.TestScenario{ID: "TS-ledger-001", Title: "Post rejects a frozen account"})
	blackboard.NoteGap("analyse", "internal/x.go", "unreadable")

	snapshot, err := blackboard.Checkpoint()
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	restored, err := memory.RestoreBlackboard(snapshot)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.RepoMap() == nil || restored.RepoMap().RepositoryName != "paymentsvc" {
		t.Fatal("the repository map must survive a checkpoint round trip")
	}
	if len(restored.Scenarios()) != 1 || len(restored.Gaps()) != 1 {
		t.Fatal("scenarios and gaps must survive a checkpoint round trip")
	}
}
