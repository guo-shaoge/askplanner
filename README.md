# askplanner

An AI assistant for TiDB query optimizer questions.

The current runtime uses Codex CLI as the primary agent engine. askplanner's domain value comes from a generated system prompt, a curated skills corpus, a curated TiDB SQL tuning docs overlay, and access to the local TiDB source tree.

## How It Works

1. A user asks a TiDB optimizer question in the local REPL or in Lark.
2. askplanner loads the domain prompt from `bin/printprompt` or falls back to the in-process prompt builder.
3. The prompt is normalized for Codex CLI. askplanner-specific tool references such as `read_file` and `search_docs` are translated into shell-based workspace exploration rules.
4. askplanner starts a new Codex session with `codex exec` or resumes an existing one with `codex exec resume`.
5. Codex reads local skills, local TiDB docs, and local TiDB source code from the workspace and returns the answer.
6. askplanner persists the conversation's `session_id` in `.askplanner/codex_sessions.json` so later turns can reuse the same Codex thread.

The runtime is intentionally simple:
- No custom MCP tool bridge yet.
- No in-process LLM API client for the main runtime path.
- No askplanner-managed tool loop on the hot path.

## Architecture

### Runtime Path

- `cmd/askplanner` is the local REPL
- `cmd/larkbot` is the Feishu/Lark websocket bot
- `internal/codex` is the active runtime
- `cmd/printprompt` prints the askplanner domain prompt

### Core Principle

Codex CLI is the agent runtime.

askplanner is the relay and domain-context layer:
- it prepares the TiDB tuning prompt
- it normalizes that prompt for Codex
- it manages session reuse
- it wires user transport to Codex CLI

### Main Packages

| Package | Role |
|--------|------|
| `internal/codex` | Codex session runtime, prompt normalization, process execution, session store |
| `internal/askplanner` | Canonical system prompt assembly and legacy agent/tool code |
| `internal/askplanner/tools` | Skills index, docs overlay, and legacy tool implementations |
| `internal/askplanner/config` | Environment-driven config for prompt building and Codex runtime |
| `cmd/printprompt` | Prints raw or normalized system prompt |

### Session Flow

For a new conversation:

```text
question
  -> load prompt
  -> normalize prompt
  -> codex exec
  -> capture thread.started.thread_id
  -> store session_id
  -> return final answer
```

For a later turn in the same conversation:

```text
question
  -> lookup session_id
  -> codex exec resume
  -> update session metadata
  -> return final answer
```

## Domain Prompt

`cmd/printprompt` is the canonical way to inspect the askplanner system prompt.

Raw prompt:

```bash
./bin/printprompt
```

Codex-normalized prompt:

```bash
./bin/printprompt --normalized
```

The raw prompt is still generated from:
- `internal/askplanner/agent.go`
- `internal/askplanner/tools/skills.go`
- `internal/askplanner/tools/docsindex.go`

The normalized prompt adds Codex-specific guidance so the runtime can ignore askplanner's legacy custom tool names and rely on shell-based exploration instead.

## Skills and Docs

askplanner still uses the same domain assets:

- `contrib/agent-rules/skills/tidb-query-tuning/references/`
- `contrib/tidb/`
- `contrib/tidb-docs/`
- `prompts/tidb-query-tuning-official-docs/`

The key distinction is that the current runtime does not expose `read_file`, `search_code`, `list_dir`, `list_skills`, `read_skill`, or `search_docs` as live model tools. Instead, the normalized prompt instructs Codex to inspect those assets directly through the local workspace.

## Prerequisites

- Go 1.23+
- Codex CLI installed and authenticated via `codex login`
- TiDB source code available at `contrib/tidb/`
- TiDB docs available at `contrib/tidb-docs/` for the docs overlay
- Skills repo available at `contrib/agent-rules/` (git submodule)
- For Lark bot: a Feishu/Lark app with websocket event subscription enabled

## Quick Start

```bash
git clone https://github.com/guo-shaoge/askplanner.git
cd askplanner
git submodule update --init --recursive

codex login

go build -o bin/askplanner_cli ./cmd/askplanner
go build -o bin/askplanner_lark ./cmd/larkbot
go build -o bin/printprompt ./cmd/printprompt

./bin/askplanner_cli
```

The REPL supports:
- regular questions
- `reset` to drop the local Codex session
- `quit` / `exit`

## Lark Bot

Build:

```bash
go build -o bin/askplanner_lark ./cmd/larkbot
```

Run:

```bash
FEISHU_APP_ID="cli_xxxx" \
FEISHU_APP_SECRET="xxxx" \
./bin/askplanner_lark
```

The bot computes a conversation key from:
- `thread_id` when present
- otherwise `chat_id + sender_id`
- otherwise a message-level fallback

That conversation key maps to a Codex `session_id` in the local session store.

## Configuration

The main runtime is now driven by Codex-related environment variables.

| Env Var | Default | Description |
|--------|---------|-------------|
| `CODEX_BIN` | `codex` | Codex CLI binary |
| `CODEX_MODEL` | `gpt-5.3-codex` | Codex model |
| `CODEX_REASONING_EFFORT` | `medium` | Reasoning effort passed via `model_reasoning_effort` |
| `CODEX_SANDBOX` | `read-only` | Sandbox mode for `codex exec` |
| `CODEX_PROJECT_ROOT` | `.` | Working root for Codex |
| `CODEX_PROMPT_COMMAND` | `bin/printprompt` | Prompt command, supports args such as `bin/printprompt --normalized` |
| `CODEX_SESSION_STORE` | `.askplanner/codex_sessions.json` | Session store path |
| `CODEX_MAX_TURNS` | `30` | Max turns before forcing a new Codex session |
| `CODEX_SESSION_TTL_HOURS` | `24` | Session TTL |
| `CODEX_TIMEOUT_SEC` | `120` | Timeout per Codex subprocess |
| `SKILLS_DIR` | `contrib/agent-rules/skills/tidb-query-tuning/references` | Skills path |
| `TIDB_CODE_DIR` | `contrib/tidb` | TiDB source path |
| `TIDB_DOCS_DIR` | `contrib/tidb-docs` | TiDB docs path |
| `DOCS_OVERLAY_DIR` | `prompts/tidb-query-tuning-official-docs` | Curated docs overlay |

Lark-specific variables:

| Env Var | Required | Description |
|--------|----------|-------------|
| `FEISHU_APP_ID` | Yes | Feishu app ID |
| `FEISHU_APP_SECRET` | Yes | Feishu app secret |

## Build and Verify

```bash
go build -o bin/askplanner_cli ./cmd/askplanner
go build -o bin/askplanner_lark ./cmd/larkbot
go build -o bin/printprompt ./cmd/printprompt
go test ./...
```

## Project Structure

```text
askplanner/
├── cmd/askplanner/main.go
├── cmd/larkbot/main.go
├── cmd/printprompt/main.go
├── internal/codex/
│   ├── prompt_normalizer.go
│   ├── prompt_source.go
│   ├── responder.go
│   ├── runner.go
│   └── session_store.go
├── internal/askplanner/
│   ├── agent.go
│   ├── config/config.go
│   ├── llmprovider/
│   ├── tools/
│   └── util/
├── contrib/
│   ├── agent-rules/
│   ├── tidb/
│   └── tidb-docs/
├── prompts/
├── .askplanner/                        # runtime session state (gitignored)
├── README.md
└── AGENTS.md
```

## Legacy Notes

The old Kimi/Moonshot provider code still exists under `internal/askplanner/llmprovider/`, and the old askplanner tool implementations still exist under `internal/askplanner/tools/`.

Those packages are no longer the primary runtime path for `cmd/askplanner` and `cmd/larkbot`, but they remain important because:
- they are still the canonical source for the generated domain prompt
- they preserve the previous architecture for reference
- they may be reused later if askplanner adds an MCP bridge or another backend
