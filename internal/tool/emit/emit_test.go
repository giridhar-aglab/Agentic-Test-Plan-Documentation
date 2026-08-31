package emit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

func TestAScenarioLinksToTheSymbolItNames(t *testing.T) {
	// A link that says one function and points at another — or at no line at
	// all — sends a reviewer to the wrong place, which is worse than no link.
	blackboard := memory.NewBlackboard("run")
	blackboard.AddComponentModel(model.ComponentModel{
		ComponentName: "gin/response-writer",
		Path:          "response_writer.go",
		PublicSymbols: []model.Symbol{
			{Name: "Flush", Exported: true, BranchCount: 9,
				Ref: model.SourceRef{Path: "response_writer.go", Symbol: "Flush", StartLine: 20}},
			{Name: "WriteHeader", Exported: true, BranchCount: 2,
				Ref: model.SourceRef{Path: "response_writer.go", Symbol: "WriteHeader", StartLine: 71}},
		},
	})

	scenarioTool := &ScenarioTool{
		Blackboard: blackboard,
		IDPrefix:   "TS-response-writer",
		Risk: model.Risk{
			ID: "RISK-response-writer", ComponentName: "gin/response-writer",
			Level: model.RiskCritical,
			// The anchor is the busiest symbol, which is not the one the model
			// chose to write about.
			Ref: model.SourceRef{Path: "response_writer.go", Symbol: "Flush", StartLine: 20},
		},
	}

	_, err := scenarioTool.Invoke(context.Background(), json.RawMessage(`{"scenarios":[{
		"title":"WriteHeader does not override a status already written",
		"type":"unit","symbol":"responseWriter.WriteHeader",
		"steps":["call WriteHeaderNow","call WriteHeader with another status"],
		"expectedResult":"the original status remains"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scenarios := blackboard.Scenarios()
	if len(scenarios) != 1 {
		t.Fatalf("expected one scenario, got %d", len(scenarios))
	}
	reference := scenarios[0].SourceRef
	if reference.StartLine != 71 {
		t.Errorf("the link should point at WriteHeader (line 71), got line %d", reference.StartLine)
	}
	if reference.Symbol != "responseWriter.WriteHeader" {
		t.Errorf("the model's own symbol name should survive, got %q", reference.Symbol)
	}
}

func TestAnUnresolvableSymbolKeepsTheRiskAnchor(t *testing.T) {
	// If the named symbol is not in the parsed component, the anchor is still
	// the best available pointer — better than dropping the link entirely.
	blackboard := memory.NewBlackboard("run")
	blackboard.AddComponentModel(model.ComponentModel{
		ComponentName: "pkg/thing", Path: "thing.go",
		PublicSymbols: []model.Symbol{{Name: "Known", Exported: true,
			Ref: model.SourceRef{Path: "thing.go", Symbol: "Known", StartLine: 5}}},
	})
	scenarioTool := &ScenarioTool{
		Blackboard: blackboard, IDPrefix: "TS-thing",
		Risk: model.Risk{
			ID: "RISK-thing", ComponentName: "pkg/thing", Level: model.RiskHigh,
			Ref: model.SourceRef{Path: "thing.go", Symbol: "Known", StartLine: 5},
		},
	}
	_, err := scenarioTool.Invoke(context.Background(), json.RawMessage(`{"scenarios":[{
		"title":"something","type":"unit","symbol":"Invented",
		"steps":["do it"],"expectedResult":"it happens"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reference := blackboard.Scenarios()[0].SourceRef
	if reference.StartLine != 5 || reference.Path != "thing.go" {
		t.Fatalf("expected the anchor to be kept, got %+v", reference)
	}
}
