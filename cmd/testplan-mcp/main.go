// Command testplan-mcp exposes the pipeline as an MCP server, so an IDE or
// another agent can generate a test plan without shelling out to the CLI.
//
// A full run takes minutes, so generate_test_plan returns a task handle
// immediately and the caller polls tasks/get. Holding a request open for the
// length of a run would be the wrong shape even before a human approval gate
// is involved.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/approval"
	"github.com/giri-ms19/testplan-agent/internal/guard"
	"github.com/giri-ms19/testplan-agent/internal/llm"
	"github.com/giri-ms19/testplan-agent/internal/llm/anthropic"
	"github.com/giri-ms19/testplan-agent/internal/mcpx"
	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/pipeline"
	"github.com/giri-ms19/testplan-agent/internal/render"
	"github.com/giri-ms19/testplan-agent/internal/tool/repo"
)

const generatePlanSchema = `{
  "type":"object",
  "properties":{
    "path":{"type":"string","description":"Absolute path to a local repository to analyse"},
    "name":{"type":"string","description":"Repository name for the report"},
    "maxFiles":{"type":"integer","description":"Refuse repositories larger than this (default 400)"}
  },
  "required":["path"],
  "additionalProperties":false
}`

func main() {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	taskStore := mcpx.NewTaskStore()
	server := &mcpx.Server{
		Name:    "testplan-agent",
		Version: "0.2",
		Tasks:   taskStore,
		Tools: []mcpx.ServerTool{
			{
				Name: "generate_test_plan",
				Description: "Analyse a repository and produce a test planning document. " +
					"Returns a task handle immediately; poll tasks/get for the result.",
				InputSchema: json.RawMessage(generatePlanSchema),
				Handler:     makePlanHandler(taskStore),
			},
		},
	}

	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
		// Logging goes to stderr: stdout is the protocol channel and anything
		// written there that is not JSON-RPC corrupts the stream.
		fmt.Fprintf(os.Stderr, "testplan-mcp: %v\n", err)
		os.Exit(1)
	}
}

func makePlanHandler(taskStore *mcpx.TaskStore) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, arguments json.RawMessage) (string, error) {
		var parameters struct {
			Path     string `json:"path"`
			Name     string `json:"name"`
			MaxFiles int    `json:"maxFiles"`
		}
		if err := json.Unmarshal(arguments, &parameters); err != nil {
			return "", fmt.Errorf("could not decode arguments: %w", err)
		}
		if strings.TrimSpace(parameters.Path) == "" {
			return "", errors.New("\"path\" is required and must be an absolute repository path")
		}
		if parameters.MaxFiles <= 0 {
			parameters.MaxFiles = 400
		}
		displayName := parameters.Name
		if displayName == "" {
			displayName = filepath.Base(parameters.Path)
		}

		task := taskStore.Start()

		// The run outlives this call, so it gets its own context rather than
		// the request's — otherwise the work would be cancelled the moment the
		// handle is returned.
		runContext, cancelRun := context.WithTimeout(context.Background(), 45*time.Minute)
		go func() {
			defer cancelRun()
			reportMarkdown, err := generatePlan(runContext, taskStore, task.ID,
				parameters.Path, displayName, parameters.MaxFiles)
			if err != nil {
				taskStore.Fail(task.ID, err.Error())
				return
			}
			taskStore.Complete(task.ID, reportMarkdown)
		}()

		handle, err := json.Marshal(map[string]any{
			"taskId": task.ID, "status": task.Status,
			"note": "poll tasks/get with this taskId",
		})
		if err != nil {
			return "", err
		}
		return string(handle), nil
	}
}

func generatePlan(
	ctx context.Context, taskStore *mcpx.TaskStore, taskID, repositoryPath, displayName string, maxFiles int,
) (string, error) {
	blackboard := memory.NewBlackboard(taskID)
	configuration := pipeline.Config{
		Source:             repo.NewLocalSource(repositoryPath, maxFiles),
		Provider:           providerFromEnvironment(),
		RepositoryName:     displayName,
		AnalystConcurrency: 4,
		MaxRevisionRounds:  2,
		RunBudget:          guard.Budget{MaxWallClock: 40 * time.Minute},
		OnPhase:            func(phaseName string) { taskStore.SetPhase(taskID, phaseName) },
	}

	runReport, err := pipeline.Build(configuration, blackboard).Run(ctx)
	if err != nil {
		return "", err
	}

	validationReport := approval.Validate(blackboard)
	reportMarkdown := render.PlanMarkdown(blackboard, render.Options{Validation: &validationReport})

	if runReport.Aborted {
		// A partial plan with an honest gaps section is still worth returning.
		return reportMarkdown, nil
	}
	return reportMarkdown, nil
}

func providerFromEnvironment() llm.Provider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil
	}
	return anthropic.New(apiKey)
}
