# askplanner

An AI agent that answers TiDB query optimizer questions by reading engineer-written skills and TiDB planner source code on demand via LLM tool calling.

## How It Works

1. You ask a question about TiDB query optimization
2. The agent sends your question to an LLM (Kimi/Moonshot) with a system prompt containing the core diagnostic workflow (SKILL.md) and an index of 212+ reference files
3. The LLM decides which tools to call — reading skill files, searching code, or browsing directories
4. Tool results are fed back to the LLM, which may call more tools or return the final answer
5. The answer is printed to your terminal

### Available Tools

| Tool | Purpose |
|------|---------|
| `read_file` | Read source code files (sandboxed to allowed paths) |
| `search_code` | Grep for patterns in TiDB planner code |
| `list_dir` | Browse directory structure |
| `list_skills` | List skill/reference files by category |
| `read_skill` | Read a specific skill or oncall experience |

### Skills & References

The agent has access to 212+ reference files organized into:
- **Core references** (15 files) — optimizer hints, join strategies, index selection, stats health, etc.
- **Oncall experiences** (19+ files) — redacted real-world optimizer incidents with symptoms, investigation steps, and fixes
- **Customer planner issues** (176 files) — GitHub issue corpus with linked PRs and merge timestamps

These come from the [pingcap/agent-rules](https://github.com/pingcap/agent-rules) repository (mounted at `contrib/agent-rules/`).

## Prerequisites

- Go 1.23+
- A Kimi (Moonshot AI) API key — get one at [platform.moonshot.cn](https://platform.moonshot.cn)
- TiDB source code at `contrib/tidb/` (or symlinked)
- Skills repository at `contrib/agent-rules/` (git submodule)
- (Lark bot only) A Feishu/Lark app with **messaging** capability and websocket event enabled — create one at [open.feishu.cn](https://open.feishu.cn)

## Quick Start

```bash
# Clone with submodules
git clone https://github.com/guo-shaoge/askplanner.git
cd askplanner
# clone tidb repo and agent-rules repo, it could be time consuming, you can copy your local tidb repo to contrib/tidb if necessary.
git submodule update --init --recursive

# Set up your API key (option A: env var)
export KIMI_API_KEY="sk-your-key-here"

# Set up your API key (option B: key file)
echo "sk-your-key-here" > keys/kimi_free

# Build and run
go build -o bin/askplanner ./cmd/askplanner
./bin/askplanner
```

You'll see a REPL prompt:
```
askplanner (model: moonshot-v1-8k, provider: kimi)
Type your question, or 'quit' to exit.

> My query with ORDER BY and LIMIT picks a full table scan instead of using the index. What should I check?

  [tool] list_skills({"category": "core"})

The most common cause is stale statistics...
```

## Lark Bot

The Lark bot mode connects the same agent to Feishu/Lark via websocket, so users can ask questions directly in a Lark chat.

### Setup

1. Create a Feishu app at [open.feishu.cn](https://open.feishu.cn) and enable:
   - **Bot** capability (under Features)
   - **Event subscription** via websocket (under Events & Callbacks → Subscription Method → use **Long Connection**)
   - Add event `im.message.receive_v1` to receive messages
2. Grant the app these scopes: `im:message`, `im:message:send_as_bot`

### Build & Run

```bash
go build -o bin/larkbot ./cmd/larkbot

FEISHU_APP_ID="cli_xxxx" \
FEISHU_APP_SECRET="xxxx" \
KIMI_API_KEY="sk-your-key-here" \
./bin/larkbot
```

The bot will connect via websocket and start listening for messages. Send a message to the bot in Feishu and it will reply with the agent's answer.

All agent configuration env vars (`KIMI_MODEL`, `MAX_TOOL_STEPS`, etc.) work the same as the CLI mode.

### Lark Bot Environment Variables

| Env Var | Required | Description |
|---------|----------|-------------|
| `FEISHU_APP_ID` | Yes | Feishu app ID (starts with `cli_`) |
| `FEISHU_APP_SECRET` | Yes | Feishu app secret |

## Configuration

All configuration is via environment variables with sensible defaults:

| Env Var | Default | Description |
|---------|---------|-------------|
| `KIMI_API_KEY` | read from `keys/kimi_free` | Moonshot API key |
| `KIMI_MODEL` | `moonshot-v1-8k` | Model ID (see below) |
| `KIMI_BASE_URL` | `https://api.moonshot.cn` | API base URL |
| `LLM_PROVIDER` | `kimi` | LLM backend to use |
| `MAX_TOOL_STEPS` | `50` | Max tool call rounds per question |
| `MAX_RESULT_CHARS` | `12000` | Max chars per tool result |
| `STEP_DELAY_MS` | `1000` | Delay (ms) between LLM requests (rate limit protection) |
| `SKILLS_DIR` | `contrib/agent-rules/skills/tidb-query-tuning/references` | Skills path (relative to project root) |
| `TIDB_CODE_DIR` | `contrib/tidb` | TiDB source path (relative to project root) |

### Model Options

| Model | Context | Input Price | Output Price | Notes |
|-------|---------|-------------|--------------|-------|
| `moonshot-v1-8k` | 8k | 2 yuan/1M | 10 yuan/1M | Cheapest, good for testing |
| `moonshot-v1-32k` | 32k | 5 yuan/1M | 20 yuan/1M | Better for multi-step |
| `kimi-k2-0905-preview` | 256k | 4 yuan/1M | 16 yuan/1M | Best for deep investigation |

Switch models:
```bash
KIMI_MODEL=kimi-k2-0905-preview ./bin/askplanner
```

## Project Structure

```text
askplanner/
├── cmd/askplanner/main.go              # Entry point: REPL loop
├── cmd/larkbot/main.go                 # Entry point: Lark websocket bot
├── internal/askplanner/agent.go        # Agent loop: system prompt + tool dispatch
├── internal/askplanner/config/
│   └── config.go                       # Configuration from env vars
├── internal/askplanner/llmprovider/
│   ├── provider.go                     # LLM provider interface + message types
│   └── kimi.go                         # Kimi/Moonshot implementation (with retry)
├── internal/askplanner/tools/
│   ├── skills.go                       # Skills directory scanner
│   ├── registry.go                     # Tool interface + registry
│   ├── readfile.go                     # read_file tool
│   ├── searchcode.go                   # search_code tool
│   ├── listdir.go                      # list_dir tool
│   └── toolskills.go                   # list_skills + read_skill tools
├── internal/askplanner/util/
│   └── sandbox.go                      # Path sandboxing
├── contrib/
│   ├── agent-rules/                    # Skills from pingcap/agent-rules (submodule)
│   └── tidb/                           # TiDB source code
├── keys/                               # API key files (gitignored)
├── llm_api/kimi/                       # Kimi API documentation (reference)
├── Makefile
├── prompt                              # the original prompt that I first used to build this project
├── README.md
└── AGENTS.md                           # help AI get onboard this project quickly
```

## Adding a New LLM Provider

Implement the `llmprovider.Provider` interface in `internal/askplanner/llmprovider/`:

```go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Name() string
}
```

Then wire it in `cmd/askplanner/main.go` under a new `LLM_PROVIDER` value.

# Roadmap
1. support other LLM as backend
2. support fetch url tool
3. ~~integration with lark bot~~ ✓
4. generate SKILL automatically based on user questions
5. 
