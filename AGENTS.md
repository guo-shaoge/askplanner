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

### Internal Package Design

Shared logic now lives under `internal/askplanner/` across a few focused packages:
- `askplanner` — agent loop and system prompt assembly
- `llmprovider` — provider interfaces and concrete LLM backends
- `tools` — tool interface/registry plus all tool implementations and skill indexing
- `config` — application configuration from env vars
- `util` — small shared infrastructure helpers such as sandboxing

Both `cmd/askplanner/main.go` and `cmd/larkbot/main.go` consume these shared packages and only differ in transport wiring.

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
| `llmprovider.Provider` | `internal/askplanner/llmprovider/provider.go` | Interface for any LLM backend (`Complete` + `Name`) |
| `llmprovider.KimiProvider` | `internal/askplanner/llmprovider/kimi.go` | Kimi/Moonshot API client with 429 retry |
| `askplanner.Agent` / `askplanner.AgentConfig` | `internal/askplanner/agent.go` | Orchestrates the LLM-tool loop |
| `config.Config` | `internal/askplanner/config/config.go` | App configuration from env vars |
| `tools.Tool` | `internal/askplanner/tools/registry.go` | Interface for tools (`Name`, `Description`, `Parameters`, `Execute`) |
| `tools.Registry` | `internal/askplanner/tools/registry.go` | Holds tools, provides `Definitions()` for LLM and `Execute()` for dispatch |
| `util.Sandbox` | `internal/askplanner/util/sandbox.go` | Path validation — restricts file access to allowed directory roots |
| `tools.Index` | `internal/askplanner/tools/skills.go` | Pre-scanned skill metadata for system prompt |

Note: `config.Config` (app config) and `askplanner.AgentConfig` (agent wiring) are separate structs.

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

### Sandboxing (`util/sandbox.go`)

All file-reading tools go through `util.Sandbox.Resolve()` which validates paths against an allowlist. This prevents the LLM from reading API keys, `.git`, or anything outside designated directories.

Allowed roots are configured when each entrypoint builds the agent:
- `contrib/agent-rules/skills/tidb-query-tuning/references/`
- `contrib/tidb/pkg/planner/`, `statistics/`, `expression/`, `parser/`
- `contrib/tidb/.agents/skills/`, `contrib/tidb/AGENTS.md`

### Rate Limiting

Kimi free-tier returns 429 frequently. Two mitigations:
1. **Retry with backoff** in `internal/askplanner/llmprovider/kimi.go`: 5s, 10s, 20s on 429 responses
2. **Step delay** in `agent.go`: configurable pause (default 1s) between LLM requests

### Tools (5 total)

| Tool | File | Purpose |
|------|------|---------|
| `read_file` | `internal/askplanner/tools/readfile.go` | Read sandboxed files with offset/limit (default 200 lines) |
| `search_code` | `internal/askplanner/tools/searchcode.go` | Grep via `rg` (or `grep` fallback), max 30 results |
| `list_dir` | `internal/askplanner/tools/listdir.go` | List directory with `[dir]`/`[file]` markers |
| `list_skills` | `internal/askplanner/tools/toolskills.go` | List skill files by category: core, oncall, customer-issues |
| `read_skill` | `internal/askplanner/tools/toolskills.go` | Read a specific skill .md file by name |

## How to Extend

### Add a new tool
1. Create a struct in `internal/askplanner/tools/` implementing the `tools.Tool` interface
2. Register it anywhere the shared agent is constructed (`cmd/askplanner/main.go`, `cmd/larkbot/main.go`) via `tools.NewRegistry(...)`

### Add a new LLM provider
1. Implement `llmprovider.Provider` in a new file in `internal/askplanner/llmprovider/` (e.g. `openai.go`)
2. Add a case in the `switch cfg.LLMProvider` block anywhere the shared agent is constructed (`cmd/askplanner/main.go`, `cmd/larkbot/main.go`)

### Add a new skill category
1. Update `BuildIndex()` in `internal/askplanner/tools/skills.go` to scan the new directory
2. Update `ListSkillsTool.Execute()` in `internal/askplanner/tools/toolskills.go` to expose the new category
3. Update `ReadSkillTool.Execute()` in `internal/askplanner/tools/toolskills.go` to resolve the name prefix

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
internal/askplanner/           Agent loop and prompt assembly
internal/askplanner/config/    Application configuration loading
internal/askplanner/llmprovider/ LLM provider interfaces and implementations
internal/askplanner/tools/     Tool registry, tool implementations, and skill index
internal/askplanner/util/      Shared helpers such as sandboxing
contrib/agent-rules/           Skills repository (git submodule)
contrib/tidb/                  TiDB source code
keys/                          API key files (gitignored)
llm_api/kimi/                  Kimi API documentation (reference only)
```

## Conventions

- `internal/askplanner/*` stays stdlib-only; `cmd/larkbot` is the place that pulls in the Lark SDK
- Module path: `lab/askplanner`
- All Kimi API wire types are private (lowercase) in `internal/askplanner/llmprovider/kimi.go`
- Tool results are plain strings, truncated to `maxResultChars`
- System prompt is built once at startup from the skill index
- `cmd/larkbot` parses text message JSON payloads (`{"text":"..."}`) and replies with Feishu `text` messages
