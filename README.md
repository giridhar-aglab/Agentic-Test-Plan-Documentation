# testplan-agent

A multi-agent system in Go that reads a repository and produces a test planning
document. Architecture doc: `claude/architecture.md` in the project.

**Status: all seven phases implemented. 198 tests, `go vet` clean, zero dependencies.**

Setup, troubleshooting and a level-by-level verification ladder:
**[SETUP.md](SETUP.md)**. On Windows, `run-demo.bat` walks the same ladder and
tells you which credentials are missing.

## Run it

```bash
go test ./...                                                     # 198 tests

# Local directory, no API key — every phase runs with deterministic fallbacks
go run ./cmd/testplan -source testdata/fixtures/paymentsvc -name paymentsvc -v

# With a model. The backend is a configuration choice — see SETUP.md.
#   ANTHROPIC_API_KEY   Anthropic
#   OPENAI_API_KEY      OpenAI or any compatible host
#   OPENAI_BASE_URL     a local runtime (Ollama, LM Studio, vLLM) — no API cost
OPENAI_BASE_URL=http://localhost:11434/v1 OPENAI_MODEL=qwen2.5-coder:7b \
  go run ./cmd/testplan -source /path/to/repo -out plan.md

# GitHub over MCP — no clone. This is the intended path.
#
# GitHub's official MCP server is a Go binary, so the Go toolchain you already
# have builds it. (The old npx @modelcontextprotocol/server-github is archived.)
go install github.com/github/github-mcp-server/cmd/github-mcp-server@latest

export ANTHROPIC_API_KEY=sk-ant-...
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
export GITHUB_MCP_COMMAND="github-mcp-server stdio"
export GITHUB_TOOLSETS="repos,git"   # optional: enables the one-call tree tool

# Preflight: connect, list the catalogue, fetch one file, exit. Run this first.
go run ./cmd/testplan -github https://github.com/owner/repo -mcp-check

# Writes reports/<repo>_<sha>_<timestamp>.md and .html — runs never overwrite
go run ./cmd/testplan -github https://github.com/owner/repo
go run ./cmd/testplan -github https://github.com/owner/repo -format html

# Windows cmd.exe uses set, and the value must NOT be quoted:
#   set GITHUB_MCP_COMMAND=github-mcp-server stdio
# Windows PowerShell:
#   $env:GITHUB_MCP_COMMAND="github-mcp-server stdio"

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

**Structured output is a tool call.** `analysis_emit_component`, `scenario_emit`
and `review_emit` are schema-constrained, so malformed output becomes a
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

**A rate limit belongs to the account, not to the request that met it.**
Retrying one 429 correctly still lost agents: four workers each backed off
independently, woke together, and the requests waiting on the limit were what
re-triggered it. A 429 now pauses every concurrent caller through that provider
until the stated wait elapses, so the quota is shared rather than raced for.

**Prioritisation that selects most of a codebase is a list, not a ranking.**
The risk bands were absolute numbers tuned on a three-file fixture. On a real
repository they called 42 of 58 components critical or high — so the author
phase ran 42 strong-tier agents, and a reviewer opening the plan learned
nothing about where to start. Levels are now relative to the repository's own
distribution (top 10% critical, next 20% high), which puts the same 58
components at 6 critical and 14 high. The *score* stays absolute so two runs
can still be diffed; only the band is relative, and equal scores never straddle
a boundary.

**A phase that prints nothing reads as a hang.** Authoring ran one risk at a
time on the strongest tier and logged only after every risk had finished, so a
repository with twenty prioritised risks showed one line and then minutes of
silence. Risks are independent, so the Author now fans out the way the Analyst
does and reports each risk as it completes. The revision round is worse than
slow when it is untargeted: one blocking finding used to trigger a second full
pass over the whole register, redoing work nothing had invalidated. `Revise`
now re-authors only the risks a finding actually names.

**A budget is only as good as the thing it bounds.** Every guard in this
system caught its symptom correctly and none of them named a cause: "reached 8
iterations", "used 81108 tokens of 60000". What the numbers meant was that a
read was rendering 108 KB — a 1500-*line* cap is not a size cap once
line-number prefixes and dense code are involved — which spilled, was digested,
and got fetched back in five pieces that were re-sent on every subsequent turn.
Reads are now capped in bytes, so "an analyst read never spills" is exact rather
than estimated, and two tests tie the read cap to the spill threshold and to the
phase's token budget. Guards report; they do not diagnose. Anything a guard is
meant to catch should also be impossible to reach by ordinary operation.

**Silence is the worst failure mode.** A tool that cannot read what a server
sent must say so, never return an empty string: empty is indistinguishable from
"the file really is empty", and a model handed an empty file simply reads it
again. The MCP client modelled only `{"type":"text"}` content blocks, while
GitHub returns file contents as an embedded resource — so every file read over
MCP came back empty and every analyst spent its whole budget re-reading. All
specification block types are now modelled, an unrecognised one is an error
naming the types that arrived, and an empty payload fails loudly at both the
source and the tool.

**Every instruction the model receives names a tool it can actually call.**
This sounds trivial and was the source of three separate production failures:
`repo.read_file` told the model to "call repo.tree" in agents that register no
such tool; the prompts named `analysis.emit_component` while the OpenAI adapter
offered `analysis_emit_component`; and an oversized tool result was replaced by
a digest ending "fetch the rest by handle" when nothing in the system could
fetch a handle. Each one produced the same signature — a model with no legal
move, repeating itself until a guard stopped it. Tests now assert that prompt
text, tool descriptions and digests only name tools present in the registry, and
that no tool name needs rewriting on any backend.

**A component is never dropped for the model's mistakes.** The parser runs
before the model and establishes symbols, complexity and error paths; the model
only enriches that. So when a model spends its whole iteration budget without
emitting, the parser's findings are still recorded — at 0.3 confidence, with no
claimed responsibility and a gap noted in the report. Dropping the file instead
would take it out of the risk register entirely, which is a worse answer than an
honestly shallow one.

**Structural defects never reach the reviewer.** `report_validate` asserts that
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

**The MCP path has not met a live GitHub MCP server.** It has been driven end to
end over the real transport against a stand-in server backed by a local checkout
(`internal/mcpx/fakegh` — `go build -o fakegh ./internal/mcpx/fakegh` and point
`-mcp-command` at it), which exercises `server/discover`,
`tools/list`, tool-name resolution, base64 blob decoding and the full seven-phase
pipeline. That stand-in now advertises GitHub's own parameter vocabulary and
*rejects* any argument it did not declare, which is what forced the client to
read each tool's schema rather than assume one: GitHub's tree tool is built on
the Git API and names its reference `tree_sha`, while its file tool calls the
same idea `ref`. What remains untested is authentication. `-mcp-check` is the
preflight for exactly that, and it now prints the parameters each resolved tool
accepts.

A second gap showed up on a live catalogue: GitHub ships `get_repository_tree`
in the `git` toolset, which is off by default, so there is often no tree tool at
all. Name resolution then fell through to a hint word of "list" and picked
`list_branches` — a call that succeeded, returned well-formed JSON, and meant
nothing. Three changes came out of that. A hint word now has to mean what the
role needs, and a name containing `branch`, `commit`, `tag`, `release` and the
rest is never a candidate. A decoded listing whose entries carry no paths is
rejected rather than believed. And a missing or unusable tree tool is no longer
fatal: the listing is built by walking directories through the file tool, one
call per directory, which is slower but works in the default configuration.
Truncation is the one tree failure that still stops the run — the server saw the
whole repository and said it did not send all of it, so walking around it would
produce the same silent half-plan by a slower route.

**The wire formats were written from documentation, and production found one
gap.** Both adapters are exercised against `httptest` — tool-call translation in
both directions, `Retry-After` honouring, non-retryable 4xx, retry exhaustion —
and the OpenAI-compatible one has driven the full pipeline against a stub
endpoint, producing 34 scenarios across 17 components with working source links.

First contact with the real OpenAI API rejected every request: it enforces
`^[a-zA-Z0-9_-]+$` on function names, and every tool here is named with a dot
(`repo_read_file`, `analysis_emit_component`). Anthropic accepts dots, so the
naming was never questioned. The fix is a name mapping inside the adapter
(`toolNameMapping`) rather than a rename of the tools, because renaming would
leak one vendor's constraint into the whole system; collisions get distinct wire
names and replayed history uses the same translation, so the model never sees a
name it was not offered. The lesson worth keeping is about the *test*, not the
bug: the stub accepted any name, so it agreed with the adapter instead of
checking it. It now enforces the pattern and returns the same 400 the real API
does, and the tests drive it with the system's actual tool names
(`TestEverySystemToolNameSurvivesTheWireNamePattern`). The Anthropic adapter has
still not met its live API.

## Not yet built

Jira publication and the scenario-ID → issue-key ledger (designed, not coded);
the SQLite long-term store, so the blob-SHA analysis cache does not yet persist
between runs; Python analysis via tree-sitter; the MRTR `input_required` flow
for approval over MCP (the phase exists and halts, but the protocol round trip
is not wired).

Of these, the **analysis cache is the highest-value next piece** — it is the
largest cost lever in the system and the seam for it already exists.
