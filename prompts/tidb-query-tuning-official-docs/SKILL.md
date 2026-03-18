# Official TiDB SQL Tuning Docs

Use this overlay as the official-doc companion to the existing TiDB query tuning skill.

## Source Selection

1. Use official docs for documented SQL tuning behavior, syntax, optimizer hints, plan cache behavior, statistics guidance, and best practices.
2. Use the existing `contrib/agent-rules` tuning skill for investigation workflow, oncall patterns, customer precedents, and field-proven mitigation order.
3. Use TiDB source code for optimizer internals, implementation truth, and behavior that is unclear or incomplete in docs.

## Working Rules

- Treat the checked-out `contrib/tidb-docs` tree as version-sensitive.
- Prefer `search_docs` to find the right official page, then use `read_file` on the returned path for exact details.
- If official docs and source code appear to disagree, say so explicitly and explain which source you are using for which claim.
- Keep answers actionable: point to the exact hint, variable, SQL statement, or tuning concept described in the official docs.

## High-Value Topics

- Reading and interpreting `EXPLAIN` and `EXPLAIN ANALYZE`
- SQL logical optimization such as predicate pushdown, decorrelation, partition pruning, TopN pushdown, and join reorder
- SQL physical optimization such as index selection, statistics, extended statistics, distinct optimization, cost model, and runtime filter
- Execution plan control via optimizer hints, SQL plan management, optimization blocklist, and optimizer fix controls
- Prepared and non-prepared plan cache behavior
- Index advisor and practical SQL tuning workflow
