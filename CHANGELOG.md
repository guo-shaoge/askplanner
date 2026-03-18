# CHANGE LOG
## first prompt to build this project
date: 2026/03/17

I'd like to build a new version of this project(you can ignore agent.go and main.go, and start it from zero and ignore current implementation).
The projection aims to build an agent for tidb optimizer, the required function is:
1. receive user input message
    1. the final target is to receive by lark websocket.
    2. but for now, you can just implement a simple one, for example read message by http or terminal input. so I can test it locally.
2. support read skills and source code to answer question(function call in LLM)
    1. the skill files are from contrib/agent-rules/skills/tidb-query-tuning/references, and you can skip other skill path for now.
    2. the code path is contrib/tidb, and you can just focus on planner code (and there are also some skills in tidb repo).
       (PS: using git submodule to clone tidb repo is too slow, so I just copy tidb directory here, I'll use git submodule later)
3. support different backend LLM
    1. Kimi
        1. for now you can just use Kimi as backend LLM
        2. you can find the Kimi API KEY in keys/kimi_free
        3. check this page to understand the Kimi model and its price, you should use the lowest price model for test(https://platform.moonshot.cn/docs/pricing/chat#%E7%94%9F%E6%88%90%E6%A8%A1%E5%9E%8B-moonshot-v1)
        4. check this page to understand how to call Kimi API: https://platform.moonshot.cn/docs/api/chat#%E5%85%AC%E5%BC%80%E7%9A%84%E6%9C%8D%E5%8A%A1%E5%9C%B0%E5%9D%80
        5. check this page to understand how to use tool: https://platform.moonshot.cn/docs/api/tool-use
        6. you can browse the other Kimi document webset to find what you need: https://platform.moonshot.cn/docs/overview
        7. write an skill or memory file to record what you learned from Kimi document websit, so you can get the context fast next time.
    2. Other models support is optional and you can implement in the future, but you need provide a easy way to switch different models
4. use plan mode first to let know important decisions
    1. for example what language you plan to use for this agent. I tend to prefer Golang, because I'm most familar with it. If you think others are better, you can argue for it(for example Python), and the main goal is just to get it implemented smoothly.

## support tidb docs as new source for LLM
date: 2026/03/18

### first version plan
```
# Add a Local Official-Docs SQL Tuning Overlay

## Summary
Implement this as an askplanner-local prompt overlay, not as changes inside the `contrib/agent-rules` submodule. The new overlay will preload a compact, docs-derived `SKILL.md` based only on the self-managed `SQL Tuning` section of `contrib/tidb-docs/TOC.md` plus `sql-tuning-best-practice.md`, and will add one discovery tool, `search_docs`, for scoped lookup within that curated docs set.

## Files to add
- `prompts/tidb-query-tuning-official-docs/AGENTS.md`: maintenance instructions for this local overlay. Define the source of truth as `contrib/tidb-docs/TOC.md` self-managed `SQL Tuning` section plus `sql-tuning-best-practice.md`; state that this file is for maintainers only and is not loaded into the runtime prompt.
- `prompts/tidb-query-tuning-official-docs/SKILL.md`: distilled official-doc workflow and guardrails. Keep it concise and complementary to the existing `contrib/agent-rules` `SKILL.md`: official docs for documented behavior and syntax, existing skill for diagnosis workflow and field precedents, source code for implementation truth when docs are vague.
- `prompts/tidb-query-tuning-official-docs/manifest.json`: curated docs list used by code and prompt generation. Store an array of objects with `title` and `path`, where `path` is relative to `contrib/tidb-docs`. Include exactly the SQL Tuning pages from `TOC.md` and `sql-tuning-best-practice.md`; exclude Cloud TOCs, release notes, media, and unrelated docs.
- `internal/askplanner/tools/docsindex.go`: load `SKILL.md` and `manifest.json` from the local overlay directory into a new `DocsOverlay` type. Expose a small `SystemPromptSection()` method that renders the overlay prompt plus a compact docs index.
- `internal/askplanner/tools/searchdocs.go`: new `search_docs` tool. Search only the manifest-listed files under `contrib/tidb-docs`, return matching paths and line numbers, and instruct the model to use `read_file` for full page reads.
- `internal/askplanner/tools/docsindex_test.go`: unit tests for overlay loading, manifest validation, and prompt-section rendering.
- `internal/askplanner/tools/searchdocs_test.go`: unit tests for scoped search behavior, bad manifest paths, and no-match handling.

## Files to change
- `internal/askplanner/config/config.go`: add `TiDBDocsDir` and `DocsOverlayDir` to `Config`. Add env vars `TIDB_DOCS_DIR` defaulting to `contrib/tidb-docs` and `DOCS_OVERLAY_DIR` defaulting to `prompts/tidb-query-tuning-official-docs`.
- `internal/askplanner/agent.go`: extend `AgentConfig` with `DocsOverlay *tools.DocsOverlay`. Update `buildSystemPrompt` to compose, in order: current core rules, existing skill index section, then official-doc overlay section. Add a short rule that the docs overlay reflects the checked-out `contrib/tidb-docs` working tree and should be treated as version-sensitive.
- `cmd/askplanner/main.go`: load the docs overlay at startup; fail fast if the overlay or manifest is invalid. Add `contrib/tidb-docs` to the sandbox allowlist and register `search_docs` in the tool registry.
- `cmd/larkbot/main.go`: mirror the same startup loading, sandbox update, and tool registration as the CLI path.
- `README.md`: document the new official-doc overlay, the new `search_docs` tool, and the two new env vars. Clarify that only the SQL Tuning subset of `tidb-docs` is pre-curated in v1.
- `AGENTS.md`: update onboarding to describe the new official-doc source, the local overlay location, and the prompt/source precedence.

## Public APIs and behavior
- `internal/askplanner/config.Config` gains `TiDBDocsDir` and `DocsOverlayDir`.
- `internal/askplanner/askplanner.AgentConfig` gains `DocsOverlay *tools.DocsOverlay`.
- Add tool `search_docs` with parameters `pattern` required and `limit` optional. Default `limit` to 10; cap at 20. Search scope is only `manifest.json` paths.
- Keep `read_file` as the page reader. Do not add `read_doc`; the model should `search_docs` then `read_file`.

## Manifest contents
Include these paths from `contrib/tidb-docs` in `manifest.json`:
`sql-tuning-best-practice.md`, `sql-tuning-overview.md`, all `explain-*.md` pages listed under SQL Tuning, `sql-optimization-concepts.md`, `sql-logical-optimization.md`, `subquery-optimization.md`, `column-pruning.md`, `correlated-subquery-optimization.md`, `max-min-eliminate.md`, `predicate-push-down.md`, `partition-pruning.md`, `topn-limit-push-down.md`, `join-reorder.md`, `derive-topn-from-window.md`, `sql-physical-optimization.md`, `choose-index.md`, `statistics.md`, `extended-statistics.md`, `wrong-index-solution.md`, `agg-distinct-optimization.md`, `cost-model.md`, `runtime-filter.md`, `sql-prepared-plan-cache.md`, `sql-non-prepared-plan-cache.md`, `control-execution-plan.md`, `optimizer-hints.md`, `sql-plan-management.md`, `blocklist-control-plan.md`, `optimizer-fix-controls.md`, and `index-advisor.md`.

## Test plan
- `go test ./...` passes with the new overlay loader and tool.
- CLI and Lark both start successfully when the overlay files exist and fail with a clear startup error when `manifest.json` or `SKILL.md` is missing.
- `search_docs` returns only curated SQL Tuning files and never searches release notes or unrelated docs.
- Prompt composition includes both the existing `contrib/agent-rules` skill content and the new official-doc overlay in the expected order.
- Manual QA scenarios:
  - “What optimizer hint controls join reorder?” should use the official docs overlay and/or `search_docs`.
  - “Why does TiDB choose the wrong index after stats drift?” should still use the existing tuning skill first and use docs as supporting reference.
  - “Does prepared plan cache apply here?” should find `sql-prepared-plan-cache.md`.
  - “Show me the official doc for TopN pushdown” should search and then read the curated doc path.

## Assumptions and defaults
- The new prompt assets live in the askplanner repo under `prompts/tidb-query-tuning-official-docs`; `contrib/agent-rules` remains untouched.
- v1 only curates the self-managed `TOC.md` SQL Tuning section plus `sql-tuning-best-practice.md`; TiDB Cloud SQL Tuning docs stay out of scope.
- The official-doc overlay is additive and does not replace the existing `contrib/agent-rules` `SKILL.md`.
- Version handling in v1 is prompt-level only: the model is told that docs reflect the checked-out `contrib/tidb-docs` tree; no dynamic branch/commit detection is added.
```

### review comment
```
 Plan Review: Support TiDB Docs as New Source for LLM

  Overall Assessment

  The plan is well-structured and architecturally sound. The additive overlay approach, manifest-based scoping, and reuse
   of read_file are all good decisions. A few issues worth addressing before implementation:

  Issues

  1. DocsOverlay placement is fine, but naming could confuse

  Putting docsindex.go in tools/ is consistent with the existing skills.go (the Index type also lives there and isn't a
  tool). However, the plan places two files in tools/ that serve different purposes — docsindex.go is a prompt component,
   searchdocs.go is an actual tool. This is the same pattern as today (skills.go vs toolskills.go), so it's consistent.
  No change needed, just noting it.

  2. Startup fail-fast inconsistency

  The plan says "fail fast if the overlay or manifest is invalid." Currently, BuildIndex() in skills.go silently handles
  missing skill directories (returns empty lists). Adding fail-fast only for the docs overlay creates an asymmetry — if
  contrib/tidb-docs isn't cloned (e.g., fresh checkout without git submodule update), the app won't start. Consider
  either:
  - Making it a warning + graceful degradation (skip overlay if missing), or
  - Also tightening the existing skill index loading to fail-fast (breaking change, but more consistent).

  I'd recommend graceful degradation with a clear warning log — this lets the agent still work for code-only questions
  when docs aren't available.

  3. Missing: guidance in system prompt for tool selection

  The SKILL.md describes precedence (official docs → existing skill → source code), but the search_docs tool description
  and the search_code tool description should also help the model decide which to use. Consider adding a line to
  search_code's description like "Use this for TiDB source code internals" and to search_docs like "Use this for official
   TiDB documentation on SQL tuning features, syntax, and best practices." This is an implementation detail, but worth
  noting in the plan.

  4. Manifest file existence validation

  The plan says "fail fast if manifest is invalid" but doesn't specify whether invalid means malformed JSON or also
  includes missing files. I'd recommend validating that every path in manifest.json actually exists under
  contrib/tidb-docs/ at startup — this catches stale manifest entries when tidb-docs is updated.

  5. Duplication risk between CLI and Lark entry points

  Currently both cmd/askplanner/main.go and cmd/larkbot/main.go duplicate the agent setup logic (sandbox paths, tool
  registration, skill index building). The plan adds more shared setup (overlay loading, sandbox update, tool
  registration) to both. This is a good opportunity to extract a shared buildAgent() or Setup() function into
  internal/askplanner/ to avoid drift. The larkbot already has a buildAgent() — consider promoting it to a shared package
   function.

  6. Prompt size budget

  The existing system prompt is ~600 lines. Adding another SKILL.md + a 30-file manifest index will grow it. With Kimi's
  moonshot-v1-8k (if that's the model in use), this could eat into the question/answer budget. Verify the total prompt
  stays well within the model's context window, or consider using a larger model for docs-heavy queries.

  7. search_docs limit cap at 20 — possibly too generous

  Each result returned by search_docs includes path + line numbers. If the model then calls read_file on multiple
  results, it could trigger many tool rounds. The existing search_code caps at 30 results. A cap of 10 default / 20 max
  is reasonable, but consider whether the default of 10 is too many for the Kimi model's tool-loop budget (currently
  maxSteps=50).

  What Looks Good

  - Additive overlay — not touching contrib/agent-rules is the right call.
  - Reusing read_file instead of adding read_doc — avoids tool proliferation.
  - Manifest-based scoping — clean, auditable, easy to update.
  - Config via env vars with sensible defaults — consistent with existing config pattern.
  - Test plan — both unit tests and manual QA scenarios are well-defined.
  - Version-sensitive prompt note — pragmatic v1 approach without over-engineering.
  - Curated scope (self-managed SQL Tuning only, no Cloud docs) — keeps the overlay focused.

  Minor Suggestions

  - In manifest.json, consider adding a section field (e.g., "section": "explain", "section": "optimization") so the
  model can filter search results by topic area if needed in v2.
  - The plan lists AGENTS.md in the overlay directory as "maintenance instructions, not loaded into runtime." Document
  this convention clearly so future contributors don't accidentally load it.

  Verdict

  Ready to implement with the above considerations. The most actionable items are #2 (graceful degradation vs fail-fast),
   #5 (extract shared setup), and #6 (verify prompt size budget). The rest are polish.

  ---
  Translation / Refined English:

  ▎ I plan to add contrib/tidb-docs as a new retrieval source for the LLM. The concrete plan is in the "Support TiDB Docs
   as New Source for LLM" section of CHANGELOG.md. Please review this plan.

  → "I'd like to add contrib/tidb-docs as a new retrieval source for the LLM. The detailed implementation plan is
  documented in the 'Support TiDB Docs as New Source for LLM' section of CHANGELOG.md — could you review it?"
```


### final plan
```
  # Add a Local Official-Docs SQL Tuning Overlay

  ## Summary

  Implement an askplanner-local official-doc overlay that preloads a compact docs-derived SKILL.md based on the self-managed SQL Tuning section of contrib/tidb-docs/TOC.md plus sql-tuning-best-practice.md. Keep it additive to the
  existing contrib/agent-rules skill corpus. Add one new retrieval tool, search_docs, scoped to a curated manifest of SQL tuning docs.

  The overlay must degrade gracefully: if contrib/tidb-docs or the local overlay assets are missing or invalid, the app still starts and logs a clear warning, and the agent continues to work with the existing skill/code sources.

  ## Files to add

  - prompts/tidb-query-tuning-official-docs/AGENTS.md
      - Maintainer-only instructions for refreshing the overlay.
      - Define source of truth as contrib/tidb-docs/TOC.md self-managed SQL Tuning section plus sql-tuning-best-practice.md.
      - State explicit exclusions: Cloud docs, release notes, media, unrelated docs.
      - Document that missing overlay assets should not block startup.
  - prompts/tidb-query-tuning-official-docs/SKILL.md
      - Concise official-doc overlay prompt.
      - State source-selection policy:
          - Official docs for documented behavior, syntax, variables, hints, and best practices.
          - Existing contrib/agent-rules skill for diagnosis workflow, oncall incidents, and customer precedents.
          - TiDB source code for implementation internals or when docs are ambiguous.
      - State version caveat: docs reflect the checked-out contrib/tidb-docs tree.
  - prompts/tidb-query-tuning-official-docs/manifest.json
      - Curated list of SQL tuning pages to preload/index.
      - Format: array of { "title": "...", "path": "..." }, where path is relative to contrib/tidb-docs.
  - internal/askplanner/tools/docsindex.go
      - New prompt-support component, analogous to skills.go.
      - Define DocsOverlay type with:
          - SkillMD string
          - Docs []DocEntry
          - Available bool
          - Warning string
      - Implement loader that:
          - Reads local overlay SKILL.md
          - Parses manifest.json
          - Validates that every manifest path exists under contrib/tidb-docs
          - Returns a usable overlay when valid
          - Returns Available=false plus warning text when missing or invalid
      - Implement SystemPromptSection() that renders the overlay prompt and a compact docs index only when available.
  - internal/askplanner/tools/searchdocs.go
      - New real tool, analogous to searchcode.go.
      - Search only manifest-listed files under contrib/tidb-docs.
      - Return matching file path, line number, and snippet summary.
      - Tool description must clearly say it is for official TiDB documentation on SQL tuning features, syntax, variables, and best practices.
  - internal/askplanner/tools/docsindex_test.go
      - Unit tests for overlay loading, graceful degradation, manifest parsing, missing-file validation, and prompt rendering.
  - internal/askplanner/tools/searchdocs_test.go
      - Unit tests for scoped search, limit behavior, no-match behavior, and rejection of files outside the manifest.

  ## Files to change

  - internal/askplanner/config/config.go
      - Add TiDBDocsDir and DocsOverlayDir to Config.
      - Add env vars:
          - TIDB_DOCS_DIR, default contrib/tidb-docs
          - DOCS_OVERLAY_DIR, default prompts/tidb-query-tuning-official-docs
  - internal/askplanner/agent.go
      - Extend AgentConfig with DocsOverlay *tools.DocsOverlay.
      - Update buildSystemPrompt to compose:
          - existing core rules
          - TiDB planner code layout
          - existing skill index section
          - official-doc overlay section when DocsOverlay.Available
      - Add one short rule telling the model to prefer official docs for documented SQL tuning behavior and use code for internals.
      - Do not mention overlay content when unavailable.
  - internal/askplanner/tools/searchcode.go
      - Update tool description to clarify scope: use for TiDB source code internals, optimizer implementation, and code-level behavior.
      - Keep existing behavior unchanged otherwise.
  - cmd/askplanner/main.go
      - Load docs overlay at startup after config load.
      - If overlay load fails or validation fails, log a warning and continue without docs overlay.
      - Add contrib/tidb-docs to sandbox allowlist.
      - Register search_docs only when the overlay is available and valid.
      - Keep existing skill index loading behavior unchanged.
  - cmd/larkbot/main.go
      - Mirror the same overlay loading, warning behavior, sandbox update, and conditional tool registration as CLI.
  - README.md
      - Document:
          - new official-doc overlay
          - search_docs tool
          - TIDB_DOCS_DIR and DOCS_OVERLAY_DIR
          - graceful degradation behavior when docs overlay is absent
          - v1 scope limited to self-managed SQL tuning docs
  - AGENTS.md
      - Update onboarding to describe:
          - local overlay location
          - source precedence
          - conditional availability of docs overlay/search tool
          - the fact that manifest entries are validated against the checked-out contrib/tidb-docs

  ## Public APIs and behavior

  - internal/askplanner/config.Config gains:
      - TiDBDocsDir string
      - DocsOverlayDir string
  - askplanner.AgentConfig gains:
      - DocsOverlay *tools.DocsOverlay
  - Add tool search_docs
      - Parameters:
          - pattern required
          - limit optional
      - Defaults:
          - default limit=10
          - cap limit=20
      - Availability:
          - only registered when docs overlay is valid and all manifest paths exist
  - Keep read_file as the page reader for docs content.
      - The model should search_docs first, then read_file specific doc pages.

  ## Manifest contents

  manifest.json should include exactly these self-managed SQL tuning pages from contrib/tidb-docs:

  - sql-tuning-best-practice.md
  - sql-tuning-overview.md
  - explain-overview.md
  - explain-walkthrough.md
  - explain-indexes.md
  - explain-joins.md
  - explain-mpp.md
  - explain-subqueries.md
  - explain-aggregation.md
  - explain-views.md
  - explain-partitions.md
  - explain-index-merge.md
  - sql-optimization-concepts.md
  - sql-logical-optimization.md
  - subquery-optimization.md
  - column-pruning.md
  - correlated-subquery-optimization.md
  - max-min-eliminate.md
  - predicate-push-down.md
  - partition-pruning.md
  - topn-limit-push-down.md
  - join-reorder.md
  - derive-topn-from-window.md
  - sql-physical-optimization.md
  - choose-index.md
  - statistics.md
  - extended-statistics.md
  - wrong-index-solution.md
  - agg-distinct-optimization.md
  - cost-model.md
  - runtime-filter.md
  - sql-prepared-plan-cache.md
  - sql-non-prepared-plan-cache.md
  - control-execution-plan.md
  - optimizer-hints.md
  - sql-plan-management.md
  - blocklist-control-plan.md
  - optimizer-fix-controls.md
  - index-advisor.md

  ## Validation behavior

  - Overlay is considered invalid if:
      - local SKILL.md is missing
      - manifest.json is missing
      - manifest.json is malformed
      - any manifest path is empty
      - any manifest path resolves outside contrib/tidb-docs
      - any manifest path does not exist on disk
  - Invalid overlay does not abort startup.
      - Log one clear warning with the reason.
      - Do not register search_docs.
      - Do not include official-doc overlay text in the system prompt.

  ## Test plan

  - go test ./... passes.
  - Loader behavior:
      - valid overlay loads and contributes prompt text
      - missing overlay directory logs warning and continues
      - malformed manifest logs warning and continues
      - manifest entry pointing to a missing file logs warning and continues
      - manifest entry escaping docs root is rejected
  - Tool behavior:
      - search_docs only searches manifest-listed files
      - search_docs is absent when overlay is unavailable
      - search_code description clearly distinguishes code-internal usage from docs usage
  - Prompt composition:
      - prompt contains existing contrib/agent-rules skill section plus docs overlay section when available
      - prompt omits docs overlay entirely when unavailable
  - Manual QA scenarios:
      - “What optimizer hint controls join reorder?” prefers docs overlay and/or search_docs
      - “Why does TiDB choose the wrong index after stats drift?” still uses the existing tuning skill and may cite official docs as support
      - “How does prepared plan cache work?” reaches the curated docs set
      - fresh checkout without contrib/tidb-docs still starts and answers code/skill questions

  ## Assumptions and defaults

  - Docs-derived prompt assets live in the askplanner repo under prompts/tidb-query-tuning-official-docs.
  - contrib/agent-rules remains unchanged.
  - v1 only covers self-managed SQL tuning docs from TOC.md plus sql-tuning-best-practice.md.
  - Existing skill index loading remains as-is; only the new docs overlay uses graceful degradation and explicit validation.
  - No dynamic branch/commit-aware prompt logic is added in v1 beyond stating that docs reflect the checked-out contrib/tidb-docs tree.


• Model changed to gpt-5.4 xhigh for Default mode.
```

## refine search_docs tool
```
Plan: Replace grep-based search_docs with keyword-based heading search

 Context

 The current search_docs tool runs ripgrep/grep on curated doc files, treating the LLM's query as a literal regex
 pattern. This fails for natural language queries — e.g., search_docs(pattern="reload binding") finds nothing, even
 though the answer lives in sql-plan-management.md under headings like "Cache bindings" and "SQL binding". The
 fundamental issue is a semantic gap: grep needs exact text matches, but users (and the LLM) use natural language.

 With only 40 curated docs, we can do much better by searching headings and metadata (which are semantically meaningful
 topic labels) instead of raw content.

 Approach: Heading Index + Compact Prompt Outline

 Two complementary changes:

 1. Build a heading index at startup — parse all 40 manifest docs, extract headings (lines starting with #) plus YAML
 title/summary. Store as in-memory index.
 2. Replace grep search with keyword matching against headings — tokenize the query into keywords, match against heading
  text + doc titles + summaries. Return results grouped by doc with line numbers, so the LLM can read_file at the right
 offset.
 3. Add compact ##-level outlines to the system prompt — each manifest entry shows its top-level sections (e.g., SQL
 Plan Management (sql-plan-management.md): SQL binding | Baseline capturing | Cross-database binding | ...). This lets
 the LLM pick the right file directly for obvious queries without a tool call. Adds ~400 tokens.

 Files to Change

 internal/askplanner/tools/docsindex.go

 - Add types:
 type DocHeading struct {
     DocPath  string // relative path from manifest
     DocTitle string // from manifest
     Heading  string // heading text
     Level    int    // 1-4
     Line     int    // 1-based line number
 }

 type HeadingIndex struct {
     Entries []DocHeading
 }
 - Add BuildHeadingIndex(docs []DocEntry, docsDir string) *HeadingIndex:
   - For each manifest doc, read file, scan for ^#{1,4}\s+(.*) lines
   - Also parse YAML frontmatter title: and summary: as synthetic level-0 entries
   - Store heading text, level, line number
 - Add (idx *HeadingIndex) Search(query string, limit int) []HeadingMatch:
   - Tokenize query into lowercase keywords, filter stop words (a, an, the, to, in, of, and, for, how, is, by, with,
 using, that, does, do, can, what)
   - For each heading entry, score = count of query keywords found in lowercase(heading + " " + docTitle + " " +
 summary)
   - Sort by score desc, then by manifest order
   - Return top limit results
 - Add HeadingIndex field to DocsOverlay struct; populate in LoadDocsOverlay
 - Update SystemPromptSection():
   - Change each doc line from - Title (path) to - Title (path): Section1 | Section2 | ... using only ##-level headings
   - Update guidance text: "Browse the outlines below to find the right doc. Use search_docs with keywords to find
 specific subsections, then read_file with the returned path and line offset."

 internal/askplanner/tools/searchdocs.go

 - Rename parameter pattern → query (signals to LLM to use keywords, not regex)
 - Update Description(): "Search official TiDB SQL tuning docs by keywords. Matches doc titles, summaries, and section
 headings. Returns matching sections with file paths and line numbers for use with read_file."
 - Replace Execute() body:
   - Parse query from args (keep limit as-is)
   - Call overlay.HeadingIndex.Search(query, limit)
   - Format results grouped by doc, showing matching headings with line numbers
   - Include hint: Use read_file(path="contrib/tidb-docs/<path>", offset=<line>) to read the section.
 - Delete runScopedSearch() and parseSearchLine() (no longer needed)

 internal/askplanner/tools/docsindex_test.go

 - Add test TestBuildHeadingIndex: create temp docs with headings, verify extraction
 - Add test TestHeadingIndexSearch: verify keyword matching, scoring, stop-word filtering
 - Update TestLoadDocsOverlaySuccess: verify HeadingIndex is populated and SystemPromptSection includes |-separated
 outlines

 internal/askplanner/tools/searchdocs_test.go

 - Update TestSearchDocsToolExecute: use query parameter, test doc with headings, verify heading matches
 - Update TestSearchDocsToolNoMatches: use query parameter
 - Add test: multi-keyword scoring (more keywords matched = higher rank)
 - Add test: case insensitivity

 Example Walkthrough

 User asks: "how to reload binding"

 Before (grep):
 1. LLM calls search_docs(pattern="reload binding") → grep finds nothing → "No matches found."

 After (heading index):
 1. LLM sees in prompt: SQL Plan Management (sql-plan-management.md): SQL binding | Baseline capturing | Cross-database
 binding | Baseline evolution | Upgrade checklist
 2. Either reads directly, OR calls search_docs(query="reload binding")
 3. Tool tokenizes → ["reload", "binding"], matches against headings:
   - "Cache bindings" (line 459): matches "binding" → score 1
   - "Create a binding" (line 31): matches "binding" → score 1
   - "SQL binding" (line 11): matches "binding" → score 1
 4. Returns grouped results with line numbers → LLM calls read_file at the right offset

 Verification

 1. go test ./internal/askplanner/tools/... — all tests pass
 2. Manual test with CLI: ask "how to reload binding" → should find sql-plan-management.md
 3. Manual test: ask "What optimizer hint controls join reorder?" → should find optimizer-hints.md and/or
 join-reorder.md
 4. Manual test: ask "Does prepared plan cache apply here?" → should find sql-prepared-plan-cache.md
 5. Check system prompt size: log.Printf the total system prompt length at startup, verify it fits comfortably in the
 model's context window
```

## more than 40 files in manifest
```
 Plan: Extend search_docs heading index to cover SQL statements

 Context

 The heading-based search_docs (already implemented) indexes only the 40 curated manifest docs. This is insufficient:
 1. Better answers: e.g., "how to reload binding" — the actual SQL syntax ADMIN RELOAD BINDINGS lives in
 sql-statements/sql-statement-admin.md:77-101, not in the 40 curated docs.
 2. SQL validity verification: The LLM needs access to SQL statement reference docs to verify whether a suggested SQL
 statement is valid TiDB syntax.

 The contrib/tidb-docs/sql-statements/ directory contains 162 .md files (~18K lines total), each with consistent YAML
 frontmatter (title, summary) and heading structure. These should be searchable via search_docs but NOT listed in the
 system prompt (too many — would blow the prompt budget).

 Approach

 Extend BuildHeadingIndex to accept extra search directories beyond the manifest docs. Files from extra dirs are indexed
  for heading search but don't appear in the system prompt outline.

 - Manifest docs (40 files): indexed + shown in system prompt with ##-level outlines
 - Extra dirs like sql-statements/ (162 files): indexed for search only, not in prompt
 - System prompt gets a short note: "SQL statement reference docs are also searchable via search_docs."

 Files to Change

 internal/askplanner/tools/docsindex.go

 1. Change BuildHeadingIndex signature to accept extra dirs:
 func BuildHeadingIndex(docs []DocEntry, docsDir string, extraDirs []string) *HeadingIndex
 2. Add helper indexDir — extract the per-file parsing loop (frontmatter + headings) into a reusable function:
 func (idx *HeadingIndex) indexFile(docsDir, relPath, docTitle string, trackOutline bool)
   - trackOutline=true for manifest docs (stores ##-level headings in idx.outlines)
   - trackOutline=false for extra dir files (no outline needed)
 3. Scan extra dirs — after indexing manifest docs, for each dir in extraDirs:
   - filepath.Glob(filepath.Join(docsDir, dir, "*.md")) to find all .md files
   - For each file, derive DocTitle from YAML frontmatter title: field (strip  | TiDB SQL Statement Reference suffix)
   - relPath = dir/filename.md (relative to docsDir)
   - Call indexFile with trackOutline=false
 4. Update LoadDocsOverlay call site:
 overlay.HeadingIndex = BuildHeadingIndex(docs, absDocsDir, []string{"sql-statements"})
 5. Update SystemPromptSection — add a note after the curated pages list:
 SQL statement reference docs (`sql-statements/`) are also searchable via `search_docs`.

 internal/askplanner/tools/searchdocs.go

 No changes needed — it already uses overlay.HeadingIndex.Search() which will now include sql-statements entries. The
 search output already shows DocPath which will naturally be sql-statements/sql-statement-admin.md.

 internal/askplanner/tools/docsindex_test.go

 - Add TestBuildHeadingIndexWithExtraDirs: create a temp sql-statements/ subdir with a sample doc, verify its headings
 are indexed
 - Add TestHeadingIndexSearchFindsExtraDirEntries: verify search finds headings from extra dir files
 - Update existing TestBuildHeadingIndex call to pass nil for extraDirs

 internal/askplanner/tools/searchdocs_test.go

 - Add TestSearchDocsToolFindsSQLStatements: create overlay with extra dir, search for "ADMIN RELOAD BINDINGS", verify
 it finds the sql-statement file

 Example Walkthrough

 User asks: "how to reload binding"

 After (with sql-statements indexed):
 1. LLM calls search_docs(query="reload binding")
 2. Tool matches against ALL indexed headings (manifest + sql-statements):
   - sql-statements/sql-statement-admin.md → "ADMIN BINDINGS related statement" heading (matches "binding")
   - sql-plan-management.md → "Cache bindings" (matches "binding")
   - sql-plan-management.md → "Create a binding" (matches "binding")
   - sql-statements/sql-statement-create-binding.md → title match (matches "binding")
 3. LLM reads the relevant sections and finds ADMIN RELOAD BINDINGS syntax

 Verification

 1. go test ./internal/askplanner/tools/... — all tests pass including new extra-dir tests
 2. Build: go build ./... succeeds
 3. Manual test: ask "how to reload binding" → should find both sql-plan-management.md AND sql-statement-admin.md
 4. Manual test: ask "CREATE INDEX syntax" → should find sql-statements/sql-statement-create-index.md
 5. Manual test: ask "EXPLAIN ANALYZE" → should find both curated explain-overview.md AND
 sql-statements/sql-statement-explain-analyze.md
```
