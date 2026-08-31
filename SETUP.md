# Setup and testing guide

Windows-first, because that is where this is being run. Bash equivalents are
noted where the syntax differs.

Work through the levels in order. Each one proves something the next depends on,
so if level 2 fails you already know levels 0 and 1 were sound.

---

## Prerequisites

| Need | Check | Notes |
|---|---|---|
| Go 1.24+ | `go version` | Required. `go.mod` declares 1.24. |
| git | `git --version` | Only for cloning this project. |
| A model backend | see level 1 | Anthropic, **any OpenAI-compatible endpoint**, or a **local model (free)**. |
| GitHub PAT | see level 3 | Needed for the MCP path. Private repos need `repo` scope; public repos work with a token that has no scopes. |

No other dependencies. The project has **zero third-party Go modules**, so
`go build` never contacts a module proxy.

### Environment variable syntax

This trips people up constantly. Pick the one matching your shell:

```cmd
:: Windows cmd.exe  — no quotes; cmd would include them in the value
set ANTHROPIC_API_KEY=sk-ant-...
```

```powershell
# Windows PowerShell — quotes required
$env:ANTHROPIC_API_KEY="sk-ant-..."
```

```bash
# macOS / Linux
export ANTHROPIC_API_KEY=sk-ant-...
```

Check a variable took: `echo %ANTHROPIC_API_KEY%` (cmd),
`$env:ANTHROPIC_API_KEY` (PowerShell), `echo $ANTHROPIC_API_KEY` (bash).

Variables set with `set` last only for that terminal window. Use
*System Properties → Environment Variables* to persist them, or put them in a
`.bat` you run first.

---

## Level 0 — prove the build is sound (no credentials)

```cmd
cd testplan-agent
go build ./...
go test ./...
```

Expect every package `ok`, 134 tests. This exercises the whole control layer —
guards, retry, circuit breaker, MCP client and server against each other,
the Anthropic wire format against a local HTTP stub — with no network and no
credentials.

Then run the pipeline against the bundled fixture:

```cmd
go run ./cmd/testplan -source testdata\fixtures\paymentsvc -name paymentsvc -v
```

You should see six phases complete and a test plan print to stdout. Scenarios
will be **placeholders** — no API key yet — and the report says so in its Gaps
section. That is correct behaviour, not a failure.

---

## Level 1 — real scenarios (pick any backend)

The vendor is a configuration choice. `internal/llm.Provider` is the seam, and
two adapters ship: Anthropic, and anything speaking the OpenAI chat-completions
format — which includes a model running on your own machine.

Selection is by environment, first match wins:

| Variables | Backend |
|---|---|
| `ANTHROPIC_API_KEY` | Anthropic |
| `OPENAI_API_KEY` (+ optional `OPENAI_BASE_URL`) | OpenAI, or any compatible host |
| `OPENAI_BASE_URL` alone | A local runtime that needs no key |
| none | Deterministic fallbacks; placeholders, no cost |

### Option A — a local model, no API cost at all

**Ollama is a separate program and must be installed first.** `ollama` is not a
Go command; if the shell says *"the term 'ollama' is not recognized"*, it is not
installed yet.

1. Download the installer from [ollama.com/download](https://ollama.com/download)
   and run `OllamaSetup.exe`. It installs into your user account and needs no
   administrator rights.
2. **Close and reopen your terminal** so the new PATH is picked up. This is the
   most common reason the command is still not found after installing.
3. Confirm: `ollama --version`

On Windows the installer leaves Ollama running in the background, so
**`ollama serve` is not needed** — it is already listening on port 11434.

```cmd
ollama pull qwen2.5-coder:7b

set OPENAI_BASE_URL=http://localhost:11434/v1
set OPENAI_MODEL=qwen2.5-coder:7b
go run ./cmd/testplan -source testdata\fixtures\paymentsvc -v -out plan.md
```

Requirements: Windows 10 22H2 or newer, ~5 GB of disk for a 7B model, and
enough memory to hold it. Without a GPU it runs on CPU — correct, but slow
enough that a large repository is impractical. Check the server is up with
`curl http://localhost:11434/v1/models` before running the pipeline.

Tool calling is the hard requirement — every agent emits its findings through a
schema-constrained tool call, so a model without it produces empty phases.
Qwen2.5-Coder, Llama 3.1+ and Mistral all support it.

### Option B — OpenAI

```cmd
set OPENAI_API_KEY=sk-...
set OPENAI_MODEL=gpt-4o-mini
```

Optionally route tiers separately with `OPENAI_MODEL_FAST`,
`OPENAI_MODEL_BALANCED`, `OPENAI_MODEL_STRONG`. On a local runtime one model for
all three is usually right — there is no cheap tier when the marginal cost is
zero.

### Option C — Anthropic

```cmd
set ANTHROPIC_API_KEY=sk-ant-...
```

[console.anthropic.com](https://console.anthropic.com) → credits under Settings
→ Billing → key under Settings → API Keys.

### Why a subscription does not work here

Claude Pro/Max and ChatGPT Plus authorise **their own first-party clients** —
claude.ai, Claude Code, the Codex CLI. This project is a third-party Go binary
you built; neither vendor lets an arbitrary program draw on subscription quota.
That is a billing boundary, not a limitation of this design — which is exactly
why the provider is swappable, and why option A costs nothing.

---

## Level 2 — a real repository, still local

```cmd
git clone --depth 1 https://github.com/gin-gonic/gin
go run ./cmd/testplan -source .\gin\binding -name gin/binding ^
  -link-base https://github.com/gin-gonic/gin/blob/master/binding ^
  -v -checkpoints .\cp -out plan.md
```

`gin\binding` is 30 files, 17 analysed — about 21 model calls. The full `gin`
repo is 58 analysed and roughly 85 calls, so start with the subdirectory.

This level exists to separate two questions: *does the pipeline produce a good
plan* and *does the MCP transport work*. Answer the first here.

---

## Level 3 — GitHub over MCP (the intended path, no clone)

### Install GitHub's MCP server

It is a Go binary, so your existing toolchain builds it. The old
`npx @modelcontextprotocol/server-github` package is archived — do not use it.

```cmd
go install github.com/github/github-mcp-server/cmd/github-mcp-server@latest
```

The binary lands in `%USERPROFILE%\go\bin`. If that is not on your PATH, either
add it or reference the binary directly in the command below.

### Configure

```cmd
set ANTHROPIC_API_KEY=sk-ant-...
set GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
set GITHUB_MCP_COMMAND=github-mcp-server stdio
```

Note the token variable is `GITHUB_PERSONAL_ACCESS_TOKEN`, not `GITHUB_TOKEN`.
The server inherits this process's environment, so setting it in the same
terminal is enough.

If the binary is not on PATH:

```cmd
set GITHUB_MCP_COMMAND=%USERPROFILE%\go\bin\github-mcp-server.exe stdio
```

### Preflight — always run this first

```cmd
go run ./cmd/testplan -github https://github.com/gin-gonic/gin -mcp-check
```

It connects, negotiates the protocol, lists the catalogue, resolves which tools
to use, then fetches the tree and one real file. Takes seconds. Expected output:

```
connected     server responded to server/discover on protocol 2026-07-28
repository    gin-gonic/gin
catalogue     N tools
              - get_repository_tree
              - get_file_contents
              ...
resolved      tree="get_repository_tree" file="get_file_contents"
tree          130 files
read          BENCHMARKS.md (... bytes)

ok — this server can drive the pipeline.
```

If that prints, the full run will work. If it fails, you have a precise error
instead of a mysterious mid-pipeline crash.

### Run

```cmd
go run ./cmd/testplan -github https://github.com/gin-gonic/gin -v -out plan.md
```

Source links are derived from the URL automatically, so `-link-base` is not
needed here. A branch in the URL is honoured:
`https://github.com/owner/repo/tree/my-branch`.

---

## Level 4 — expose the pipeline as an MCP server

```cmd
go run ./cmd/testplan-mcp
```

Speaks JSON-RPC on stdio. `generate_test_plan` returns a task handle
immediately; poll `tasks/get` for the finished document. Useful for showing the
integration works in both directions.

---

## Every flag

| Flag | Default | What it does |
|---|---|---|
| `-source` | `.` | Local directory to analyse |
| `-github` | — | GitHub URL, SSH remote, or `owner/repo`. Overrides `-source` |
| `-ref` | — | Branch, tag or commit; overrides a branch found in the URL |
| `-mcp-command` | `$GITHUB_MCP_COMMAND` | Command that starts the GitHub MCP server |
| `-mcp-check` | off | Preflight only: connect, report, exit |
| `-mcp-tool-tree` | auto | Override the remote tree-listing tool |
| `-mcp-tool-file` | auto | Override the remote file-reading tool |
| `-name` | derived | Repository name shown in the report |
| `-out` | stdout | Write the report to a file |
| `-link-base` | derived for `-github` | Base URL for source links |
| `-checkpoints` | off | Directory for per-phase blackboard snapshots |
| `-max-files` | 400 | Refuse repositories above this size |
| `-concurrency` | 4 | Analyst fan-out width |
| `-revision-rounds` | 2 | Maximum Author ⇄ Critic rounds |
| `-v` | off | Log phase progress to stderr |

Environment:

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Use the Anthropic backend |
| `ANTHROPIC_BASE_URL` | Override the Anthropic endpoint |
| `OPENAI_API_KEY` | Use an OpenAI-compatible backend |
| `OPENAI_BASE_URL` | Endpoint for that backend (e.g. `http://localhost:11434/v1`) |
| `OPENAI_MODEL` | Model for all tiers |
| `OPENAI_MODEL_FAST` / `_BALANCED` / `_STRONG` | Per-tier overrides |
| `GITHUB_MCP_COMMAND` | Command that starts the GitHub MCP server |
| `GITHUB_PERSONAL_ACCESS_TOKEN` | Read by that server, not by this program |

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `no such directory. Check the path exists` | `-source` points nowhere | Clone first, or check the path. Windows paths need `\` |
| `'$env:VAR' is not recognized` | PowerShell syntax in cmd.exe | Use `set VAR=value`, no quotes |
| Scenarios are placeholders, Gaps says "no model provider" | No backend variable set in this shell | Run with `-v`; the first line names the backend it chose |
| Phases complete but emit nothing, with a local model | The model does not support tool calling | Use a tool-calling model: Qwen2.5-Coder, Llama 3.1+, Mistral |
| `'ollama' is not recognized` | Ollama is not installed, or the terminal predates the install | Install from ollama.com/download, then **open a new terminal** |
| `connection refused` on port 11434 | Ollama is not running | On Windows it starts automatically; check with `curl http://localhost:11434/v1/models` |
| `no model configured for tier` | `OPENAI_MODEL` unset and no per-tier override | Set `OPENAI_MODEL` |
| `-github needs a GitHub MCP server` | `GITHUB_MCP_COMMAND` unset | See level 3. The error prints the exact commands |
| `could not start the MCP server` | Binary not on PATH | Use the full path to `github-mcp-server.exe` |
| `server offers no tool for listing a repository tree` | Unfamiliar server vocabulary | The error lists every tool it *does* offer; pass `-mcp-tool-tree` / `-mcp-tool-file` |
| `repository has N analysable files, above the ceiling of 400` | Repository too large | Raise `-max-files`, or point `-source` at a subdirectory |
| `credit balance is too low` | No Console credits | Add credits at console.anthropic.com. A Pro/Max plan is separate |
| Run dies partway | Anything | With `-checkpoints`, per-phase snapshots survive in that directory |

---

## What is genuinely unverified

Two things have never met production, and you should know which they are before
demonstrating:

**The MCP path against GitHub's real server.** It has been driven end to end over
the real transport against a stand-in server backed by a local checkout
(`internal/mcpx/testdata/fake_github_server.go.txt`), covering `server/discover`,
`tools/list`, tool-name resolution, base64 blob decoding and all seven phases.
What is untested is authentication and GitHub's exact argument names.
`-mcp-check` exists to answer that in seconds.

**Both model adapters against their live APIs.** Each is tested against a local
HTTP stub covering tool-call translation in both directions, `Retry-After`
honouring, non-retryable 4xx and retry exhaustion. The OpenAI-compatible adapter
has additionally driven the full pipeline end to end against a stub endpoint,
producing 34 scenarios across 17 components of `gin/binding` with working source
links. Level 1 against the three-file fixture is the cheapest way to confirm
either one for real.

One known calibration issue: risk *levels* are miscalibrated on large
repositories — 12 of 58 gin components come out "critical" because the
thresholds were tuned against a three-file fixture. The *ordering* is sound.
Worth mentioning as known rather than being asked about it.
