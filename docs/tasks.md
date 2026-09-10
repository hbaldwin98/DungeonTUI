# Dungeon Task Snapshot

This file mirrors the current implementation queue. It is intentionally small
and task-oriented; detailed product rationale lives in [design.md](design.md).

## Done

- Persistent typed entity records with authority markers and scoped search.
- Markdown entity creation and editing honors explicit `scope: campaign|world`
  and authority; shared-world records remain visible across sibling campaigns
  (Argus #92; D-040).
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
  `preferences.json`, separate from campaign `workspace.sqlite`.
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
- Live sessions started from prep show its rendered Markdown as a scrollable run
  sheet beside scene/cast context (Argus #93; D-041).
- Browser scope, tag, type, and collection filters no longer alter live
  references, suggestions, scene context, threads, or counts (Argus #94;
  D-042).
- `/` searches the wiki, prep, sessions, and transcripts during live capture and
  inspects hits in a read-only overlay without ending the sit (Argus #95;
  D-043).
- `internal/fivecli` runs the external `5e` binary over its `--json` boundary
  and reports typed setup diagnostics; the integration is optional and degrades
  to a typed unavailable state (Argus #96; D-044).
- Entity create/edit uses a single markdown document (`type:` / `# Title` /
  summary / body) instead of form fields (Argus #33).
- Close focused type/session panes with `-`; delete and supersede require `y`
  confirmation (Argus #34).
- Library launch picker: choose world → campaign (vault-style); `b` switches
  back; `e` renames; `d` then `y` deletes (Argus #35, #81; decision #36 / #86 /
  D-014, D-040).
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
- One prep arc can seed several live sits; prep detail lists LIVE SITS and
  later nights keep a timestamped title (Argus #31).
- Post-session playback: Enter an ended session to rewind/fast-forward derived
  beats without mutating the transcript (Argus #32; D-011).
- Named collections as a filter on the campaign tree (`c` / `g` / `a`)
  (Argus #39; D-017).
- Session folders: collapsible named paths inside the Sessions list, month
  buckets for unfiled sits, `m` to file (Argus #44; D-018).
- Wiki `@` mentions resolve in prose; missing targets stay broken; LINKED /
  REFERENCES / CAST / prep links preview from detail before navigating
  (Argus #45–#46; D-019, D-020).
- Library source corpus: ingest markdown or 5e.tools books into
  `SourceDocument`s, campaign enablement, and `@` association (Argus #47–#54;
  D-021–D-023). Dedicated Import screen with a 5e.tools catalog (`I`)
  (Argus #51; D-022). `d` then `y` on SOURCES removes a book so it can be
  ingested again (Argus #61). Campaign data and FTS5 search live in
  `workspace.sqlite`; JSON is export/migrate (Argus #55, #107; D-024, D-032).
  Imported records file under source/type folders, collapsed by default
  (Argus #56–#57; D-025).
- Wiki/prep detail, link previews, and `@` peeks render markdown with
  Glamour; the editor stays source (Argus #58; D-026). Wide tables flatten
  and rendered lines clamp to the pane so complex stat blocks cannot stagger
  borders (Argus #60).
- Adventure ingest nests related rooms, keeps one NPC record per person, and
  pads tables so following lines stay out of the grid (Argus #59; D-027).
  Introduction stays a note; markdown that starts with that heading uses the
  filename as the book title (Argus #62).
- D&D 5e plugin and import agent removed from this app: 5e.tools adventures
  cache as read-only reference; MM/PHB is not ingested; `@` / `/` hit enabled
  caches as **reference** (wiki wins); Sources groups names under chapter
  folders, collapses duplicate names onto one document, and detail renders the
  selected slice (Argus #72–#76; D-028, D-034, D-037–D-039).
- Headless `dungeon dump` inspects the workspace; markdown FILES classify from
  structure **before** wiki rows are written (Argus #64, #67; D-030, D-033).
  The `classify` CLI and import-agent harness are removed (Argus #99).
- Campaign search covers wiki, prep, sessions, transcripts, reconciliation, and
  labeled adventure references (Argus #94). SQLite holds campaign data and FTS
  in `workspace.sqlite` (Argus #55, #107).
- Reconciliation applies editable, source-linked wiki mutations; the transcript
  stays immutable (Argus #95).
- Session and record transactions live in `internal/app` (Argus #97).
- Empty first-run library, `dungeon export` / `dungeon restore`, MIT license,
  `make build`, and GitHub Actions CI (Argus #100).
- SQLite is the campaign store (`workspace.sqlite`); JSON is export and
  migrate-from (Argus #107).

## In progress

## Next
### Later

- Add optional AI adapters and proposal approval flow after the factual path is
  stable.
- Optional: interactive pane reordering and fully recursive split editing UI.

## Deferred

- Optional embeddings later (JSON export remains; sqlite is the operational
  store, Argus #55, #107; D-024, D-032).
- Context-aware AI generators.
- Web/API client over the shared domain services.
