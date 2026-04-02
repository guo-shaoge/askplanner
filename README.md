# askplanner

An AI assistant for TiDB query optimizer questions.

The current runtime uses Codex CLI as the primary agent engine. askplanner's domain value comes from a generated system prompt, a curated skills corpus, a curated TiDB SQL tuning docs overlay, and access to the local TiDB source tree.

Here is an example of how to use it:
````text
[Yes VPN-ON] [2026/03/19 6:32:52 Day-078] 192 :: ~/work/askplanner_v2 ‹main*› » ./bin/askplanner_cli
askplanner v2 (backend: codex-cli, model: gpt-5.3-codex)
Type your question, or 'quit' to exit. Use 'reset' to start a new session. Use '/model' to inspect or switch the model for this conversation.

> hi


Hi, I’m here. Share your SQL and `EXPLAIN ANALYZE`, and I’ll help tune it.

Refined English: Hi.

> how to optimize a query like 'select * from tbl_a where a < 10 or b > 100;'


For TiDB, this pattern is usually optimized in this order:

1. Refresh stats first (most important).
```sql
ANALYZE TABLE tbl_a;
EXPLAIN ANALYZE SELECT * FROM tbl_a WHERE a < 10 OR b > 100;
```

2. Make sure both predicates are indexable.
```sql
CREATE INDEX idx_a ON tbl_a(a);
CREATE INDEX idx_b ON tbl_a(b);
```

3. Let TiDB use union-type `IndexMerge` for `OR` (if cost model does not pick it automatically).
```sql
EXPLAIN ANALYZE
SELECT /*+ USE_INDEX_MERGE(tbl_a, idx_a, idx_b) */ *
FROM tbl_a
WHERE a < 10 OR b > 100;
```

4. If `IndexMerge` is still slower, rewrite to `UNION ALL` (keeps two range scans, avoids full scan):
```sql
SELECT * FROM tbl_a WHERE a < 10
UNION ALL
SELECT * FROM tbl_a WHERE b > 100 AND NOT (a < 10);
```

Practical notes:
- `SELECT *` often forces table row fetch (`TableRowIDScan`), so speedup might be limited. If possible, select only needed columns.
- In `EXPLAIN ANALYZE`, look for `IndexMerge ... type: union` and check actual time/rows before deciding final form.
- On TiDB v8.1+, auto choice of IndexMerge can be improved by fix control `52869`, but test carefully before enabling globally.
````

## How It Works

1. A user asks a TiDB optimizer question in the local REPL or in Lark.
2. askplanner loads the domain prompt from `prompt` or falls back to the in-process prompt builder.
3. For Lark bot users, askplanner provisions a **per-user workspace** with isolated git worktrees of TiDB source, docs, and agent-rules (shared bare mirrors avoid full clones per user). Users can switch repo versions via `/ws switch tidb <ref>`. The workspace state is tracked in `workspace.json` and influences session resumption through an environment hash.
4. The prompt is normalized for Codex CLI. askplanner-specific tool references such as `read_file` and `search_docs` are translated into shell-based workspace exploration rules.
5. askplanner starts a new Codex session with `codex exec` or resumes an existing one with `codex exec resume`.
6. Codex reads local skills, local TiDB docs, and local TiDB source code from the workspace and returns the answer.
7. askplanner persists the conversation's `session_id` in `.askplanner/sessions.json` so later turns can reuse the same Codex thread.

The runtime is intentionally simple:
- No custom MCP tool bridge yet.
- No in-process LLM API client for the main runtime path.
- No askplanner-managed tool loop on the hot path.

## Architecture

### Runtime Path

- `cmd/askplanner` is the local REPL
- `cmd/larkbot` is the Feishu/Lark websocket bot
- `cmd/askplanner_usage` is the local usage dashboard web server
- `cmd/askplanner_migrate_userdata` copies persisted askplanner user data to a new root
- `internal/codex` is the active runtime

### Core Principle

Codex CLI is the agent runtime.

askplanner is the relay and domain-context layer:
- it prepares the TiDB tuning prompt
- it normalizes that prompt for Codex
- it manages session reuse
- it wires user transport to Codex CLI

reference AGENTS.md for detailed


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

# better use version of codex-cli 0.114.0 or later
codex login

make

# optional: open a local usage dashboard at http://127.0.0.1:18080
./bin/askplanner_usage

# dashboard pages:
# - /            summary + user metrics
# - /questions   paginated question detail view

# by default, log ouput to ./.askplanner/askplanner.log, and session info stored in ./.askplanner/sessions.json
./bin/askplanner_cli
```

`make` also builds `bin/askplanner_migrate_userdata`, which is the helper for moving persisted user data to a new directory or machine.

## Usage Dashboard

askplanner includes a local usage dashboard for lightweight observability.

- Binary: `bin/askplanner_usage`
- Default address: `http://127.0.0.1:18080`
- Pages:
  - `/` overview dashboard (session snapshot + cumulative usage)
  - `/questions` paginated question detail page with filters
- APIs:
  - `/api/usage`
  - `/api/users`
  - `/api/questions`

What it shows:

- Cumulative metrics: total users, total questions, active users (24h / 7d), avg questions per user
- User-level metrics: per-user question counts, last question time, top users
- Question details: question text, user, source, status, latency, model, conversation key, pagination/filtering
- Operational snapshots: recent requests, recent errors, session/workspace breakdowns

Data sources and metric semantics:

- `.askplanner/usage_questions.jsonl` (`USAGE_QUESTIONS_PATH`) is the append-only event store for cumulative user/question metrics.
- Existing `sessions.json` is used for session snapshots.
- `askplanner.log` tail is used for recent request/error trend cards.
- Snapshot-style and cumulative-style metrics are both displayed; interpret them separately.

The REPL supports:
- regular questions
- `reset` to drop the local Codex session
- `/model`, `/model set <model>`, `/model effort <level>`, `/model reset`, `/model effort reset` for conversation-scoped model management
- `quit` / `exit`

## User Data Migration

askplanner stores runtime state under `.askplanner/` by default. That state usually includes:

- `sessions.json`
- `usage_questions.jsonl`
- `askplanner.log`
- `workspaces/uploads/` user attachment files
- `workspaces/clinic/` saved Clinic snapshots
- `workspaces/users/*/data/workspace.json` and other per-user workspace metadata

To migrate that state to a new directory, build and run the migration tool:

```bash
make migrate-userdata
./bin/askplanner_migrate_userdata --source .askplanner /path/to/askplanner-backup
```

You can also run it directly with `go run`:

```bash
go run ./cmd/askplanner_migrate_userdata --source .askplanner /path/to/askplanner-backup
```

What the tool does:

- Recursively copies the source askplanner data directory to the destination.
- Preserves normal files and directories, including sessions, usage events, logs, uploaded files, Clinic snapshots, and workspace metadata.
- Rewrites absolute workspace symlinks such as `user-files` and `clinic-files` so they remain valid under the destination root.
- Skips managed repository data under `workspaces/mirrors/`.
- Skips managed worktrees under `workspaces/users/*/root/contrib/{tidb,tidb-docs,agent-rules}` because askplanner can recreate them from the configured remotes.

Recommended migration procedure:

1. Stop `askplanner_cli`, `askplanner_larkbot`, and `askplanner_usage`.
2. Run `./bin/askplanner_migrate_userdata --source <old_data_dir> <new_data_dir>`.
3. Point the new deployment at the migrated directory, or move the migrated tree into the default `.askplanner` location.
4. Start askplanner and run `/ws status` for one Lark user or send a test question so managed worktrees are recreated as needed.

Constraints:

- The destination directory must be different from the source.
- The destination cannot be inside the source directory.
- The tool is for askplanner's persisted runtime data, not for the top-level git repo.
- Managed repo mirrors/worktrees are intentionally not copied; the first workspace `Ensure`, `/ws sync`, or `/ws switch` will recreate or refresh them.

## Lark Bot

Build:

```bash
go build -o bin/askplanner_larkbot ./cmd/larkbot
```

Run:

```bash
FEISHU_APP_ID="cli_xxxx" \
FEISHU_APP_SECRET="xxxx" \
FEISHU_BOT_NAME="askplanner" \
./bin/askplanner_larkbot
```

Or run multiple bots in one process:

```bash
FEISHU_BOTS_JSON='[
  {"key":"bot-a","app_id":"cli_xxxx","app_secret":"xxxx","bot_name":"askplanner-a"},
  {"key":"bot-b","app_id":"cli_yyyy","app_secret":"yyyy","bot_name":"askplanner-b"}
]' \
./bin/askplanner_larkbot
```

In group chats, the bot only handles `text` and `post` rich-text messages that are explicitly addressed to it. The most reliable setup is to configure `FEISHU_BOT_NAME` (defaults to `askplanner`) so the bot can verify that the mention target is actually itself by matching the `name` field in the mentions list.

When `FEISHU_BOTS_JSON` is used, each bot gets its own logical namespace in conversation keys, session keys, workspaces, and attachment libraries, while still sharing the same askplanner process and storage formats.

The bot stores attachments in a persistent per-user library under `FEISHU_FILE_DIR`. Each sender gets a separate directory, and the library keeps at most `FEISHU_USER_FILE_MAX_ITEMS` top-level items (default `100`). When the limit is exceeded, the oldest top-level items are deleted first.

In 1:1 chats, direct `file` and `image` uploads are downloaded and saved immediately. Images are renamed with a timestamped filename so users can distinguish them later. Uploading a file with the same top-level name replaces the existing item. Uploading a `.zip` file extracts it into a directory with the zip basename and removes the downloaded archive after a successful extraction.

In group chats, attachment loading is explicit. Users can send `@bot /upload_3 analyze the latest files`, and the bot will walk backward through recent messages, download the newest three `file` or `image` attachments previously sent by the same sender in the same chat/thread, save them into that sender's library, and then answer the remaining question. If there is no trailing question, the bot replies with only the download summary.

For every Codex request, the runtime injects the current user's library root path plus a compact top-level summary into the prompt. The model is instructed to inspect that directory directly and to ask the user which file they mean when the reference is ambiguous instead of guessing.

Both `file` and `image` attachments are supported. Other message types (audio, video, sticker, etc.) are not yet supported.

### Workspace Commands

Users can manage their per-user workspace (TiDB source version, docs, skills) directly in the chat:

```
/ws status                              # show current workspace state
/ws switch tidb release-8.1 -- question # switch TiDB to a branch and ask a question
/ws sync [repo|all]                     # pull latest from remote and re-checkout
/ws reset [repo|all]                    # revert to default branch
```

Switching `tidb` automatically follows `tidb-docs` to the same branch if it exists. See the Workspace section in AGENTS.md for architecture details.

### Conversation Keys

The bot computes a conversation key from:
- `thread_id + sender_id` when present
- otherwise `chat_id + sender_id`
- otherwise a message-level fallback

That conversation key maps to a Codex `session_id` in the local session store.

## Configuration

All configuration is via environment variables. See AGENTS.md for the full reference table.

Core variables:

| Env Var | Default | Description |
|--------|---------|-------------|
| `CODEX_BIN` | `codex` | Codex CLI binary |
| `CODEX_MODEL` | `gpt-5.3-codex` | Codex model |
| `CODEX_REASONING_EFFORT` | `medium` | Default reasoning effort used for Codex execution when the selected model does not expose a cache-derived default |
| `CODEX_SANDBOX` | `read-only` | Sandbox mode for `codex exec` |
| `CODEX_PROJECT_ROOT` | `.` | Working root for Codex |
| `CODEX_PROMPT_COMMAND` | `bin/printprompt` | Prompt command, supports args such as `bin/printprompt --normalized` |
| `CODEX_SESSION_STORE` | `.askplanner/codex_sessions.json` | Session store path |
| `CODEX_REASONING_EFFORT` | `medium` | `low` / `medium` / `high` |
| `CODEX_SESSION_STORE` | `.askplanner/sessions.json` | Session store path |
| `CODEX_MAX_TURNS` | `30` | Max turns before forcing a new Codex session |
| `CODEX_SESSION_TTL_HOURS` | `24` | Session TTL |
| `CODEX_TIMEOUT_SEC` | `120` | Timeout per Codex subprocess |
| `LOG_FILE` | `.askplanner/askplanner.log` | Log file path |
| `USAGE_HTTP_ADDR` | `127.0.0.1:18080` | Usage dashboard listen address |
| `USAGE_LOG_TAIL_BYTES` | `4194304` | Max log bytes scanned per refresh |
| `USAGE_QUESTIONS_PATH` | `.askplanner/usage_questions.jsonl` | Append-only question event store used for cumulative user/question metrics and paginated question details |
| `PROJECT_ROOT` | auto-detected | Walks up looking for `prompt` file |
| `PROMPT_FILE` | `prompt` | Relative to project root |

Workspace variables:

| Env Var | Default | Description |
|--------|---------|-------------|
| `WORKSPACE_ROOT` | `.askplanner/workspaces` | Per-user workspace root |
| `WORKSPACE_IDLE_TTL_HOURS` | `72` | Idle workspace TTL before GC |
| `WORKSPACE_GC_INTERVAL_MIN` | `15` | Workspace GC sweep interval |
| `AGENT_RULES_SYNC_INTERVAL_MIN` | `10` | `agent-rules` mirror sync interval |
| `WORKSPACE_REPO_TIDB_URL` | (gh-proxy mirror) | TiDB mirror remote |
| `WORKSPACE_REPO_TIDB_DEFAULT_REF` | `master` | Default TiDB ref |
| `WORKSPACE_REPO_AGENT_RULES_URL` | (gh-proxy mirror) | Agent rules mirror remote |
| `WORKSPACE_REPO_AGENT_RULES_DEFAULT_REF` | `main` | Default agent-rules ref |
| `WORKSPACE_REPO_TIDB_DOCS_URL` | `https://github.com/pingcap/docs.git` | TiDB docs remote |
| `WORKSPACE_REPO_TIDB_DOCS_DEFAULT_REF` | `master` | Default docs ref |

Clinic variables:

| Env Var | Default | Description |
|--------|---------|-------------|
| `CLINIC_ENABLE_AUTO_SLOWQUERY` | `false` | Enable Clinic auto-prefetch |
| `CLINIC_API_KEY` | — | Required when Clinic auto-prefetch is enabled |
| `CLINIC_HTTP_TIMEOUT_SEC` | `15` | Clinic API timeout |
| `CLINIC_STORE_DIR` | `<WORKSPACE_ROOT>/clinic` | Stored Clinic snapshots |
| `CLINIC_STORE_MAX_ITEMS` | `50` | Max Clinic snapshots per user |

Lark-specific variables:

| Env Var | Required | Description |
|--------|----------|-------------|
| `FEISHU_BOTS_JSON` | No | JSON array of bot configs for multi-bot mode; when set, askplanner ignores the single-bot Feishu credential vars |
| `FEISHU_APP_ID` | Yes* | Feishu app ID in legacy single-bot mode |
| `FEISHU_APP_SECRET` | Yes* | Feishu app secret in legacy single-bot mode |
| `FEISHU_BOT_NAME` | No | Bot display name for group-chat mention matching; defaults to `askplanner` |
| `FEISHU_DEDUP_MESSAGE_TIMEOUT_IN_MIN` | No | Dedup window in minutes; defaults to `3600` |
| `FEISHU_FILE_DIR` | No | Per-user attachment library root; defaults to `<WORKSPACE_ROOT>/uploads` |
| `FEISHU_USER_FILE_MAX_ITEMS` | No | Max top-level items per user library; defaults to `100` |

## Build and Verify

```bash
go build -o bin/askplanner_cli ./cmd/askplanner
go build -o bin/askplanner_larkbot ./cmd/larkbot
go build -o bin/askplanner_usage ./cmd/askplanner_usage
go build -o bin/askplanner_migrate_userdata ./cmd/askplanner_migrate_userdata
go test ./...
```
## Roadmap
### user perspective
- [x] fix the larkbot message resent problem
- [x] support process image
- [x] support process plan replayer(unzip replayer and start diagnose automatically)
- [x] support diagnose by clinic link
- [x] support Feishu rich-text (`post`) plan questions and quote-style refs ([#12](https://github.com/guo-shaoge/askplanner/issues/12))
- [x] output a markdown file if output too long
- [x] error handling: make sure all errors should return to user clearly(like network issue, rate limte ect) (https://github.com/guo-shaoge/askplanner/pull/17)
- [x] support switch to different release version of tidb repo and tidb-docs repo (https://github.com/guo-shaoge/askplanner/commit/4e11ec3ac9f319d5bec545eeb0d159a452bf7d56)
- [ ] support process clinic in batch mode
- [ ] support context management
- [x] support model management
- [ ] slow query finder
- [ ] optimize the response time, especially the first user input request
- [ ] add a watchdog/daemon to keep the askplanner background process alive

### implementation perspective
- [ ] rotate log to avoid getting log too large
- [ ] optimize the write process of session store
- [ ] security related: better put agent in docker
- [ ] rate-limit
- [ ] use codex cli sdk to avoid start a new process for every user requesT
- [ ] store dedup map in disk to avoid the map was lost when process restart.
