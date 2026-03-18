# askplanner — AI Agent Onboarding

## What This Is

An AI agent that answers TiDB query optimizer questions. It uses Kimi (Moonshot AI) as the LLM backend and reads engineer-written skills + TiDB planner source code on demand via tool calling.

There are two entrypoints:
- `cmd/askplanner` — terminal REPL for local interactive use
- `cmd/larkbot` — Feishu/Lark websocket bot that receives chat messages and replies with the same agent

## Build & Verify

```bash
go build -o bin/askplanner ./cmd/askplanner   # build the CLI
go build -o bin/larkbot ./cmd/larkbot         # build the Lark bot
go vet ./...                                   # lint
```

## Architecture

### Single Package Design

All shared logic lives in one package: `internal/askplanner/`. Both `cmd/askplanner/main.go` and `cmd/larkbot/main.go` consume it and only differ in transport wiring.

There is no reason to split into multiple packages at this project's scale. If you add code, put it in `internal/askplanner/`.

### cmd/larkbot Overview

`cmd/larkbot/main.go` wraps the same `askplanner.Agent` used by the CLI in a Feishu/Lark websocket event loop.

High-level flow:
1. Read `FEISHU_APP_ID` and `FEISHU_APP_SECRET`
2. Build the shared agent via `buildAgent()`
3. Start a websocket client with `im.message.receive_v1` handler
4. Extract incoming text message content
5. Call `agent.Answer(...)`
6. Reply to the original message with plain text

Important behavior:
- Empty incoming text falls back to `"Please introduce your capabilities."`
- Tool calls are logged for debugging
- Agent errors are returned to chat as `Agent Error: ...`
- Sandbox, skills index, provider selection, and rate limiting are shared with the CLI path

### Key Types

| Type | File | Role |
|------|------|------|
| `Provider` | provider.go | Interface for any LLM backend (`Complete` + `Name`) |
| `KimiProvider` | kimi.go | Kimi/Moonshot API client with 429 retry |
| `Agent` / `AgentConfig` | agent.go | Orchestrates the LLM-tool loop |
| `Config` | config.go | App configuration from env vars |
| `Tool` | registry.go | Interface for tools (`Name`, `Description`, `Parameters`, `Execute`) |
| `Registry` | registry.go | Holds tools, provides `Definitions()` for LLM and `Execute()` for dispatch |
| `Sandbox` | sandbox.go | Path validation — restricts file access to allowed directory roots |
| `Index` | skills.go | Pre-scanned skill metadata for system prompt |

Note: `Config` (app config) and `AgentConfig` (agent wiring) are separate structs — the names differ to avoid collision within the single package.

### Agent Loop (agent.go)

```
Answer(question) ->
  messages = [system_prompt, user_question]
  loop (max 50 steps):
    response = LLM.Complete(messages, tool_definitions)
    if no tool_calls: return response.Content
    for each tool_call:
      result = Registry.Execute(tool_name, args_json)
      append tool result to messages
    sleep(step_delay)  // rate limit protection
```

Tool errors become `"TOOL_ERROR: <msg>"` strings so the LLM can reason about failures.

### Lazy Skills Strategy

The 212+ skill files (~80KB+) cannot fit in the 8k context window. The system prompt includes only:
1. SKILL.md (89 lines — core diagnostic workflow)
2. A compact file index (filenames only)

The LLM uses `list_skills` and `read_skill` tools to read specific files on demand.

### Sandboxing (sandbox.go)

All file-reading tools go through `Sandbox.Resolve()` which validates paths against an allowlist. This prevents the LLM from reading API keys, `.git`, or anything outside designated directories.

Allowed roots are configured when each entrypoint builds the agent:
- `contrib/agent-rules/skills/tidb-query-tuning/references/`
- `contrib/tidb/pkg/planner/`, `statistics/`, `expression/`, `parser/`
- `contrib/tidb/.agents/skills/`, `contrib/tidb/AGENTS.md`

### Rate Limiting

Kimi free-tier returns 429 frequently. Two mitigations:
1. **Retry with backoff** in `kimi.go`: 5s, 10s, 20s on 429 responses
2. **Step delay** in `agent.go`: configurable pause (default 1s) between LLM requests

### Tools (5 total)

| Tool | File | Purpose |
|------|------|---------|
| `read_file` | readfile.go | Read sandboxed files with offset/limit (default 200 lines) |
| `search_code` | searchcode.go | Grep via `rg` (or `grep` fallback), max 30 results |
| `list_dir` | listdir.go | List directory with `[dir]`/`[file]` markers |
| `list_skills` | toolskills.go | List skill files by category: core, oncall, customer-issues |
| `read_skill` | toolskills.go | Read a specific skill .md file by name |

## How to Extend

### Add a new tool
1. Create a struct in `internal/askplanner/` implementing the `Tool` interface
2. Register it anywhere the shared agent is constructed (`cmd/askplanner/main.go`, `cmd/larkbot/main.go`) via `askplanner.NewRegistry(...)`

### Add a new LLM provider
1. Implement `Provider` interface in a new file in `internal/askplanner/` (e.g. `openai.go`)
2. Add a case in the `switch cfg.LLMProvider` block anywhere the shared agent is constructed (`cmd/askplanner/main.go`, `cmd/larkbot/main.go`)

### Add a new skill category
1. Update `BuildIndex()` in skills.go to scan the new directory
2. Update `ListSkillsTool.Execute()` in toolskills.go to expose the new category
3. Update `ReadSkillTool.Execute()` in toolskills.go to resolve the name prefix

## Configuration

All via environment variables. See README.md for the full table. Key ones:
- `KIMI_API_KEY` or `keys/kimi_free` file
- `KIMI_MODEL` — default `moonshot-v1-8k` (cheapest, 8k context)
- `STEP_DELAY_MS` — default 1000ms between LLM calls
- `MAX_TOOL_STEPS` — default 50
- `MAX_RESULT_CHARS` — default 12000 (truncation limit per tool result)

Lark bot specific:
- `FEISHU_APP_ID` (required)
- `FEISHU_APP_SECRET` (required)

Feishu app prerequisites for `cmd/larkbot`:
- Enable websocket event subscription (Long Connection)
- Subscribe to `im.message.receive_v1`
- Grant scopes: `im:message`, `im:message:send_as_bot`

## Project Layout

```
cmd/askplanner/main.go         Entry point: config, wiring, REPL
cmd/larkbot/main.go            Entry point: Feishu/Lark websocket bot
internal/askplanner/           All logic (single package)
contrib/agent-rules/           Skills repository (git submodule)
contrib/tidb/                  TiDB source code
keys/                          API key files (gitignored)
llm_api/kimi/                  Kimi API documentation (reference only)
```

## Conventions

- `internal/askplanner` stays stdlib-only; `cmd/larkbot` is the place that pulls in the Lark SDK
- Module path: `lab/askplanner`
- All Kimi API wire types are private (lowercase) in kimi.go
- Tool results are plain strings, truncated to `maxResultChars`
- System prompt is built once at startup from the skill index
- `cmd/larkbot` parses text message JSON payloads (`{"text":"..."}`) and replies with Feishu `text` messages
