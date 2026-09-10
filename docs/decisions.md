# Dungeon Decision Record

This is the repository-visible mirror of the decisions guiding the current
vertical slice. Argus remains the preferred shared board when its daemon is
available; this file keeps the reasoning inspectable and portable when it is
not.

## D-001 — Stable workspace; entity-level drafts

The application does not have global Prep/Live/Review modes. Preparation,
play, and reconciliation are activities in one workspace, while draft status
belongs to an individual entity. This preserves access to facts, notes, and
assistance during a session without mode switching.

## D-002 — Facts and AI proposals have separate write paths

AI can read assembled factual context and produce answers or proposals, but it
cannot write canon directly. A DM explicitly approves any proposed factual
change. Authority and source labels remain visible in search, review, and
session UI.

## D-003 — Transcript is durable and reversible

Submitted session entries are campaign evidence, not disposable chat. Entries
can be undone/restored, while derived rolls, links, and future reconciliation
remain associated with the original entry. The transcript itself is not
silently rewritten by AI extraction.

## D-004 — Mouse resizing changes complementary panes

Dragging a vertical gutter reallocates left/right width; dragging a horizontal
gutter reallocates upper/transcript height. The implementation clamps both
sides to usable minimums. This is the interaction contract for all future pane
layouts.

## D-005 — Layout preferences are not campaign data

Pane visibility, focus, ordering, and split positions belong to the owner's
workspace preferences and must be portable independently of factual records.
The current prototype keeps split positions in memory; persistence is a
follow-on task rather than an implicit claim of the JSON campaign store.

## D-006 — Session context is explicit and scoped

`@` links resolve in the active campaign and shared world; `$` creation and
`#random` generation inherit the active world, campaign, session, source, and
location context. Generated entities remain drafts/proposals until approved.

## D-007 — Agentic TUI validation uses in-process screencaps

Layout and interaction regressions are exercised through an in-process harness
that drives `Update`/`View`, asserts exact terminal fill, and writes plain-text
plus ANSI screencaps. This is preferred over flaky PTY golden files for agent
and CI use.

## D-008 — `#` rolls are linked transcript evidence

Dice and arithmetic `#` expressions are evaluated locally and stored on the
originating transcript entry with expression, faces, and total. They remain
session evidence until the DM later reconciles them; `#random` and `#location`
stay separate command verbs.

## D-009 — Layout is a named-pane split tree

Personal layout preferences store browser and session binary split trees of
named panes (nav, list, detail, campaign, context, transcript, input). Ratios
and leaf visibility persist in `preferences.json`. Legacy pane_layout/pane_split
integers and older dense typed-pane browser layouts migrate on load.

## D-010 — Planned session notes ≠ live sessions (superseded)

Superseded by D-012. Earlier wording treated planned and live as phases of one
session record.

## D-011 — Post-session playback is derived, not transcript rewrite

After play, viewing a session supports scrubbing derived beats (cast,
transcript, reconciliation) with rewind and fast-forward. That view is
derived from linked records and never mutates the original transcript.

## D-012 — Planned session notes are distinct prep documents

Planned notes are markdown prep docs (with `@` links and location markers) that
live play may consume as context. They are not live session records and do not
become one by flipping status. Each table sitting still gets its own immutable
transcript. One planned note may later follow or reference several prior sits,
and the same note may seed several later live sits without flipping into a
session record.

## D-013 — Campaign organization is a hierarchical library tree

Primary navigation inside a campaign is a tree of first-class branches
(Sessions, Prep, typed wiki sections), not more flat type panes. The browser
layout is IDE-style **nav | list | detail**. Tags and collections are secondary
aids layered on that tree (`f` tags, `c` collections).

## D-014 — Launch with a world/campaign picker

Dungeon opens like an Obsidian vault switcher: pick a world, then a campaign,
then work inside that context. `b` returns to the library picker to switch.
`e` renames the selected world or campaign. `d` then `y` deletes it (and that
scope's wiki, sessions, prep, and collections); `n` or Esc cancels. Preferences
may restore the last-opened campaign on relaunch, but switching is always
available by going back. In-campaign hierarchy (sections / sessions / prep)
comes after that entry gate.

## D-040 — Library vaults can be renamed or deleted

World and campaign names are labels on stable IDs. Delete from the picker is
hard and confirmed, like entity delete: it drops that vault's scoped wiki,
sessions, prep, and collections. World-shared rows survive campaign delete.
Sibling campaigns and other worlds stay.

## D-015 — Entity coherence spine

Entities are hubs. Sessions own durable `Links` (present/cast) while wiki
records stay global. Live `@` / `$` auto-associate into the session set.
Entity detail shows backlinks and a session-grouped change log (expandable
events later). References offer an inline peek so mentions carry context in
place. History is derived; transcripts remain immutable.

## D-016 — Prep notes are drafted against prior live sits

New planned notes attach recent ended sessions in the current campaign as
context (cast, location, last transcript beat). The attachment is a reference,
not a copy of the transcript. Ctrl+P cycles how many prior sits are attached.

## D-017 — Named collections sit on top of tags

Collections are campaign-scoped named groups of wiki record IDs. They filter
the existing tree; they are not a second navigator. `c` cycles the filter, `g`
creates a collection, and `a` toggles membership of the selected entity.

## D-018 — Session folders live inside the Sessions branch

The Sessions list is a collapsible folder tree, not a flat scroll of sits.
`SessionRecord.Folder` is an optional slash path (`Greywatch/Crypt`). Unfiled
sits group by started month (or `Unfiled` when undated). Folders are an
organization aid on that one branch — they do not replace the typed campaign
navigator (D-013). `m` files a sit or the sits under a selected folder header;
Enter on a header collapses or expands it; Enter on an ended sit still opens
playback. New live sits inherit the selected named folder.

## D-019 — Wiki mentions resolve in place and stay when targets vanish

`@` in a wiki record's summary or body is a derived reference, not a separate
edge table. Resolution uses the longest matching title or alias. If the target
is removed, the mention remains in the prose as unresolved and is listed as
missing so it can be fixed. Backlinks (wiki, prep, session) are followable
from entity detail; they do not replace the campaign tree. Enter on a
followable hop opens a preview buffer first (D-020).

## D-020 — Detail hops preview before navigating

Enter on a followable detail hop (`@` mention, backlink, CAST, history) opens
a floating preview buffer instead of jumping the navigator. The owner can
scroll and read, then Enter or click `[open]` to jump, or Esc / click outside
/ `[close]` to dismiss and keep their place. Broken mentions still only warn.
List Enter (playback, folder collapse, wiki edit) is unchanged.

## D-021 — Library sources, campaign enablement, ingest outside the TUI

Imported books are `SourceDocument`s on the workspace. 5e.tools **adventures**
are fetched and cached as read-only reference (D-037). They do not become wiki
rows. Local markdown FILES still become owner wiki records with `SourceID`.
Mechanical 5e.tools books (Monster Manual, Player's Handbook, and similar) are
not imported in this app. Each campaign enables the adventure sources it uses.
`@` and `/` search resolve against campaign + world-shared wiki first, then
enabled adventure caches as labeled **reference** (never wiki). Ingest is a
domain module (`internal/ingest`) with a CLI adapter. The TUI opens a dedicated
Import screen (library sources, 5e.tools adventure catalog, file
browser), not a path overlay on the campaign wiki. Parser output is canon with
provenance; empty stat-block exports stay unknown (no AI fill). 5e.tools JSON
is fetched at import time and never vendored in git (D-023).

## D-022 — Import is a full-screen library view

Import sits beside the world/campaign picker: `I` from the picker or campaign
browser leaves the wiki and opens SOURCES | 5E.TOOLS | FILES. Esc returns to
wherever the owner came from. Enabling a source is a campaign action on that
screen (`e`). 5e.tools Enter caches the selected adventure and enables it for
the current campaign. FILES markdown still classifies and writes wiki records.
There is no import agent and no `H` harness cycle (D-038). Structural classify
runs on markdown ingest before wiki rows are written. `d` then `y`
removes an ingested source from the library so it
can be ingested again.

## D-023 — 5e.tools adapter fetches by book id

The owner chooses which 5e.tools **adventures** to cache by catalog id.
Mechanical books are rejected. `{@creature}` tags in adventure JSON become
named reference hits for `@` peeks. Raw JSON stays out of the repository; a
local 5e.tools checkout can be passed with `-data`. The durable copy is the
local 5e.tools cache used by the Sources reader.

## D-024 — SQLite is the campaign store; JSON is export

Campaign data lives in `workspace.sqlite`: library, wiki, prep, sessions,
transcripts, reconciliations, collections, adventure books, and the FTS5
search index. JSON is the portable export/restore format and the one-time
migrate-from path (`workspace.json` → sqlite). Embeddings stay optional and
are not required to reconstruct canon.

## D-025 — Imported wiki records file under source folders

Enabled sourcebooks do not flatten into campaign type lists. Records carry a
slash `Folder` (`Test Bestiary (2025)/Creatures`). The typed list shows
campaign records loose at the top and source folders collapsed until Enter
expands them. List rendering only paints the visible window so large books
do not stall the TUI.

## D-026 — Wiki and prep panes render markdown

Entity detail, prep notes, link previews, and `@` peeks display Glamour-rendered
markdown (Tokyo Night, matching the TUI). Editors stay source so the DM can
still type `**`, headings, and `@` mentions. Rendering lives in the TUI; stored
records remain plain markdown strings. Rendered output is clamped to the pane
inner width. Tables that would wrap into a staggered grid flatten to labeled
pairs so ability scores and similar blocks stay readable without blowing layout.

## D-027 — Adventure sites nest; characters are one record

Imported adventures file numbered rooms and named sites under the parent chapter
(`The Hollow Crown/Locations/Millhaven`, `.../The Ambush/The Hideout`)
with short titles (`1. The Mill Inn`, `The Mill Inn`), sorted with the overview first then numeric
order. Named characters become a single NPC (table role plus their own writeup) instead
of repeating as `Millhaven - 1` locations, stub NPCs, and copies inside the parent body.
Markdown tables get a blank line after them so following stats are not parsed as table rows.
An opening **Introduction** chapter stays a note; it does not become the source title or a
second copy of the adventure tree. Markdown exports that start with `# Introduction` take
the book name from the filename.

## D-030 — Outline ingest plus structural classify, not per-book skip lists

Adventure ingest dumps a faithful outline. Types come from structure that is
the same across 5e books: front matter (introduction, preface, appendix),
numbered room headings, 5e.tools `section` nodes, and Name/Role tables.
Front-matter subtrees that reprint later chapters are omitted. Advice boxes
stay notes. Per-adventure title deny lists are not used. `dungeon dump` is a
headless inspect of the workspace. Markdown FILES ingest still classifies from
those structural rules before wiki rows are written. AI still must not
silently rewrite canon (D-002).

## D-028 — 5e mechanical data is not a Dungeon plugin

Adventures cite shared 5e.tools files (MM bestiary, items, spells). Those
mechanical books are **not ingested** in this app: no MM/PHB plugin, no
composed stat blocks on `@`. 5e.tools **adventures** are cached books for the
Sources reader and `@` / `/` reference peeks (D-037). The campaign wiki stays
the owner's notes (sites, NPCs, prep from markdown FILES). Raw JSON is never
vendored (D-023).

## D-029 — A 5e ruleset pack is out of this app

A D&D 5e / Next lookup plugin is not part of this workstation. Later games or
a separate tool may compose mechanical tables; Dungeon does not.

## D-031 — Detail pane windows overflow instead of clipping the tail

Pane bodies still fit inner height so borders never clip (Argus #27). When the
detail document is taller than that height, the pane windows a scroll offset
rather than dropping the unread tail (Argus #60). `PgUp`/`PgDn`, Home/End, and
the mouse wheel over the pane (or while it is focused) move the body. `j`/`k`
keep hop-row movement when REFERENCES/LINKED/CAST exist (Argus #61, D-020).

## D-032 — Adventure cache is ingest source; sqlite stores parsed books

5e.tools adventure JSON is fetched at ingest and cached on the owner's
machine. Parsed adventure books and their FTS hits live in `workspace.sqlite`
(D-024). Cached adventures still must not land on `Workspace.Records` (D-037).
Embeddings stay optional and are not required to reconstruct canon.

## D-033 — Classify before wiki write

Markdown FILES ingest always runs structural classify on new records **before**
they are appended to the workspace (front matter → notes, numbered rooms →
locations). 5e.tools adventures skip this path: they are cached books, not wiki
rows. There is no import agent (D-038). AI still must not silently rewrite
canon (D-002).

## D-034 — Adventure reference lookups require an enabled source

`@` peek, mention hops, suggestions, and `/` search only resolve adventure
reference hits from 5e.tools adventures that are in the library **and** enabled
for the current campaign. An empty enablement list loads nothing, even if
adventure JSON is already on disk from another campaign. Owner wiki wins on
name collision. Mechanical MM/PHB lookups are not offered.

## D-035 — Import does not run an agent harness

Import does not cycle Claude, OpenCode, Codex, or Cursor Agent. There is no
`-harness` flag and no `H` key. `dungeon dump` remains a separate inspect
command. Markdown classify is local and structural (D-033).

## D-036 — Import agents are out of this app

Dungeon does not shell out to an agent to rewrite ingested records. Markdown
FILES stay owner wiki after local classify. Cached adventures stay read-only.

## D-037 — 5e.tools adventures are cached reference books

Fetching an adventure stores JSON in the local 5e.tools cache and upserts a
`SourceDocument`. It does not materialize NPCs, rooms, or prep into the wiki.
The campaign tree **Sources** branch lists enabled adventure **titles**. The
center list nests **chapters** as collapsible folders; named people and rooms
sit inside a chapter. Detail renders **only the selected name**, not the whole
book. `@` peeks stay the small overlay and are read-only. The left tree does
not nest every heading.

## D-038 — Markdown FILES are wiki; 5e.tools adventures are not

A file the owner adds is theirs to edit. Published 5e.tools JSON is read-only
reference. Those write paths stay distinct (D-002).

## D-039 — Sources reader paints one named slice

Named hits nest under collapsible chapter folders in the Sources list. Enter
expands a chapter. Names that share the same text collapse to one row; extra
labels stay aliases for `@` and `/`. The detail pane Glamour-renders the
selected document, not the entire cached adventure.

## D-040 — Entity documents own scope and authority explicitly

The Markdown entity editor always exposes `scope: campaign|world` and an
`authority:` value. New entities default to a campaign-scoped draft. Saving
honors both fields and rejects unknown metadata values instead of silently
substituting defaults. Changing a campaign entity to world scope preserves its
world identity and clears campaign ownership; changing a world entity back to
campaign scope requires an active campaign in that same world. Entity detail
labels ownership as `WORLD SHARED` or `CAMPAIGN` so list filters cannot be
mistaken for persisted scope.

## D-041 — Planned notes remain visible during live capture

A live session started from planned notes uses the session's left upper pane as
a scrollable prep run sheet. The title stays visible while `j`/`k`,
`PgUp`/`PgDn`, and `Home`/`End` move through the rendered Markdown body. The
right pane continues to show scene and cast context, and the transcript remains
a separate pane. A session without linked prep keeps the campaign navigator.

## D-042 — Browser filters never define live session scope

List scope, tag, type, and collection filters only change the browser display.
Live `@` completion and resolution, location lookup, scene context, thread lists,
and type counts always use records visible to the active campaign: its own
records, world-shared records, and records from enabled sources. Expanding the
browser to world or library scope cannot expose another campaign's records to a
live session.

## D-043 — Search and wiki inspection are non-destructive overlays during live capture

`/` opens search while a session is live. The search and preview overlays float
over the session view; the transcript, its entries, and the pending capture text
are untouched. `Enter` on a result opens a read-only preview instead of
navigating the browser, and `Enter`, `Esc`, or `q` in that preview returns
directly to capture with the input refocused. Reconciliation results are refused
with an explanatory status because reviewing them requires leaving the session.
Inspection never ends capture, so a table question no longer costs the sit.

## D-044 — 5e-cli is an external data plane reached over its JSON boundary

Dungeon still ships no mechanical ruleset (D-029). Mechanical lookup is
delegated to the separate `5e` binary, invoked as a subprocess with `--json` by
`internal/fivecli`, so neither tool vendors the other's store and both stay
independently installable. MCP remains available later for agent tool use.

The adapter is optional by construction. A workstation without `5e` gets a
typed `ErrUnavailable`, never a crash, and `Adapter.Status` collapses an absent
binary and a misconfigured one into a single reportable state.

`5e doctor --json` exits nonzero while still printing a complete diagnostic
body, so a nonzero exit is not treated as failure on its own: the body is
decoded when present, and an `ExitError` carrying stderr is returned only when
the tool produced no JSON to interpret. A tool that is installed but not
ingested is a diagnosis to report, not an error to raise.

Readiness for lookup is not the tool's own `ready` flag. `5e doctor` reports
not-ready when the 5etools data directory is unset, but that only blocks
re-ingesting the index; search and get keep answering from the existing sqlite
index. `Doctor.CanLookup` therefore gates on the index, and `LookupSummary`
still surfaces the tool's remedy for the setup that is genuinely blocked, so a
working index is never reported to the DM as a broken installation.

Decoded fields follow the tool, not the shape that reads best here: `5e get`
prints `page` as a JSON number, so `fivecli.Page` accepts a number or a string.
A contract test runs the installed binary when one is present and skips
otherwise, which is what catches the JSON drifting from these structs.

Every run is bounded by a timeout and a `WaitDelay`, because killing the
subprocess does not close output pipes a grandchild still holds; without the
delay a wedged lookup outlives its own deadline and blocks the TUI.

## D-045 — Workspace saves are incremental, and the backup is throttled

`SQLiteStore.Save` used to delete every non-adventure row, drop and rebuild the
FTS table, and copy the whole database aside, on every call. Live capture saves
after each transcript entry, so the cost of typing one line grew with the size
of the campaign: at a thousand records a save took roughly 40ms, most of it
whole-file I/O.

The store now keeps a snapshot of what the open database holds — a digest per
entity row and per search document — and a save writes only the rows whose
digest changed, inside one transaction. Search documents are deleted by rowid
rather than by kind and id, because those columns are UNINDEXED in the FTS
table and matching on them would scan the whole index.

The snapshot is built by the first save after the database is opened, which
still rewrites everything. That keeps the fast path honest: the store only
claims to know the contents when it wrote them itself. Closing the store, a
`Reindex`, or any failed write clears the snapshot, so the next save rebuilds
from scratch rather than trusting a stale plan.

Backups are throttled to one per five minutes rather than one per save. Writes
are transactional, so the sidecar guards against a corrupt file or a mistaken
bulk edit, not against a torn write, and it does not need to track every
captured line.

What remains proportional to workspace size is serialization, not I/O: every
save still marshals each entity and builds each search document in order to
compare it. That is the same order as the validation pass a save already runs,
and removing it would mean threading change information down from the model
through the whole save API.

## D-046 — Git sync is a per-entity mirror, not the operational store

SQLite remains the campaign store (D-024, D-032). `dungeon sync` mirrors that
workspace into a git repository the owner already authenticates with, so a
second machine can pull the same campaigns. The repository is backup and
transport, not a second database Dungeon opens at runtime.

A single JSON export is one enormous blob: git cannot show which NPC changed,
and two machines that edited different records cannot merge. The mirror writes
one JSON file per entity under `workspace/`, so diffs stay readable and git
can merge edits that did not touch the same record. Files outside `workspace/`
belong to the owner and are never rewritten. Layout preferences stay on the
machine that owns them (D-005).

The workstation `git` binary is the client. SSH agents and credential helpers
already know the owner; a vendored library would not. Push refuses when origin
is ahead, so the other machine's work is not overwritten. Pull fast-forwards
when only the remote changed, merges when both machines edited distinct
records, and aborts on a same-file conflict without touching sqlite.
`dungeon sync pull -force` takes origin as-is.

## D-047 — Live rules lookup uses the search overlay's 5e rules scope

Mechanical lookup stays out of the campaign wiki (D-029). During play or prep,
`/` still opens the same non-destructive search overlay (D-043). **Ctrl+S** now
cycles a fourth scope, **5e rules**, that queries the external `5e` binary
through `internal/fivecli` instead of the local FTS index. Hits carry the
`reference` authority — never canon — open in the existing read-only preview,
and never write campaign canon. A workstation without `5e` installed keeps
working: the scope shows the typed setup diagnosis from `5e doctor --json`
rather than failing the TUI.

Every subprocess runs as a command, never inline in `Update`. Keystrokes only
schedule a 180 ms debounce, so typing a word costs one lookup instead of one
per character, and the `5e doctor` diagnosis is fetched once and cached per
configured binary. A wedged or missing tool can therefore delay a result line
but never freeze capture.

## D-048 — The 5e binary is configured in the app, not only on PATH

`,` opens a settings overlay (`Ctrl+G` from the rules search scope) that stores
the path to the external `5e` binary in `preferences.json` alongside pane
layout. Preferences are personal and machine-local, so the path belongs there
rather than in the campaign workspace, which syncs between machines. The
configured path wins over `DUNGEON_5E_BIN`, which still wins over `PATH`, so an
existing environment-based setup keeps working. Saving re-runs the diagnosis and
shows it in place, making the overlay the one screen that answers "why is rules
lookup not working here".

