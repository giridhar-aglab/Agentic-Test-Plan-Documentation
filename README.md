# testplan-agent

A multi-agent system in Go that reads a repository and produces a test planning
document. Architecture doc: `claude/architecture.md` in the project.

**Status: slice 1 of the build — the runtime core and the Surveyor.**

Everything in this slice runs with no API key, no network and no MCP server, and
the whole control layer is exercised by tests. That is deliberate: the control
layer is where five of the six client requirements actually live, so it is the
part that has to be right before anything else is worth writing.

## Run it

```bash
go test ./...                                         # 48 tests, all green
go run ./cmd/testplan -source testdata/fixtures/paymentsvc -name paymentsvc -v
```

Flags: `-source`, `-name`, `-out`, `-checkpoints`, `-max-files` (default 400), `-v`.

## What is built

| Package | What it does |
|---|---|
| `internal/model` | The typed values agents exchange. No agent ever passes another a transcript. |
| `internal/llm` | Provider abstraction + a scriptable fake. Agents ask for a *tier*, never a model name. |
| `internal/tool` | The one `Tool` interface, the scoped registry, and the policy layer: timeouts, retry with jitter, circuit breaker, fallback chains. |
| `internal/tool/repo` | Repository access behind a `Source` seam. Local directory now; the GitHub MCP client drops in behind it. |
| `internal/tool/code` | `go/ast` analysis: symbols, signatures, error returns, branch complexity. |
| `internal/memory` | Blackboard (typed run state, checkpointed), working set (compaction), spill store (handles + digests). |
| `internal/guard` | Budget, repeat-call, no-progress, scope. |
| `internal/agent` | The bounded iteration loop, and the Surveyor. |
| `internal/orchestrator` | Phase DAG with postconditions, budgets, checkpointing. |
| `internal/render` | Deterministic Markdown. Same blackboard, same bytes, every time. |

## Where each requirement lives

| Requirement | Code | Tests that prove it |
|---|---|---|
| Multi-agent architecture | `orchestrator/`, `agent/` | `TestPhasesRunInDependencyOrder`, `TestDependentPhaseIsSkippedNotRun` |
| MCP service integration | `tool/repo.Source` seam | *(slice 2)* |
| Internal and external tool calls | `tool.Tool` + `Registry` | `TestScopedRegistryRefusesUnknownTool` |
| Agent loop avoidance | `guard/`, `orchestrator.topologicalOrder` | `TestRepeatCallGuardGraduates…`, `TestCyclicPipelineIsRefused`, `TestLoopStopsOnIterationBudget` |
| Long-term and short-term memory | `memory/` | `TestWorkingSetCompacts…`, `TestBlackboardCheckpointRoundTrips` |
| Tool call failure handling | `tool/policy.go`, `agent/loop.go` | `TestNonIdempotentToolIsNeverRetried`, `TestBreakerOpensAndHidesToolFromModel`, `TestLoopFeedsCorrectableErrorBackToModel` |

## Four decisions worth knowing before you read the code

**Findings live in structs, not in the conversation.** Agents write typed values
to the blackboard. A sub-agent starts with a fresh minimal context holding its
task and the blackboard slices it needs. This bounds context growth, makes every
handoff testable with ordinary `go test`, and removes the transcript
accumulation that causes runaway agent cost.

**A transport failure and a correctable error are different things.** A timeout
is the runtime's problem: retry it, break the circuit, fall back — the model
never sees it. A tool *rejecting its arguments* is the model's problem, and it
reaches the model as readable, actionable text ("no such path `x`; call
`repo.tree` to list valid paths"). Collapsing these two is the most common defect
in agent implementations; `TestLoopFeedsCorrectableErrorBackToModel` and
`TestCorrectableFailureIsNotRetriedAndDoesNotTripBreaker` pin the distinction.

**The strongest loop guard is a data structure.** Phases form a DAG and agents
cannot invoke each other, so mutual recursion is not detected — it cannot be
constructed. `TestCyclicPipelineIsRefused` fails at wiring time, not at runtime.
The four runtime guards catch the remaining case: a single agent spinning alone.

**Determinism where determinism is possible.** Classifying files, parsing
manifests, scoring complexity and rendering the report are all model-free. A
model would make them slower, approximate, and different on every run. The
Surveyor calls a model for exactly one thing — a sentence describing what the
repository is for — and the survey is complete without it.

## Not yet built

Analyst, Risk, Author, Critic, the approval gate and Jira publication; the
GitHub MCP client and the MCP server; the Anthropic provider adapter; the
SQLite long-term store; Python analysis.

The seams they attach to already exist: `repo.Source` for GitHub, `llm.Provider`
for the model, `orchestrator.Phase` for each new agent, and `tool.Tool` for
everything either of them needs.
