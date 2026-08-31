// Package emit holds the tools an agent uses to record its findings.
//
// Structured output is a tool call here rather than a parsed text response, and
// that choice pays three times over: the model is constrained by a JSON Schema
// the provider enforces, malformed output becomes a correctable tool error the
// model can fix, and every emission moves the blackboard revision counter so
// the no-progress guard measures real work rather than token spend.
package emit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool"
)

// ComponentTool records one analysed component.
type ComponentTool struct {
	Blackboard *memory.Blackboard
	// Base carries everything the runtime already knows exactly: path, blob
	// SHA, language, exported symbols and complexity, all from the AST. The
	// model is never asked for a fact that can be computed, so it cannot get
	// one wrong. It supplies only the interpretive fields on top.
	Base model.ComponentModel
}

func (componentTool *ComponentTool) Name() string { return "analysis_emit_component" }

func (componentTool *ComponentTool) Description() string {
	return "Record your analysis of the component you were asked to read. Call this exactly once, " +
		"when you have finished reading. Base every field on code you actually read."
}

func (componentTool *ComponentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{
    "responsibility":{"type":"string","description":"One or two sentences: what this component is responsible for"},
    "errorPaths":{"type":"array","items":{"type":"string"},"description":"Each distinct way this component can fail, as a short phrase"},
    "sideEffects":{"type":"array","items":{"type":"string"},"description":"Observable effects beyond the return value: writes, network calls, mutation of arguments"},
    "externalCalls":{"type":"array","items":{"type":"object","properties":{
        "name":{"type":"string"},
        "requiresMock":{"type":"boolean"},
        "reason":{"type":"string"}
      },"required":["name"],"additionalProperties":false},
      "description":"Dependencies a test must cross or stub"},
    "confidence":{"type":"number","minimum":0,"maximum":1,"description":"How well the code supported this analysis"}
  },
  "required":["responsibility","errorPaths"],
  "additionalProperties":false
}`)
}

func (componentTool *ComponentTool) Idempotent() bool { return true }

type componentArguments struct {
	Responsibility string   `json:"responsibility"`
	ErrorPaths     []string `json:"errorPaths"`
	SideEffects    []string `json:"sideEffects"`
	ExternalCalls  []struct {
		Name         string `json:"name"`
		RequiresMock bool   `json:"requiresMock"`
		Reason       string `json:"reason"`
	} `json:"externalCalls"`
	Confidence float64 `json:"confidence"`
}

func (componentTool *ComponentTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	var parsedArguments componentArguments
	if err := json.Unmarshal(arguments, &parsedArguments); err != nil {
		return tool.Result{}, tool.Correctable(componentTool.Name(),
			"arguments did not match the schema: "+err.Error()+"; \"responsibility\" and \"errorPaths\" are required")
	}
	if strings.TrimSpace(parsedArguments.Responsibility) == "" {
		return tool.Result{}, tool.Correctable(componentTool.Name(),
			"\"responsibility\" must not be empty; describe what this component does")
	}

	externalCalls := make([]model.Dependency, 0, len(parsedArguments.ExternalCalls))
	for _, externalCall := range parsedArguments.ExternalCalls {
		externalCalls = append(externalCalls, model.Dependency{
			Name: externalCall.Name, Language: componentTool.Base.Language,
			RequiresMock: externalCall.RequiresMock, Reason: externalCall.Reason,
		})
	}

	confidence := parsedArguments.Confidence
	if confidence <= 0 || confidence > 1 {
		confidence = 0.8
	}

	enrichedModel := componentTool.Base
	enrichedModel.Responsibility = strings.TrimSpace(parsedArguments.Responsibility)
	enrichedModel.SideEffects = parsedArguments.SideEffects
	enrichedModel.Confidence = confidence
	enrichedModel.AnalysedAt = time.Now().UTC()

	// Error paths and external calls are unioned rather than replaced: the AST
	// finds every function returning an error and every third-party import, and
	// the model adds the ones only a reader would notice.
	enrichedModel.ErrorPaths = unionStrings(enrichedModel.ErrorPaths, parsedArguments.ErrorPaths)
	enrichedModel.ExternalCalls = unionDependencies(enrichedModel.ExternalCalls, externalCalls)

	componentTool.Blackboard.AddComponentModel(enrichedModel)
	return tool.Text(fmt.Sprintf("Recorded analysis of %s. You are done; stop now.", enrichedModel.Path),
		tool.Internal()), nil
}

func unionStrings(existing, incoming []string) []string {
	seen := map[string]bool{}
	merged := []string{}
	for _, value := range append(append([]string{}, existing...), incoming...) {
		normalised := strings.TrimSpace(value)
		if normalised == "" || seen[strings.ToLower(normalised)] {
			continue
		}
		seen[strings.ToLower(normalised)] = true
		merged = append(merged, normalised)
	}
	return merged
}

func unionDependencies(existing, incoming []model.Dependency) []model.Dependency {
	seen := map[string]bool{}
	merged := []model.Dependency{}
	for _, dependency := range append(append([]model.Dependency{}, existing...), incoming...) {
		if dependency.Name == "" || seen[dependency.Name] {
			continue
		}
		seen[dependency.Name] = true
		merged = append(merged, dependency)
	}
	return merged
}

// ScenarioTool records test scenarios against a risk.
type ScenarioTool struct {
	Blackboard *memory.Blackboard
	// Risk is the entry these scenarios must address. Supplying it here rather
	// than trusting the model to echo it back keeps traceability exact.
	Risk model.Risk
	// IDPrefix makes scenario IDs stable across runs, which is what lets the
	// Jira ledger update a ticket instead of creating a duplicate.
	IDPrefix string

	emittedCount int
}

func (scenarioTool *ScenarioTool) Name() string { return "scenario_emit" }

func (scenarioTool *ScenarioTool) Description() string {
	return "Record one or more test scenarios for the risk you were given. Call this once with " +
		"every scenario you intend to write. A scenario must be specific enough that a QA engineer " +
		"could execute it without reading the source."
}

func (scenarioTool *ScenarioTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{
    "scenarios":{"type":"array","minItems":1,"items":{
      "type":"object",
      "properties":{
        "title":{"type":"string","description":"Imperative and specific, e.g. \"Post rejects a debit that exceeds the balance\""},
        "type":{"type":"string","enum":["unit","integration","contract","e2e","negative","boundary","performance","security"]},
        "symbol":{"type":"string","description":"The function or method under test"},
        "preconditions":{"type":"array","items":{"type":"string"}},
        "steps":{"type":"array","minItems":1,"items":{"type":"string"},"description":"Ordered actions"},
        "expectedResult":{"type":"string","description":"A single observable, checkable outcome"},
        "testData":{"type":"array","items":{"type":"string"}},
        "mocks":{"type":"array","items":{"type":"string"},"description":"Dependencies to stub"},
        "automatable":{"type":"boolean"},
        "notes":{"type":"string"}
      },
      "required":["title","type","steps","expectedResult"],
      "additionalProperties":false
    }}
  },
  "required":["scenarios"],
  "additionalProperties":false
}`)
}

func (scenarioTool *ScenarioTool) Idempotent() bool { return true }

type scenarioArguments struct {
	Scenarios []struct {
		Title          string   `json:"title"`
		Type           string   `json:"type"`
		Symbol         string   `json:"symbol"`
		Preconditions  []string `json:"preconditions"`
		Steps          []string `json:"steps"`
		ExpectedResult string   `json:"expectedResult"`
		TestData       []string `json:"testData"`
		Mocks          []string `json:"mocks"`
		Automatable    bool     `json:"automatable"`
		Notes          string   `json:"notes"`
	} `json:"scenarios"`
}

func (scenarioTool *ScenarioTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	var parsedArguments scenarioArguments
	if err := json.Unmarshal(arguments, &parsedArguments); err != nil {
		return tool.Result{}, tool.Correctable(scenarioTool.Name(),
			"arguments did not match the schema: "+err.Error())
	}
	if len(parsedArguments.Scenarios) == 0 {
		return tool.Result{}, tool.Correctable(scenarioTool.Name(),
			"\"scenarios\" must contain at least one scenario")
	}

	scenarios := make([]model.TestScenario, 0, len(parsedArguments.Scenarios))
	for _, incoming := range parsedArguments.Scenarios {
		if strings.TrimSpace(incoming.ExpectedResult) == "" || len(incoming.Steps) == 0 {
			return tool.Result{}, tool.Correctable(scenarioTool.Name(), fmt.Sprintf(
				"scenario %q needs at least one step and a non-empty expectedResult; "+
					"a scenario without a checkable outcome is not testable", incoming.Title))
		}

		scenarioTool.emittedCount++
		steps := make([]model.Step, 0, len(incoming.Steps))
		for stepIndex, stepAction := range incoming.Steps {
			steps = append(steps, model.Step{Ordinal: stepIndex + 1, Action: stepAction})
		}
		testData := make([]model.DataRequirement, 0, len(incoming.TestData))
		for _, dataDescription := range incoming.TestData {
			testData = append(testData, model.DataRequirement{Description: dataDescription})
		}
		mocks := make([]model.Dependency, 0, len(incoming.Mocks))
		for _, mockName := range incoming.Mocks {
			mocks = append(mocks, model.Dependency{Name: mockName, RequiresMock: true})
		}

		// When the model names a symbol, take that symbol's own line from the
		// parser. Keeping the risk anchor's line while swapping the symbol name
		// produces a link that says one function and points at another — and
		// when the anchor had no line at all, at nothing.
		sourceRef := scenarioTool.Risk.Ref
		if incoming.Symbol != "" {
			sourceRef.Symbol = incoming.Symbol
			if resolved, found := resolveSymbolRef(
				scenarioTool.Blackboard, scenarioTool.Risk.ComponentName, incoming.Symbol,
			); found {
				sourceRef = resolved
			}
		}

		scenarios = append(scenarios, model.TestScenario{
			ID:             fmt.Sprintf("%s-%03d", scenarioTool.IDPrefix, scenarioTool.emittedCount),
			Title:          strings.TrimSpace(incoming.Title),
			ComponentName:  scenarioTool.Risk.ComponentName,
			SourceRef:      sourceRef,
			Type:           normaliseScenarioType(incoming.Type),
			Priority:       model.PriorityForRisk(scenarioTool.Risk.Level),
			RiskRef:        scenarioTool.Risk.ID,
			Preconditions:  incoming.Preconditions,
			Steps:          steps,
			ExpectedResult: strings.TrimSpace(incoming.ExpectedResult),
			TestData:       testData,
			Mocks:          mocks,
			Automatable:    incoming.Automatable,
			Confidence:     0.8,
			Notes:          incoming.Notes,
		})
	}

	scenarioTool.Blackboard.AddScenarios(scenarios...)
	return tool.Text(fmt.Sprintf("Recorded %d scenarios for risk %s. You are done; stop now.",
		len(scenarios), scenarioTool.Risk.ID), tool.Internal()), nil
}

// resolveSymbolRef finds a named symbol's location in the analysed component,
// so a scenario's link lands on the function it is actually about.
func resolveSymbolRef(
	blackboard *memory.Blackboard, componentName, symbolName string,
) (model.SourceRef, bool) {
	if blackboard == nil || symbolName == "" {
		return model.SourceRef{}, false
	}
	// A model may write "Type.Method" where the parser recorded "Method".
	bareName := symbolName
	if lastDot := strings.LastIndex(symbolName, "."); lastDot >= 0 {
		bareName = symbolName[lastDot+1:]
	}

	for _, componentModel := range blackboard.ComponentModels() {
		if componentModel.ComponentName != componentName {
			continue
		}
		for _, symbol := range componentModel.PublicSymbols {
			if symbol.Name != symbolName && symbol.Name != bareName {
				continue
			}
			if symbol.Ref.StartLine == 0 {
				continue
			}
			resolved := symbol.Ref
			// Keep the name the model used; it is what the reader will search
			// for, and it may be more specific than the parser's.
			resolved.Symbol = symbolName
			return resolved, true
		}
	}
	return model.SourceRef{}, false
}

func normaliseScenarioType(rawType string) model.ScenarioType {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case "integration":
		return model.ScenarioIntegration
	case "contract":
		return model.ScenarioContract
	case "e2e", "end-to-end":
		return model.ScenarioEndToEnd
	case "negative":
		return model.ScenarioNegative
	case "boundary":
		return model.ScenarioBoundary
	case "performance", "perf":
		return model.ScenarioPerformance
	case "security":
		return model.ScenarioSecurity
	default:
		return model.ScenarioUnit
	}
}

// ReviewTool records the Critic's findings.
type ReviewTool struct {
	Blackboard *memory.Blackboard
}

func (reviewTool *ReviewTool) Name() string { return "review_emit" }

func (reviewTool *ReviewTool) Description() string {
	return "Record your review of the scenario catalogue. Call this exactly once. Pass an empty " +
		"findings array to approve. Only raise a finding you would defend to the author."
}

func (reviewTool *ReviewTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{
    "findings":{"type":"array","items":{
      "type":"object",
      "properties":{
        "scenarioId":{"type":"string","description":"The scenario at fault, if the finding is about one"},
        "riskRef":{"type":"string","description":"The risk left uncovered, if the finding is a gap"},
        "severity":{"type":"string","enum":["blocking","major","minor"]},
        "issue":{"type":"string","description":"What is wrong, in one sentence"},
        "suggestion":{"type":"string","description":"What would fix it"}
      },
      "required":["severity","issue"],
      "additionalProperties":false
    }}
  },
  "required":["findings"],
  "additionalProperties":false
}`)
}

func (reviewTool *ReviewTool) Idempotent() bool { return true }

func (reviewTool *ReviewTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	var parsedArguments struct {
		Findings []model.RevisionRequest `json:"findings"`
	}
	if err := json.Unmarshal(arguments, &parsedArguments); err != nil {
		return tool.Result{}, tool.Correctable(reviewTool.Name(),
			"arguments did not match the schema: "+err.Error()+"; pass \"findings\": [] to approve")
	}
	for findingIndex, finding := range parsedArguments.Findings {
		if strings.TrimSpace(finding.Issue) == "" {
			return tool.Result{}, tool.Correctable(reviewTool.Name(), fmt.Sprintf(
				"finding %d has an empty \"issue\"; say what is wrong or drop the finding", findingIndex))
		}
	}

	reviewTool.Blackboard.SetRevisionRequests(parsedArguments.Findings)
	if len(parsedArguments.Findings) == 0 {
		return tool.Text("Recorded: catalogue approved. You are done; stop now.", tool.Internal()), nil
	}
	return tool.Text(fmt.Sprintf("Recorded %d findings. You are done; stop now.",
		len(parsedArguments.Findings)), tool.Internal()), nil
}
