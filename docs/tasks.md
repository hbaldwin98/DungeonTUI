# Dungeon Task Snapshot

This file mirrors the current implementation queue. It is intentionally small
and task-oriented; detailed product rationale lives in [design.md](design.md).

## Done

- Persistent typed entity records with authority markers and scoped search.
- Campaign/entity browser with list/detail panes and mouse navigation.
- Session start/end lifecycle and durable multiline transcript.
- Plain Enter submission; Shift+Enter newline; undo/restore submitted entries.
- `@` autosuggestions, durable links, and live entity review pane.
- `$` draft entity creation with session provenance.
- Local `#random` drafts and `#location` session context.
- Scrollable transcript viewport and mouse-wheel navigation.
- Vertical and horizontal gutter dragging with complementary pane resizing.
- In-process agentic TUI harness with keyboard/mouse driving and text/ANSI
  screencaps (`go run ./cmd/dungeon-harness -scenario all`).
- Local `#` dice and arithmetic evaluation with results linked to transcript
  entries (`#d20+5`, `#damage 2d6+3`, `#10+2*3`).

## In progress

- General configurable pane system: named pane types, visibility, ordering,
  focus, and arbitrary two-axis split tree.
- Persist personal layout preferences separately from campaign JSON.
- Replace static session scene/context fixture content with dynamic notes,
  character briefs, threads, location, and current-scene records.

## Next

- Make session commands and review actions fully mouse-addressable across all
  pane types.
- Add session reconciliation records without mutating the raw transcript.
- Add export/import coverage for worlds, campaigns, sessions, links, and
  workspace preferences.
- Add optional AI adapters and proposal approval flow after the factual path is
  stable.

## Deferred

- SQLite operational store and FTS5 migration.
- Context-aware AI generators and rules/source ingestion.
- Web/API client over the shared domain services.
