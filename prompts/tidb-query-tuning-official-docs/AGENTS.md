---
name: tidb-query-tuning-official-docs-maintenance
description: Maintainer notes for the local official-doc SQL tuning overlay used by askplanner.
---

# Official SQL Tuning Docs Overlay

This directory contains askplanner-local prompt assets derived from the official TiDB docs tree in `contrib/tidb-docs/`.

## Source of Truth

Only use the self-managed `SQL Tuning` section in `contrib/tidb-docs/TOC.md` plus `contrib/tidb-docs/sql-tuning-best-practice.md`.

## Out of Scope

- TiDB Cloud docs and Cloud TOCs
- Release notes
- Media assets
- Unrelated docs outside SQL tuning

## Files

- `SKILL.md`: concise runtime prompt overlay loaded into the system prompt when available
- `manifest.json`: curated list of official docs pages searchable by the `search_docs` tool

## Maintenance Rules

- Keep `SKILL.md` short and complementary to the existing `contrib/agent-rules` tuning skill.
- Prefer summarizing official docs guidance instead of copying full page content.
- Keep every `manifest.json` entry relative to `contrib/tidb-docs`.
- Validate that every manifest path exists when updating the overlay.
- Missing or invalid overlay assets must not block startup. The app should warn and continue without this overlay.
