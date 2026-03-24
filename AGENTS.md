# askplanner_v2

Go relay for **TiDB SQL query tuning**. Receives questions (CLI or Lark bot) → forwards to [Codex CLI](https://github.com/openai/codex) → returns answer. This project handles session management, prompt loading, and I/O; all reasoning happens inside Codex CLI.

## Architecture

```
cmd/askplanner (CLI REPL)  ─┐
cmd/larkbot (bootstrap)    ─┤
                             ├→ internal/larkbot/app (Feishu bot app lifecycle)
                             │       → internal/larkbot/handler (message routing)
                             │       → internal/larkbot/thread_context (topic history prefetch for new sessions)
                             │       → internal/attachments (user file library)
                             │       → internal/clinic (slow query prefetch)
                             │       → internal/workspace (per-user repo workspace)
                             ├→ internal/codex/responder (session mgmt)
                             │       → internal/codex/runner (exec codex CLI)
                             │            → codex exec ... (external binary)
                             │                 → answer (reply file or JSON stdout)
```

## Key Files

| File | Role |
|---|---|
| `prompt` | 18KB system prompt: TiDB tuning persona, tool adaptation rules, skill refs |
| `internal/config/config.go` | Env-var config loading, `SetupLogging()` |
| `internal/codex/responder.go` | Orchestration: resume vs new session, calls runner. Entry: `NewResponder(cfg).Answer(ctx, key, question)` |
| `internal/codex/runner.go` | `RunNew()` / `RunResume()` — wraps `codex exec`, parses JSON stdout |
| `internal/codex/session_store.go` | Thread-safe JSON file store for sessions (turns, prompt hash, TTL) |
| `internal/codex/prompt.go` | `LoadPrompt()`, `PromptHash()`, `BuildInitialPrompt()` / `BuildResumePrompt()` |
| `internal/larkbot/app.go` | Lark bot app bootstrap, dependency wiring, websocket event loop |
| `internal/larkbot/handler.go` | Message preparation, workspace command flow, Codex/Clinic orchestration |
| `internal/larkbot/message.go` | Feishu message parsing, mention detection, conversation key derivation |
| `internal/larkbot/thread_context.go` | Feishu topic-thread history prefetch and runtime-context building for new sessions |
| `internal/larkbot/attachments.go` | `/upload_n` handling, attachment download/import/context building |
| `internal/larkbot/reply.go` | Reply body rendering, typing reaction, Feishu reply API |
| `internal/workspace/manager.go` | Per-user workspace lifecycle, repo switch/sync/reset, background jobs |
| `internal/clinic/prefetcher.go` | Clinic slow-query link detection, prefetch, stored snapshot context |
| `internal/attachments/store.go` | User attachment library import/snapshot/quota management |
| `cmd/askplanner/main.go` | CLI REPL (`reset`, `quit`) |
| `cmd/larkbot/main.go` | Thin bootstrap: load config/logging, construct app, call `Run()` |

## contrib/ Submodules

| Submodule | Source | Purpose |
|---|---|---|
| `contrib/agent-rules` | `pingcap/agent-rules` | Skills library: oncall patterns, diagnostic workflows |
| `contrib/tidb` | `pingcap/tidb` | TiDB source — optimizer internals ground truth |
| `contrib/tidb-docs` | `pingcap/docs` | Official TiDB docs for SQL syntax, hints, best practices |

Codex CLI `WorkDir` = project root, so it reads `contrib/` via shell commands (`rg`, `cat`, etc.).

## Build & Run

```bash
make all          # bin/askplanner_cli + bin/askplanner_larkbot
make larkbot      # larkbot only
```

Requires: **Go 1.23+**, **codex CLI** in PATH, git submodules initialized.

## Environment Variables

| Variable | Default | Notes |
|---|---|---|
| `FEISHU_APP_ID` | — | **Required** for larkbot |
| `FEISHU_APP_SECRET` | — | **Required** for larkbot |
| `CODEX_BIN` | `codex` | Path to codex binary |
| `CODEX_MODEL` | `gpt-5.3-codex` | |
| `CODEX_REASONING_EFFORT` | `medium` | `low` / `medium` / `high` |
| `CODEX_SANDBOX` | `read-only` | Always read-only |
| `CODEX_SESSION_STORE` | `.askplanner/sessions.json` | |
| `CODEX_MAX_TURNS` | `30` | Turns before auto-reset |
| `CODEX_SESSION_TTL_HOURS` | `24` | |
| `CODEX_TIMEOUT_SEC` | `120` | Per-call timeout |
| `LOG_FILE` | `.askplanner/askplanner.log` | |
| `PROJECT_ROOT` | auto-detected | Walks up looking for `prompt` file |
| `PROMPT_FILE` | `prompt` | Relative to project root |
| `WORKSPACE_ROOT` | `.askplanner/workspaces` | Per-user workspace root |
| `WORKSPACE_IDLE_TTL_HOURS` | `72` | Idle workspace TTL |
| `WORKSPACE_GC_INTERVAL_MIN` | `15` | Workspace GC interval |
| `AGENT_RULES_SYNC_INTERVAL_MIN` | `10` | `agent-rules` mirror sync interval |
| `WORKSPACE_REPO_TIDB_URL` | `https://gh-proxy.org/https://github.com/pingcap/tidb.git` | TiDB mirror remote |
| `WORKSPACE_REPO_TIDB_DEFAULT_REF` | `master` | Default TiDB ref |
| `WORKSPACE_REPO_AGENT_RULES_URL` | `https://gh-proxy.org/https://github.com/pingcap/agent-rules.git` | Agent rules mirror remote |
| `WORKSPACE_REPO_AGENT_RULES_DEFAULT_REF` | `main` | Default agent-rules ref |
| `WORKSPACE_REPO_TIDB_DOCS_URL` | `https://github.com/pingcap/docs.git` | TiDB docs mirror remote |
| `WORKSPACE_REPO_TIDB_DOCS_DEFAULT_REF` | `master` | Default docs ref |
| `CLINIC_ENABLE_AUTO_SLOWQUERY` | `false` | Enable Clinic auto-prefetch |
| `CLINIC_API_KEY` | — | Required when Clinic auto-prefetch is enabled |
| `CLINIC_HTTP_TIMEOUT_SEC` | `15` | Clinic API timeout |
| `CLINIC_STORE_DIR` | `<WORKSPACE_ROOT>/clinic` | Stored Clinic snapshots |
| `CLINIC_STORE_MAX_ITEMS` | `50` | Max Clinic snapshots per user |
| `FEISHU_BOT_NAME` | `askplanner` | Group @ detection fallback name |
| `FEISHU_DEDUP_MESSAGE_TIMEOUT_IN_MIN` | `3600` | Dedup window in minutes |
| `FEISHU_FILE_DIR` | `<WORKSPACE_ROOT>/uploads` | Imported Feishu attachment root |
| `FEISHU_USER_FILE_MAX_ITEMS` | `100` | Max stored attachments per user |

## Workspace

Each Lark bot user gets an isolated workspace so Codex CLI can explore TiDB source, docs, and skills independently per user. The workspace subsystem uses **shared git bare mirrors** plus **per-user git worktrees** to avoid cloning the full repositories for every user.

### Directory Layout

```
<WORKSPACE_ROOT>/                        (default: .askplanner/workspaces)
├── mirrors/                             # shared bare mirrors (all users share these)
│   ├── tidb.git                         #   git clone --mirror of TiDB
│   ├── agent-rules.git                  #   git clone --mirror of agent-rules
│   └── tidb-docs.git                    #   git clone --mirror of tidb-docs
├── users/<sanitized_user_key>/
│   ├── root/                            # Codex CLI WorkDir for this user
│   │   ├── contrib/
│   │   │   ├── tidb/                    #   git worktree → mirrors/tidb.git
│   │   │   ├── agent-rules/             #   git worktree → mirrors/agent-rules.git
│   │   │   └── tidb-docs/               #   git worktree → mirrors/tidb-docs.git
│   │   ├── user-files → <uploads>/<key> #   symlink to user's uploaded attachments
│   │   └── clinic-files → <clinic>/<key>#   symlink to user's Clinic snapshots
│   └── data/
│       └── workspace.json               #   metadata: refs, SHAs, env hash, last active time
├── locks/                               # per-user flock files
│   ├── <user_key>.lock                  #   exclusive lock for workspace mutations
│   └── gc.lock                          #   GC sweep lock
└── .trash/
```

### Key Operations

| Operation | Trigger | Behavior |
|---|---|---|
| `Ensure` | Every user question | Idempotently verifies/creates workspace. Uses blocking flock — must succeed. |
| `SwitchRepo` | `/ws switch <repo> <ref>` | Fetches mirror, resolves ref, checks out worktree. Switching `tidb` auto-follows `tidb-docs` to the same branch if it exists. |
| `Sync` | `/ws sync [repo\|all]` | Fetches latest from remote mirror, re-resolves current ref, re-checkouts. |
| `Reset` | `/ws reset [repo\|all]` | Reverts repo(s) to their default branch/ref. |
| `GC Sweep` | Background timer (`WORKSPACE_GC_INTERVAL_MIN`) | Scans `users/`, removes workspaces idle longer than `WORKSPACE_IDLE_TTL_HOURS`. Uses non-blocking lock — skips busy users. |

`agent-rules` mirror is also synced on a separate background timer (`AGENT_RULES_SYNC_INTERVAL_MIN`), and its worktrees track the latest default branch automatically (`TrackLatest=true`).

### Concurrency

All workspace mutations (`SwitchRepo`, `Sync`, `Reset`) and reads (`Ensure`) acquire a **per-user exclusive flock** (`locks/<user_key>.lock`). This serializes all operations for the same user. GC Sweep uses a non-blocking lock and skips users whose lock is held.

### Environment Hash

`computeEnvironmentHash()` produces a SHA256 from the workspace root path and all repo states (`name|requestedRef|resolvedSHA|trackingLatest`). This hash is stored in `workspace.json` and attached to every Codex session record.

When the hash changes (e.g., user switches branches), `canResume()` in the responder returns `false` with reason `"environment_changed"`, forcing a new Codex session. This guarantees the AI never continues a conversation based on stale source code.

### `/ws` Commands

```
/ws status                              # show current workspace state
/ws switch <repo> <ref> [-- question]   # switch repo to a branch/tag/SHA, optionally ask a question
/ws sync [repo|all]                     # pull latest and re-checkout
/ws reset [repo|all]                    # revert to default branch
```

## Session Management

- Keys: `cli:default` (CLI), `lark:thread:*` / `lark:chat:*:user:*` (bot)
- **Resume** if: same prompt hash, same work dir, same environment hash, turns < max, TTL not expired
- For Lark topic messages with non-empty `thread_id`, the relay prefetches earlier visible thread messages and injects them only into the **initial** prompt of a new bot session; resume prompts do not repeat thread history
- On resume failure: auto-starts new session with last 6 turns as context
- Editing `prompt` invalidates all sessions (hash changes)

## Codex CLI Invocation

```bash
# new
codex exec --sandbox read-only --json --model <model> -c model_reasoning_effort="medium" -o <reply> - < <prompt>
# resume
codex exec resume --json --model <model> -c model_reasoning_effort="medium" -o <reply> <session_id> - < <prompt>
```

Answer: read from reply file (`-o`), fallback to `final_answer` in JSON stdout.

## Conventions

- Module: `lab/askplanner`
- Standard `log` package, env-var-only config, no external deps beyond Lark SDK
- All paths relative to project root
