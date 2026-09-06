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
- Persist pane split positions and session pane layout cycle in
  `preferences.json`, separate from campaign `workspace.json`.
- Session scene/context panes driven by live campaign records and session
  location (`#location`, `current-scene` seed, present NPCs, open threads).
- Mouse-addressable session review: click context entities, campaign sections,
  `@` suggestions, and review pin/clear controls.
- Named pane split trees in `preferences.json` (browser + session) with
  persisted ratios and campaign/context visibility cycling via Ctrl+P.
- Click-to-focus session panes (accent border); Tab cycles focus; empty input
  always offers `@` / `$` / `#` starters with filtered completions.
- Denser campaign browser: typed section panes fill the terminal, with `+` to
  add panes and Ctrl+P to cycle section presets (Argus #25).
- Session reconciliation records derived on end-session without mutating the
  transcript; `r` opens approve/reject review (Argus #26).
- Delete (`d` confirm) and soft-supersede (`x`) for campaign entities.
- Planned session notes (`p`): markdown prep docs with `@` / `#location`,
  distinct from live transcripts; `s` starts live play seeded from those notes
  (Argus #28–#29).
- Entity create/edit uses a single markdown document (`type:` / `# Title` /
  summary / body) instead of form fields (Argus #33).
- Close focused type/session panes with `-`; delete and supersede require `y`
  confirmation (Argus #34).
- Library launch picker: choose world → campaign (vault-style); `b` switches
  back (Argus #35; decision #36 / D-014).
- In-campaign tree navigator: Sessions, Prep, and typed wiki sections as
  first-class branches with list|detail browsing (Argus #36; decision #35 /
  D-013).
- Tag and scope filters on the campaign tree (`f` / `o`); markdown `tags:`
  frontmatter (Argus #37).
- Context-sensitive `?` help overlay; footers stay short (Argus #38).
- Entity coherence spine: session `Links` + auto-associate, backlinks, session
  History (A+C), and `@` peeks (Argus #30, #40, #41; D-015).
- Prep notes drafted against prior live sits: PRIOR SITS chrome, location
  seed, Ctrl+P to cycle attachments (Argus #43; D-016).

## In progress

## Next

### Later

- Named collections as a later organization aid on top of tags (Argus #39).
- Deepen reconciliation: factual diffs, superseded history, and draft provenance.
- Add export/import coverage for worlds, campaigns, sessions, links, and
  workspace preferences.
- Add optional AI adapters and proposal approval flow after the factual path is
  stable.
- Optional: interactive pane reordering and fully recursive split editing UI.

## Deferred

- Planned notes that cover or follow multiple prior live sits (Argus #31).
- Post-session change view with rewind/fast-forward playback (Argus #32).
- SQLite operational store and FTS5 migration.
- Context-aware AI generators and rules/source ingestion.
- Web/API client over the shared domain services.
