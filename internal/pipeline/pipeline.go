// Package pipeline wires the specialists into the phase DAG.
//
// It exists so the CLI and the MCP server build exactly the same pipeline. Two
// entrypoints that assemble their own phases would drift, and the one that gets
// less use would drift silently.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/agent"
	"github.com/giri-ms19/testplan-agent/internal/approval"
	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/orchestrator"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

// Config describes one run.
type Config struct {
	Source         repo.Source
	Provider       llm.Provider
	RepositoryName string
	CommitSHA      string

	// AnalystConcurrency bounds the fan-out.
	AnalystConcurrency int
	// MaxRevisionRounds caps the Author/Critic cycle.
	MaxRevisionRounds int
	// RunBudget bounds the whole run independently of per-phase budgets.
	RunBudget guard.Budget

	Checkpoint orchestrator.CheckpointWriter
	Logf       func(format string, arguments ...any)
	// OnPhase is called as each phase starts, so an MCP caller polling tasks/get
	// sees movement rather than a black box.
	OnPhase func(phaseName string)
}

// HasProvider reports whether a model is configured. Without one the pipeline
// still runs end to end using the deterministic fallbacks, which is how the
// fixture tests exercise every phase without a network.
func (config Config) HasProvider() bool { return config.Provider != nil }

// Build assembles the orchestrator.
func Build(config Config, blackboard *memory.Blackboard) *orchestrator.Orchestrator {
	pipeline := orchestrator.New(blackboard)
	pipeline.RunBudget = config.RunBudget
	pipeline.Checkpoint = config.Checkpoint
	pipeline.Logf = config.Logf

	addSurveyPhase(pipeline, config, blackboard)
	addAnalysePhase(pipeline, config)
	addRiskPhase(pipeline, config)
	addAuthorPhase(pipeline, config)
	addValidatePhase(pipeline, config)
	addApprovalPhase(pipeline, config)
	return pipeline
}

func (config Config) notePhase(phaseName string) {
	if config.OnPhase != nil {
		config.OnPhase(phaseName)
	}
}

func addSurveyPhase(pipeline *orchestrator.Orchestrator, config Config, blackboard *memory.Blackboard) {
	surveyor := &agent.Surveyor{
		Source:         config.Source,
		RepositoryName: config.RepositoryName,
		CommitSHA:      config.CommitSHA,
	}
	pipeline.AddPhase(orchestrator.Phase{
		Name:   "survey",
		Budget: guard.Budget{MaxWallClock: 5 * time.Minute},
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			config.notePhase("survey")
			return surveyor.Survey(ctx, blackboard)
		},
		Postcondition: func(blackboard *memory.Blackboard) error {
			repoMap := blackboard.RepoMap()
			if repoMap == nil {
				return errors.New("no repository map was produced")
			}
			if len(repoMap.SelectedFiles()) == 0 {
				return errors.New("no analysable source files were selected")
			}
			return nil
		},
	})
}

func addAnalysePhase(pipeline *orchestrator.Orchestrator, config Config) {
	pipeline.AddPhase(orchestrator.Phase{
		Name:      "analyse",
		DependsOn: []string{"survey"},
		Budget:    guard.Budget{MaxWallClock: 20 * time.Minute},
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			config.notePhase("analyse")
			if !config.HasProvider() {
				return agent.AnalystFallback(ctx, config.Source, blackboard)
			}
			analyst := &agent.Analyst{
				Source:      config.Source,
				Provider:    config.Provider,
				Blackboard:  blackboard,
				Concurrency: config.AnalystConcurrency,
				PerComponentBudget: guard.Budget{
					MaxIterations: 8, MaxToolCalls: 20, MaxTokens: 60000, MaxWallClock: 3 * time.Minute,
				},
				Logf: config.Logf,
			}
			return analyst.Analyse(ctx, blackboard)
		},
		Postcondition: func(blackboard *memory.Blackboard) error {
			if len(blackboard.ComponentModels()) == 0 {
				return errors.New("no components were analysed")
			}
			return nil
		},
	})
}

func addRiskPhase(pipeline *orchestrator.Orchestrator, config Config) {
	pipeline.AddPhase(orchestrator.Phase{
		Name:      "risk",
		DependsOn: []string{"analyse"},
		Budget:    guard.Budget{MaxWallClock: time.Minute},
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			config.notePhase("risk")
			return agent.NewRiskScorer().Score(blackboard)
		},
		Postcondition: func(blackboard *memory.Blackboard) error {
			riskRegister := blackboard.RiskRegister()
			if riskRegister == nil || len(riskRegister.Risks) == 0 {
				return errors.New("the risk register is empty")
			}
			return nil
		},
	})
}

func addAuthorPhase(pipeline *orchestrator.Orchestrator, config Config) {
	pipeline.AddPhase(orchestrator.Phase{
		Name:      "author",
		DependsOn: []string{"risk"},
		Budget:    guard.Budget{MaxWallClock: 20 * time.Minute},
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			config.notePhase("author")
			if !config.HasProvider() {
				return agent.ScenarioFallback(blackboard)
			}
			revisionCycle := &agent.RevisionCycle{
				Author: &agent.Author{
					Source: config.Source, Provider: config.Provider,
					PerRiskBudget: guard.Budget{
						MaxIterations: 8, MaxToolCalls: 20, MaxTokens: 80000, MaxWallClock: 4 * time.Minute,
					},
					Logf: config.Logf,
				},
				Critic: &agent.Critic{
					Provider: config.Provider,
					Budget: guard.Budget{
						MaxIterations: 4, MaxToolCalls: 6, MaxTokens: 60000, MaxWallClock: 3 * time.Minute,
					},
				},
				MaxRounds: config.MaxRevisionRounds,
				Logf:      config.Logf,
			}
			return revisionCycle.Run(ctx, blackboard)
		},
		// Every prioritised risk must have at least one scenario. This is the
		// explicit termination condition for the phase, and it is not the same
		// thing as the model having stopped asking for tools.
		Postcondition: func(blackboard *memory.Blackboard) error {
			scenarios := blackboard.Scenarios()
			if len(scenarios) == 0 {
				return errors.New("no scenarios were produced")
			}
			riskRegister := blackboard.RiskRegister()
			if riskRegister == nil {
				return nil
			}
			coveredRiskIDs := map[string]bool{}
			for _, scenario := range scenarios {
				coveredRiskIDs[scenario.RiskRef] = true
			}
			uncoveredCount := 0
			for _, prioritisedRisk := range riskRegister.Prioritised() {
				if !coveredRiskIDs[prioritisedRisk.ID] {
					uncoveredCount++
				}
			}
			// Partial coverage is recorded, not fatal: a plan covering most of
			// the prioritised risks with an honest gaps section beats no plan.
			if uncoveredCount > 0 {
				blackboard.NoteGap("author", "coverage",
					fmt.Sprintf("%d prioritised risks have no scenario", uncoveredCount))
			}
			return nil
		},
	})
}

func addValidatePhase(pipeline *orchestrator.Orchestrator, config Config) {
	pipeline.AddPhase(orchestrator.Phase{
		Name:      "validate",
		DependsOn: []string{"author"},
		Budget:    guard.Budget{MaxWallClock: time.Minute},
		// Optional so that structural defects surface in the report rather than
		// aborting the run. A reviewer who can see what is wrong is better
		// served than one who gets nothing.
		Optional: true,
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			config.notePhase("validate")
			validationReport := approval.Validate(blackboard)
			for _, defect := range validationReport.Blocking() {
				blackboard.NoteGap("validate", defect.Rule, defect.String())
			}
			if !validationReport.OK() {
				return fmt.Errorf("%d structural defects remain", len(validationReport.Blocking()))
			}
			return nil
		},
	})
}

func addApprovalPhase(pipeline *orchestrator.Orchestrator, config Config) {
	pipeline.AddPhase(orchestrator.Phase{
		Name:      "approval",
		DependsOn: []string{"author"},
		Run: func(ctx context.Context, blackboard *memory.Blackboard) error {
			config.notePhase("approval")
			// Phase 7 is a hard stop, not an agent. The run produces a report
			// and halts; nothing reaches Jira until a person has read it and
			// said yes. Publication tools are not even constructed here.
			blackboard.NoteGap("approval", "verdict",
				"awaiting human approval; no scenario has been published")
			return nil
		},
	})
}
