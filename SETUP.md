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

### Which shell am I in?

Get this wrong and nothing else works. **Visual Studio's default terminal is
PowerShell, not cmd.** The tell is in the error text:

| Error wording | Shell |
|---|---|
| `... is not recognized as the name of a **cmdlet**` | PowerShell |
| `... is not recognized as an internal or external command` | cmd.exe |
| A `>>` continuation prompt after a line ending in `^` | PowerShell (it does not understand `^`) |

Two things differ and both bite:

| | PowerShell | cmd.exe |
|---|---|---|
| Set a variable | ``$env:NAME="value"`` | `set NAME=value` (no quotes) |
| Continue a line | `` ` `` (backtick) | `^` (caret) |

**In PowerShell, `set NAME=value` does not set an environment variable.** `set`
is an alias for `Set-Variable`, so the value never reaches the program. If you
have been using `set` in PowerShell, nothing you set has taken effect — which
shows up as the `!! NO MODEL BACKEND CONFIGURED` banner.

Simplest way to avoid line-continuation problems entirely: **put the whole
command on one line.** It is long, but it always works.

### Environment variable syntax

Pick the one matching your shell:

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

Expect every package `ok`, 198 tests. This exercises the whole control layer —
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

| Variables | Backend | Key prefix |
|---|---|---|
| `ANTHROPIC_API_KEY` | Anthropic | `sk-ant-…` |
| `OPENAI_API_KEY` (+ optional `OPENAI_BASE_URL`) | OpenAI, or any compatible host | `sk-…` (never `sk-ant-`) |
| `OPENAI_BASE_URL` alone | A local runtime that needs no key | — |
| none | Deterministic fallbacks; placeholders, no cost | — |

**The variable must match the key.** Selection is by which variable is set, not
by inspecting the key, so an OpenAI key in `ANTHROPIC_API_KEY` selects the
Anthropic backend and fails with a 401 partway through the run. A warning now
flags a mismatched prefix before any request is made.

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

Every run prints which backend it selected. Look for this line before anything
else — if you see the `!! NO MODEL BACKEND CONFIGURED` banner instead, the
variable did not reach this terminal:

```
model backend: openai-compatible at http://localhost:11434/v1 (model qwen2.5-coder:7b)
```

Requirements: Windows 10 22H2 or newer, ~5 GB of disk for a 7B model, and
enough memory to hold it. Without a GPU it runs on CPU — correct, but slow
enough that a large repository is impractical. Check the server is up with
`curl http://localhost:11434/v1/models` before running the pipeline.

Tool calling is the hard requirement — every agent emits its findings through a
schema-constrained tool call, so a model without it produces empty phases.
Qwen2.5-Coder, Llama 3.1+ and Mistral all support it.

### Option B — OpenAI

```powershell
$env:OPENAI_API_KEY="sk-..."
$env:OPENAI_MODEL="gpt-4o-mini"
$env:ANTHROPIC_API_KEY=""   # whichever is set first wins; clear the one you are not using
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

Model identifiers change faster than this code does. The built-in defaults are a
starting point, not a guarantee — if a run fails with *model not found*, correct
it without rebuilding:

```cmd
:: one model for every tier — cheapest, good for a first run
set ANTHROPIC_MODEL=claude-haiku-4-5-20251001

:: or per tier
set ANTHROPIC_MODEL_FAST=claude-haiku-4-5-20251001
set ANTHROPIC_MODEL_BALANCED=claude-sonnet-5
set ANTHROPIC_MODEL_STRONG=claude-opus-5
```

The first line every run prints names the model it chose, so a wrong identifier
is visible immediately.

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

That module needs Go 1.25 or newer. If yours is older the toolchain downloads
what it needs automatically, unless you have set `GOTOOLCHAIN=local`.

### Make a token

github.com → Settings → Developer settings → Personal access tokens. A
fine-grained token with **Contents: Read-only** on the repositories you want is
enough; a classic token needs only `public_repo` for public repositories. This
program never sees the token — GitHub's server reads it from the environment.

### Configure

**MCP and the model backend are independent.** MCP is how the agent *reads
code*; the model backend is how it *reasons about it*. Neither needs the other.

```powershell
# PowerShell — reading code over MCP is all Level 3 requires
$env:GITHUB_PERSONAL_ACCESS_TOKEN="ghp_..."
$env:GITHUB_MCP_COMMAND="github-mcp-server stdio"

# Optional but worth setting. GitHub ships get_repository_tree in the "git"
# toolset, which is OFF by default. With it, the whole file listing arrives in
# one call; without it, the listing is built by walking directories through
# get_file_contents — one call per directory. Both work.
$env:GITHUB_TOOLSETS="repos,git"
```

```cmd
:: cmd.exe — note the value is NOT quoted
set GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
set GITHUB_MCP_COMMAND=github-mcp-server stdio
```

That alone gives you the survey, the full risk register, validation, traceability
and working source links over MCP — with placeholder scenarios, since nothing is
reasoning about the code yet. `-mcp-check` needs no model key whatsoever.

Add a model backend from Level 1 — `OPENAI_API_KEY`, `OPENAI_BASE_URL` for a
local model, or `ANTHROPIC_API_KEY` — only when you want real scenarios:

```powershell
$env:OPENAI_API_KEY="sk-..."
$env:OPENAI_MODEL="gpt-4o-mini"
$env:ANTHROPIC_API_KEY=""   # whichever is set first wins; clear the one you are not using
```

Note the GitHub variable is `GITHUB_PERSONAL_ACCESS_TOKEN`, not `GITHUB_TOKEN`,
and it is read by *GitHub's server*, never by this program — which is why it
never appears in this codebase. The subprocess inherits this terminal's
environment, so setting it here is enough.

If the binary is not on PATH:

```powershell
$env:GITHUB_MCP_COMMAND="$env:USERPROFILE\go\bin\github-mcp-server.exe stdio"
```

### Preflight — always run this first

```powershell
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
              tree accepts: owner, recursive, repo, tree_sha
              file accepts: owner, path, ref, repo
tree          130 files
read          BENCHMARKS.md (... bytes)

ok — this server can drive the pipeline.
```

If that prints, the full run will work. If it fails, you have a precise error
instead of a mysterious mid-pipeline crash.

### Run

```powershell
go run ./cmd/testplan -github https://github.com/gin-gonic/gin -v -out plan.md
```

gin is 130+ analysable files. To rehearse on something smaller first, point at a
subdirectory of any repo with `-source`, or raise `-max-files` when you are ready
for the whole thing.

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

This is the same pipeline behind an MCP interface, so it reads the same model
variables as the CLI and behaves the same way without them: it runs, and the
scenarios are placeholders. Serving MCP does not itself need a model key.

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
| `-out` | derived | Write to this exact path, overwriting it. **Omit it** to get `reports/<repo>_<commit>_<time>.md` and `.html`, so runs accumulate instead of clobbering each other |
| `-out-dir` | `reports` | Where generated names go when `-out` is not given |
| `-format` | `both` | `md`, `html`, `both`, or `stdout` |
| `-link-base` | derived for `-github` | Base URL for source links |
| `-checkpoints` | off | Directory for per-phase blackboard snapshots |
| `-max-files` | 400 | Refuse repositories above this size |
| `-concurrency` | 4 | Analyst fan-out width |
| `-revision-rounds` | 2 | Maximum Author ⇄ Critic rounds |
| `-v` | **on** | Log phase progress to stderr (progress is on by default) |
| `-quiet` | off | Suppress progress; print only errors and the final path |

Environment:

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Use the Anthropic backend |
| `ANTHROPIC_BASE_URL` | Override the Anthropic endpoint |
| `ANTHROPIC_MODEL` | Model for all tiers |
| `ANTHROPIC_MODEL_FAST` / `_BALANCED` / `_STRONG` | Per-tier overrides |
| `OPENAI_API_KEY` | Use an OpenAI-compatible backend |
| `OPENAI_BASE_URL` | Endpoint for that backend (e.g. `http://localhost:11434/v1`) |
| `OPENAI_MODEL` | Model for all tiers |
| `OPENAI_MODEL_FAST` / `_BALANCED` / `_STRONG` | Per-tier overrides |
| `GITHUB_MCP_COMMAND` | Command that starts the GitHub MCP server |
| `GITHUB_PERSONAL_ACCESS_TOKEN` | Read by that server, not by this program |

The two groups are orthogonal. Model variables affect *what the agent thinks*;
GitHub variables affect *where it reads from*. Set either, both, or neither.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `no such directory. Check the path exists` | `-source` points nowhere | Clone first, or check the path. Windows paths need `\` |
| `'$env:VAR' is not recognized` | PowerShell syntax in cmd.exe | Use `set VAR=value`, no quotes |
| `'-link-base' is not recognized as the name of a cmdlet` | `^` line continuation used in PowerShell | Use a backtick `` ` ``, or put the command on one line |
| Variables set with `set`, but the no-backend banner still appears | `set` in PowerShell does not set environment variables | Use ``$env:NAME="value"`` |
| Scenarios are placeholders, Gaps says "no model provider" | No backend variable set in **this** terminal | Every run prints its backend to stderr. A `!! NO MODEL BACKEND CONFIGURED` banner names exactly what to set. Installing Ollama is not enough — `OPENAI_BASE_URL` must be set too |
| Phases complete but emit nothing, with a local model | The model does not support tool calling | Use a tool-calling model: Qwen2.5-Coder, Llama 3.1+, Mistral |
| `'ollama' is not recognized` | Ollama is not installed, or the terminal predates the install | Install from ollama.com/download, then **open a new terminal** |
| `connection refused` on port 11434 | Ollama is not running | On Windows it starts automatically; check with `curl http://localhost:11434/v1/models` |
| `no model configured for tier` | `OPENAI_MODEL` unset and no per-tier override | Set `OPENAI_MODEL` |
| `model not found` / `invalid model` | A built-in model identifier has gone stale | Set `ANTHROPIC_MODEL` or `OPENAI_MODEL` to a current one |
| `-github needs a GitHub MCP server` | `GITHUB_MCP_COMMAND` unset | See level 3. The error prints the exact commands |
| `could not start the MCP server` | Binary not on PATH | Use the full path to `github-mcp-server.exe` |
| `server offers no tool for reading a file` | Unfamiliar server vocabulary | The error lists every tool it *does* offer; pass `-mcp-tool-file` |
| `decode tree ([{"name":"benchmarks","sha":...` | Fixed in this build | Resolution matched `list_branches` and got branches instead of files. It no longer can, and a missing tree tool now falls back to walking directories |
| `analyst: <file> stopped: reached 8 iterations` on every file | Fixed in this build | The prompts named tools with dots (`analysis.emit_component`) while the OpenAI adapter offered them with underscores, so the model was told to call something absent from its own tool list. Tools are now named with underscores everywhere and a test asserts prompt and tool list agree |
| `analyst: <file> stopped: … [called: repo_read_file×8]` | Fixed in this build | Results over 4 KB were digested and replaced by a handle no tool could resolve, so the model re-read the file instead. Ordinary reads no longer spill, and `context_fetch` resolves a handle when one is issued |
| `[called: repo_read_file×7, code_parse_go×1]` on every file | Fixed in this build | Files read over MCP came back empty: the client understood only `text` content blocks and GitHub returns an embedded `resource`. Every block type is now handled, and an unreadable reply is an error rather than an empty string |
| `used 81108 tokens of 60000 [called: context_fetch×5]` | Fixed in this build | A large file rendered past the spill threshold, was digested, and got fetched back in pieces that were re-sent every turn. Reads are now byte-capped below that threshold, so no fetch is needed |
| The report keeps landing in `plan.md` | `-out plan.md` is on the command line | `-out` means that exact path. Drop it and each run writes `reports/<repo>_<commit>_<time>.md` and `.html` |
| `revision round 1: 1 findings, 1 blocking` then a long pause | Fixed in this build | Authoring was sequential with no per-risk output, and a revision re-ran every risk. It now fans out, prints each risk as it finishes, and revises only what was flagged |
| One banner line and then nothing for minutes | Older build, or `-quiet` | Progress is on by default now. A gin-sized run takes several minutes; the phase and per-file lines are what tell you it is alive |
| Banner names a model you did not configure | Fixed in this build | It reported only the analysis tier. It now lists every role's model whenever they differ |
| `429` on a few risks even with retry | Fixed in this build | Each agent retried independently and they collided again. A rate limit belongs to the account, so one now pauses every concurrent caller through that provider |
| `429: Rate limit reached ... try again in 13.94s` | Fixed earlier | OpenAI states TPM waits in the message body, not the `Retry-After` header. The adapter read only the header, backed off 500 ms, and gave up after 3 attempts. It now honours the stated wait and retries 6 times. Lower `-concurrency` also reduces how often you hit it |
| A risk logs `failed` and then a scenario count | Fixed in this build | The count was a before/after delta on a shared blackboard, so it included other workers' output. It now counts that risk's own scenarios |
| Any `stopped:` line | Diagnostic | The `[called: …]` suffix shows which tools the agent was calling and how often — a histogram dominated by one tool means it never got what it asked for |
| `analyst: <file> stopped: reached 8 iterations` (older build) | Earlier build | The read tool invited the model to page through large files and pointed it at a `repo_tree` it did not have. Both fixed; a component that still fails now keeps its parser-derived analysis instead of vanishing |
| `analyst: <file> fell back to structural analysis only` | Not fatal | The model did not emit for that file, so its symbols and error paths came from the parser alone. The component still appears, at low confidence, and the gap is listed in the report |
| `tree=none — will walk directories` in `-mcp-check` | Normal | GitHub's tree tool is in the `git` toolset, off by default. The run works either way; set `GITHUB_TOOLSETS=repos,git` to make it one call instead of one per directory |
| `repository has N analysable files, above the ceiling of 400` | Repository too large | Raise `-max-files`, or point `-source` at a subdirectory |
| `credit balance is too low` | No Console credits | Add credits at console.anthropic.com. A Pro/Max plan is separate |
| `anthropic: 401 authentication_error: API key is invalid` | An OpenAI key is in `ANTHROPIC_API_KEY` | Clear it and use `OPENAI_API_KEY`. Anthropic keys begin `sk-ant-`; OpenAI keys do not. A warning now names this before the run |
| `openai-compatible: 400: Invalid 'tools[0].function.name'` | Fixed in this build | OpenAI rejects the dots in tool names like `repo_read_file`. The adapter now translates them on the wire and back. If you still see it, you are on an older copy |
| Run dies partway | Anything | With `-checkpoints`, per-phase snapshots survive in that directory |

---

## What is genuinely unverified

Two things have never met production, and you should know which they are before
demonstrating:

**The MCP path against GitHub's real server.** It has been driven end to end over
the real transport against a stand-in server backed by a local checkout
(`internal/mcpx/fakegh`), covering `server/discover`,
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
