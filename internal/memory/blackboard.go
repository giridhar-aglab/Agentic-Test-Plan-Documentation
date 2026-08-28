// Package memory holds the two kinds of state the system distinguishes:
// short-term working context that lives for one run, and the typed blackboard
// that is the real memory. The chat transcript is a scratchpad; findings live
// in structs.
package memory

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/model"
)

// Gap records something the run could not do. Gaps are first-class: a plan
// covering 90% of a repository with an honest list of what it missed is useful,
// and a stack trace is not.
type Gap struct {
	Phase   string    `json:"phase"`
	Subject string    `json:"subject"`
	Reason  string    `json:"reason"`
	NotedAt time.Time `json:"notedAt"`
}

// Blackboard is the shared typed store. Specialists read and write it; the
// orchestrator is the only thing that advances phases. Because agents never
// talk to each other, there is no agent-to-agent conversation that can loop.
type Blackboard struct {
	mutex sync.RWMutex

	RunID     string    `json:"runId"`
	StartedAt time.Time `json:"startedAt"`

	repoMap         *model.RepoMap
	componentModels []model.ComponentModel
	riskRegister    *model.RiskRegister
	testScenarios   []model.TestScenario
	verdicts        []model.Verdict
	gaps            []Gap
	degradedTools   map[string]string

	// revision counts every mutation. The no-progress guard reads it, which is
	// why "did anything happen this iteration" is a cheap integer comparison
	// rather than a deep diff.
	revision int
}

// NewBlackboard creates an empty blackboard for a run.
func NewBlackboard(runID string) *Blackboard {
	return &Blackboard{
		RunID:         runID,
		StartedAt:     time.Now().UTC(),
		degradedTools: map[string]string{},
	}
}

// Revision returns the mutation counter.
func (blackboard *Blackboard) Revision() int {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	return blackboard.revision
}

func (blackboard *Blackboard) bumpRevision() { blackboard.revision++ }

// SetRepoMap stores the Surveyor's output.
func (blackboard *Blackboard) SetRepoMap(repoMap model.RepoMap) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	blackboard.repoMap = &repoMap
	blackboard.bumpRevision()
}

// RepoMap returns the survey, or nil if the phase has not run.
func (blackboard *Blackboard) RepoMap() *model.RepoMap {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	if blackboard.repoMap == nil {
		return nil
	}
	repoMapCopy := *blackboard.repoMap
	return &repoMapCopy
}

// AddComponentModel appends one analysed component, replacing any earlier
// analysis of the same path so a retried file does not duplicate.
func (blackboard *Blackboard) AddComponentModel(componentModel model.ComponentModel) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	for existingIndex, existingModel := range blackboard.componentModels {
		if existingModel.Path == componentModel.Path {
			blackboard.componentModels[existingIndex] = componentModel
			blackboard.bumpRevision()
			return
		}
	}
	blackboard.componentModels = append(blackboard.componentModels, componentModel)
	blackboard.bumpRevision()
}

// ComponentModels returns a copy of the analysed components.
func (blackboard *Blackboard) ComponentModels() []model.ComponentModel {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	modelsCopy := make([]model.ComponentModel, len(blackboard.componentModels))
	copy(modelsCopy, blackboard.componentModels)
	return modelsCopy
}

// SetRiskRegister stores the prioritised register.
func (blackboard *Blackboard) SetRiskRegister(riskRegister model.RiskRegister) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	blackboard.riskRegister = &riskRegister
	blackboard.bumpRevision()
}

// RiskRegister returns the register, or nil.
func (blackboard *Blackboard) RiskRegister() *model.RiskRegister {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	if blackboard.riskRegister == nil {
		return nil
	}
	registerCopy := *blackboard.riskRegister
	return &registerCopy
}

// AddScenarios appends scenarios, replacing any with a matching ID.
func (blackboard *Blackboard) AddScenarios(scenarios ...model.TestScenario) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	for _, incomingScenario := range scenarios {
		replaced := false
		for existingIndex, existingScenario := range blackboard.testScenarios {
			if existingScenario.ID == incomingScenario.ID {
				blackboard.testScenarios[existingIndex] = incomingScenario
				replaced = true
				break
			}
		}
		if !replaced {
			blackboard.testScenarios = append(blackboard.testScenarios, incomingScenario)
		}
	}
	blackboard.bumpRevision()
}

// Scenarios returns a copy of the catalogue.
func (blackboard *Blackboard) Scenarios() []model.TestScenario {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	scenariosCopy := make([]model.TestScenario, len(blackboard.testScenarios))
	copy(scenariosCopy, blackboard.testScenarios)
	return scenariosCopy
}

// AddVerdicts records approval-gate decisions.
func (blackboard *Blackboard) AddVerdicts(verdicts ...model.Verdict) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	blackboard.verdicts = append(blackboard.verdicts, verdicts...)
	blackboard.bumpRevision()
}

// Verdicts returns a copy of the recorded decisions.
func (blackboard *Blackboard) Verdicts() []model.Verdict {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	verdictsCopy := make([]model.Verdict, len(blackboard.verdicts))
	copy(verdictsCopy, blackboard.verdicts)
	return verdictsCopy
}

// NoteGap records something that could not be done.
func (blackboard *Blackboard) NoteGap(phaseName, subject, reason string) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	blackboard.gaps = append(blackboard.gaps, Gap{
		Phase: phaseName, Subject: subject, Reason: reason, NotedAt: time.Now().UTC(),
	})
	blackboard.bumpRevision()
}

// Gaps returns a copy of everything the run could not complete.
func (blackboard *Blackboard) Gaps() []Gap {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	gapsCopy := make([]Gap, len(blackboard.gaps))
	copy(gapsCopy, blackboard.gaps)
	return gapsCopy
}

// NoteDegradedTool records a capability that fell back or broke.
func (blackboard *Blackboard) NoteDegradedTool(toolName, reason string) {
	blackboard.mutex.Lock()
	defer blackboard.mutex.Unlock()
	blackboard.degradedTools[toolName] = reason
	blackboard.bumpRevision()
}

// DegradedTools returns a copy of the degraded capability map.
func (blackboard *Blackboard) DegradedTools() map[string]string {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	degradedCopy := make(map[string]string, len(blackboard.degradedTools))
	for toolName, reason := range blackboard.degradedTools {
		degradedCopy[toolName] = reason
	}
	return degradedCopy
}

// snapshot is the serialisable form used for checkpoints.
type snapshot struct {
	RunID           string                 `json:"runId"`
	StartedAt       time.Time              `json:"startedAt"`
	Revision        int                    `json:"revision"`
	RepoMap         *model.RepoMap         `json:"repoMap,omitempty"`
	ComponentModels []model.ComponentModel `json:"componentModels,omitempty"`
	RiskRegister    *model.RiskRegister    `json:"riskRegister,omitempty"`
	TestScenarios   []model.TestScenario   `json:"testScenarios,omitempty"`
	Verdicts        []model.Verdict        `json:"verdicts,omitempty"`
	Gaps            []Gap                  `json:"gaps,omitempty"`
	DegradedTools   map[string]string      `json:"degradedTools,omitempty"`
}

// Checkpoint serialises the blackboard. Written after every phase, so a crash
// costs one phase rather than the whole run.
func (blackboard *Blackboard) Checkpoint() ([]byte, error) {
	blackboard.mutex.RLock()
	defer blackboard.mutex.RUnlock()
	encoded, err := json.MarshalIndent(snapshot{
		RunID: blackboard.RunID, StartedAt: blackboard.StartedAt, Revision: blackboard.revision,
		RepoMap: blackboard.repoMap, ComponentModels: blackboard.componentModels,
		RiskRegister: blackboard.riskRegister, TestScenarios: blackboard.testScenarios,
		Verdicts: blackboard.verdicts, Gaps: blackboard.gaps, DegradedTools: blackboard.degradedTools,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("memory: checkpoint: %w", err)
	}
	return encoded, nil
}

// RestoreBlackboard rebuilds a blackboard from a checkpoint.
func RestoreBlackboard(encoded []byte) (*Blackboard, error) {
	var restored snapshot
	if err := json.Unmarshal(encoded, &restored); err != nil {
		return nil, fmt.Errorf("memory: restore checkpoint: %w", err)
	}
	blackboard := &Blackboard{
		RunID: restored.RunID, StartedAt: restored.StartedAt, revision: restored.Revision,
		repoMap: restored.RepoMap, componentModels: restored.ComponentModels,
		riskRegister: restored.RiskRegister, testScenarios: restored.TestScenarios,
		verdicts: restored.Verdicts, gaps: restored.Gaps,
		degradedTools: restored.DegradedTools,
	}
	if blackboard.degradedTools == nil {
		blackboard.degradedTools = map[string]string{}
	}
	return blackboard, nil
}
