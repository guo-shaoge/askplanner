# askplanner — AI Agent Onboarding

## What This Is

askplanner is a TiDB query optimizer Q&A assistant.

The current architecture uses Codex CLI as the primary runtime. askplanner itself is now mainly:
- a domain prompt generator
- a Codex prompt normalizer
- a transport relay for CLI and Lark
- a local session manager for Codex threads

The runtime no longer depends on the old in-process Kimi tool-calling loop for normal user requests.

## Entry Points

- `cmd/askplanner` — local REPL backed by Codex CLI
- `cmd/larkbot` — Feishu/Lark websocket bot backed by Codex CLI
- `cmd/printprompt` — prints the askplanner system prompt

Important:
- `./bin/printprompt` prints the raw askplanner system prompt
- `./bin/printprompt --normalized` prints the Codex-normalized prompt

## Current Runtime Flow

High-level flow:

1. Load the askplanner prompt from `bin/printprompt` or the fallback prompt builder
2. Normalize the prompt for Codex CLI
3. Look up the conversation's stored `session_id`
4. Run `codex exec` for a new conversation or `codex exec resume` for an existing one
5. Capture the final answer and persist updated session metadata
6. Return plain text to the REPL or Lark

The active runtime is in `internal/codex/`.

## Internal Package Design

### Active Runtime

- `internal/codex/prompt_source.go`
  Loads the bootstrap prompt from `CODEX_PROMPT_COMMAND` and supports command arguments.
  Falls back to the in-process prompt builder if the command is unavailable.

- `internal/codex/prompt_normalizer.go`
  Adapts the askplanner prompt for Codex CLI.
  This is the bridge that translates references to legacy askplanner tools into shell/workspace behavior.

- `internal/codex/runner.go`
  Spawns `codex exec` and `codex exec resume`.
  Parses `thread.started.thread_id` from JSONL output as the Codex session identifier.

- `internal/codex/session_store.go`
  Persists `conversation_key -> session_id` mappings in `.askplanner/codex_sessions.json`.

- `internal/codex/responder.go`
  Orchestrates prompt loading, session reuse, fallback to new sessions, timeout handling, and per-conversation history summarization.

### Canonical Prompt Source

- `internal/askplanner/agent.go`
  Still owns the canonical askplanner system prompt assembly.

- `internal/askplanner/tools/skills.go`
  Still builds the compact skills index used in the prompt.

- `internal/askplanner/tools/docsindex.go`
  Still builds the curated official docs overlay used in the prompt.

### Legacy Runtime Packages

- `internal/askplanner/llmprovider/`
- `internal/askplanner/tools/registry.go`
- `internal/askplanner/tools/readfile.go`
- `internal/askplanner/tools/searchcode.go`
- `internal/askplanner/tools/searchdocs.go`
- `internal/askplanner/tools/listdir.go`
- `internal/askplanner/tools/toolskills.go`
- `internal/askplanner/util/sandbox.go`

These still matter for reference and prompt generation, but they are not on the hot path for `cmd/askplanner` or `cmd/larkbot`.

Do not assume the model is currently calling these as live tools.

## Key Types

| Type | File | Role |
|------|------|------|
| `codex.Responder` | `internal/codex/responder.go` | Main runtime entrypoint used by CLI and Lark |
| `codex.Runner` | `internal/codex/runner.go` | Codex CLI subprocess wrapper |
| `codex.FileSessionStore` | `internal/codex/session_store.go` | Persistent session mapping store |
| `config.Config` | `internal/askplanner/config/config.go` | Runtime config, including Codex settings |
| `askplanner.Agent` | `internal/askplanner/agent.go` | Canonical prompt builder, not the primary runtime |

## Prompt Model

There are now two prompt layers:

1. Raw askplanner system prompt
   Generated from:
   - the TiDB tuning system prompt
   - the skills index
   - the official docs overlay

2. Normalized Codex prompt
   Adds:
   - shell-based replacements for legacy askplanner tool names
   - read-only runtime assumptions
   - language behavior for replies

Important:
- `NormalizePrompt()` is idempotent
- `CODEX_PROMPT_COMMAND` may point to `bin/printprompt --normalized`
- the responder still calls `NormalizePrompt()` defensively

## Session Model

Sessions are keyed by conversation.

### CLI

The REPL uses a fixed local conversation key and supports:
- `reset` to drop the stored session
- `quit` / `exit` to leave the REPL

### Lark

`cmd/larkbot` builds the conversation key in this order:
- `thread_id`
- otherwise `chat_id + sender_id`
- otherwise a message-level fallback

This prevents unrelated group-chat users from sharing the same Codex thread by default.

### Session Expiry

The responder forces a new Codex session when:
- the prompt hash changes
- the configured max-turn limit is reached
- the configured session TTL expires
- a resume call fails

## Build and Verify

```bash
go build -o bin/askplanner_cli ./cmd/askplanner
go build -o bin/askplanner_lark ./cmd/larkbot
go build -o bin/printprompt ./cmd/printprompt
go test ./...
```

## Configuration

Main runtime variables:

- `CODEX_BIN`
- `CODEX_MODEL`
- `CODEX_REASONING_EFFORT`
- `CODEX_SANDBOX`
- `CODEX_PROJECT_ROOT`
- `CODEX_PROMPT_COMMAND`
- `CODEX_SESSION_STORE`
- `CODEX_MAX_TURNS`
- `CODEX_SESSION_TTL_HOURS`
- `CODEX_TIMEOUT_SEC`

Prompt-source variables:

- `SKILLS_DIR`
- `TIDB_CODE_DIR`
- `TIDB_DOCS_DIR`
- `DOCS_OVERLAY_DIR`

Lark bot variables:

- `FEISHU_APP_ID`
- `FEISHU_APP_SECRET`

## Current Constraints

- The runtime does not yet expose askplanner's legacy tools as MCP tools to Codex.
- The current approach relies on Codex reading the local workspace directly.
- `internal/askplanner/util/sandbox.go` still exists, but current Codex runtime safety comes primarily from the Codex sandbox mode, not from the old askplanner tool sandbox.
- If you want stricter parity with the old read allowlist, the next architectural step is a dedicated workspace projection or an MCP bridge.

## How To Change Behavior

### Change the user-facing domain prompt

Edit:
- `internal/askplanner/agent.go`
- `internal/askplanner/tools/skills.go`
- `internal/askplanner/tools/docsindex.go`

Verify with:

```bash
./bin/printprompt
./bin/printprompt --normalized
```

### Change Codex-specific runtime instructions

Edit:
- `internal/codex/prompt_normalizer.go`

### Change how Codex is invoked

Edit:
- `internal/codex/runner.go`

### Change session semantics

Edit:
- `internal/codex/session_store.go`
- `internal/codex/responder.go`
- `cmd/larkbot/main.go`

## Practical Notes

- Prefer reading `cmd/printprompt/main.go` and `internal/codex/*` before editing runtime behavior.
- Do not assume README-era Kimi instructions are still current.
- If you are changing docs, keep README and this AGENTS file aligned.
- If you are changing prompt behavior, verify both raw and normalized prompt outputs.
