# testplan-agent

A multi-agent system in Go that reads a repository and produces a test planning
document. Architecture doc: `claude/architecture.md` in the project.

**Status: all seven phases implemented. 116 tests, `go vet` clean, zero dependencies.**

## Run it

```bash
go test ./...                                                     # 116 tests

# Local directory, no API key — every phase runs with deterministic fallbacks
go run ./cmd/testplan -source testdata/fixtures/paymentsvc -name paymentsvc -v

# With a model
ANTHROPIC_API_KEY=sk-... go run ./cmd/testplan -source /path/to/repo -out plan.md

# GitHub — paste any repo URL
export ANTHROPIC_API_KEY=sk-...
export GITHUB_TOKEN=ghp_...                                   # private repos
export GITHUB_MCP_COMMAND="npx -y @modelcontextprotocol/server-github"
go run ./cmd/testplan -github https://github.com/owner/repo -out plan.md

# A branch in the URL is honoured; -ref overrides it
go run ./cmd/testplan -github https://github.com/owner/repo/tree/feature-x

# As an MCP server
go run ./cmd/testplan-mcp        # speaks JSON-RPC on stdio
```

## The seven phases

| # | Phase | What it does | Model |
|---|---|---|---|
| 1 | survey | Repo map, manifests, existing tests, entrypoints | Optional |
| 2 | analyse | Per-component behaviour and error paths, N-way parallel | Yes |
| 3 | risk | Prioritised register from complexity, failure paths, coverage | **No** |
| 4 | author | Scenarios per prioritised risk, ⇄ critic, ≤2 rounds | Yes |
| 5 | validate | Structural checks before a human reads anything | **No** |
| 6 | — | (render is part of the composer, not a phase) | **No** |
| 7 | approval | Hard stop. Nothing publishes until a person says yes | **No** |

Four of the seven use no model at all. That is deliberate — see below.

## Where each requirement lives

| Requirement | Code | Tests that prove it |
|---|---|---|
| Multi-agent architecture | `orchestrator/`, `agent/`, `pipeline/` | `TestPhasesRunInDependencyOrder`, `TestPipelineRunsEveryPhaseWithoutAProvider` |
| MCP service integration | `mcpx/` — client, GitHub source, and server | `TestEveryRequestCarriesProtocolMetaOnTheWire`, `TestRemoteToolIsIndistinguishableFromALocalTool` |
| Internal and external tool calls | `tool.Tool` + scoped `Registry` | `TestScopedRegistryRefusesUnknownTool`, `TestRemoteToolIsIndistinguishable…` |
| Agent loop avoidance | `guard/`, `orchestrator.topologicalOrder`, `RevisionCycle` | `TestRepeatCallGuardGraduates…`, `TestCyclicPipelineIsRefused`, `TestLoopStopsOnIterationBudget` |
| Long-term and short-term memory | `memory/` | `TestWorkingSetCompacts…`, `TestBlackboardCheckpointRoundTrips` |
| Tool call failure handling | `tool/policy.go`, `agent/loop.go`, `mcpx.classifyTransportError` | `TestNonIdempotentToolIsNeverRetried`, `TestBreakerOpensAndHidesToolFromModel`, `TestLoopFeedsCorrectableErrorBackToModel` |

## Six decisions worth knowing before you read the code

**Findings live in structs, not in the conversation.** Agents write typed values
to a blackboard; a sub-agent starts with a fresh context holding only its task
and the slices it needs. This is what lets the Analyst fan out across goroutines,
and it is why every handoff is testable with ordinary `go test`.

**Structured output is a tool call.** `analysis.emit_component`, `scenario.emit`
and `review.emit` are schema-constrained, so malformed output becomes a
correctable tool error the model can fix — and every emission moves the
blackboard revision counter, which is how the no-progress guard measures real
work rather than token spend.

**The model is never asked for a fact that can be computed.** The AST supplies
symbols, signatures, branch counts and third-party imports; the model supplies
only responsibility, error paths and side effects, which are then *unioned* with
what the parser already found. It cannot get the computable part wrong because
it is never asked.

**Risk scoring uses no model.** If a re-run silently reshuffled which components
are P0, the plan could not be diffed and a reviewer could not tell a real change
from sampling noise. The weights are exported and printed in the report, so a
reviewer who disagrees can point at a number.

**A transport failure and a correctable error are different things.** A timeout
is retried, breaks a circuit, falls back — the model never sees it. A tool
*rejecting its arguments* reaches the model as readable, actionable text. This
holds identically for in-process tools and for MCP servers, in both directions.

**Structural defects never reach the reviewer.** `report.validate` asserts that
every `SourceRef` resolves to a symbol that exists, every scenario has a
checkable expected result, no two scenarios duplicate, and every P0/P1 risk has
coverage. A reviewer's attention is the scarcest resource here and should not be
spent catching hallucinated file paths. `TestValidationCatchesAHallucinatedSymbol`
demonstrates the case with a deliberately lying provider.

## Deviations from the architecture doc

**The official MCP Go SDK is not vendored.** This build could not reach
`proxy.golang.org`, so `internal/mcpx` implements specification revision
`2026-07-28` directly over the standard library — stateless requests with
`_meta`, `server/discover`, `tools/list` cache honouring, and the tasks
extension. Upside: zero dependencies. Downside: it tracks only the subset of the
spec this system uses, and swapping in the real SDK later is a rewrite of one
package. Verified against a loopback server and by driving `cmd/testplan-mcp` as
a real subprocess.

**The Anthropic adapter has not run against the live API.** It is exercised
against `httptest` — tool-use translation both ways, `Retry-After` honouring,
non-retryable 4xx, retry exhaustion. The wire format is written from the
documented Messages API shape; first contact with production may need a fix.

## Not yet built

Jira publication and the scenario-ID → issue-key ledger (designed, not coded);
the SQLite long-term store, so the blob-SHA analysis cache does not yet persist
between runs; Python analysis via tree-sitter; the MRTR `input_required` flow
for approval over MCP (the phase exists and halts, but the protocol round trip
is not wired).

Of these, the **analysis cache is the highest-value next piece** — it is the
largest cost lever in the system and the seam for it already exists.
