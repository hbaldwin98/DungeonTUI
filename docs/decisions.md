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
Preferences may restore the last-opened campaign on relaunch, but switching is
always available by going back. In-campaign hierarchy (sections / sessions /
prep) comes after that entry gate.

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

Imported books are `SourceDocument`s on the workspace. Mechanical 5e.tools
books (Monster Manual, Player's Handbook, and similar) prime the D&D 5e plugin
and stay out of wiki folders. Adventure prose becomes campaign wiki records
with `SourceID`. Each campaign enables the sources it uses. `@` and campaign
lists resolve against campaign + world-shared + enabled sources, then the
5e plugin for books that campaign has imported and enabled. Ingest is a
domain module (`internal/ingest`) with a CLI adapter. The TUI opens a dedicated
Import screen (library sources, 5e.tools catalog, file
browser), not a path overlay on the campaign wiki. Parser output is canon with
provenance; empty stat-block exports stay unknown (no AI fill). 5e.tools JSON
is fetched at import time and never vendored in git (D-023).

## D-022 — Import is a full-screen library view

Import sits beside the world/campaign picker: `I` from the picker or campaign
browser leaves the wiki and opens SOURCES | 5E.TOOLS | FILES. Esc returns to
wherever the owner came from. Enabling a source is a campaign action on that
screen (`e`). `H` cycles an optional cleanup harness after ingest (`off` / `on`).
On means retype the ingested records from structure and import that cleaned
wiki; harnesses do not write `ingest-dump.json` (D-035). Structural classify
always runs on new records before they are written. `d` then `y`
removes an ingested source from the library so it
can be ingested again; session CAST links keep their record IDs because those
IDs are stable per book and title.

## D-023 — 5e.tools adapter fetches by book id

The owner chooses which 5e.tools books to ingest by catalog id.
One `SourceDocument` per source id pulls every collection that book publishes
(bestiary, spells, items, classes, races, feats, adventure/book text, …).
`{@creature}` / `{@spell}` tags become `@` mentions. Raw JSON stays out of the
repository; a local 5e.tools checkout can be passed with `-data`.

## D-024 — SQLite FTS5 and vectors are a later search backend

Ingest writes `domain.Record` values (title, summary, body, tags, source).
Those wiki records, and the composed 5e plugin entries (D-032), are the
indexable units. A later SQLite store can add FTS5 and optional embeddings for
AI retrieval without changing ingest or requiring vectors to reconstruct canon.
JSON workspace remains the inspectable campaign slice until that storage move
(design: Storage strategy).

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
headless inspect of the workspace, not the import harness. Import harnesses
clean ingested records and import the cleaned wiki (D-035). AI still must not
silently rewrite canon (D-002); cleanup only retypes from those structural
rules.

## D-028 — 5e mechanical data is a lookup plugin

Adventures cite shared 5e.tools files (MM bestiary, `template.json`,
`legendarygroups.json`, `items.json`, `items-base.json`, `magicvariants.json`,
fluff). Those are a **ruleset plugin** (`internal/ruleset`, first pack
`dnd5e`), not wiki folders. The campaign wiki stays narrative (sites, NPCs,
prep). `@` peek, mention hops, suggestions, and search display composed
markdown from the plugin when no wiki record matches. Importing MM, PHB, or
DMG from the 5e.tools catalog primes that plugin (no wiki tree) and enables
it for the current campaign. Other campaigns must enable those sources;
importing an adventure does not grant MM/PHB lookups (D-034). The plugin
corpus is pulled onto the local machine and persisted there (D-032); it is
not an in-memory-only catalog. Raw JSON is never vendored (D-023).

## D-029 — First ruleset pack is D&D 5e / Next (2014)

The first plugin targets D&D Next / 5e (2014) core tables. 2024 book codes are
additional ids in the same plugin when those sources are enabled, not a second
system. Later games get their own plugin behind the same lookup interface.

## D-031 — Detail pane windows overflow instead of clipping the tail

Pane bodies still fit inner height so borders never clip (Argus #27). When the
detail document is taller than that height, the pane windows a scroll offset
rather than dropping the unread tail (Argus #60). `PgUp`/`PgDn`, Home/End, and
the mouse wheel over the pane (or while it is focused) move the body. `j`/`k`
keep hop-row movement when REFERENCES/LINKED/CAST exist (Argus #61, D-020).

## D-032 — 5e plugin is local durable data, then SQLite FTS5

The D&D 5e plugin is fetched (5e.tools JSON at ingest) and stored on the owner’s
machine. Process maps are a working cache rebuilt from that disk copy, not the
system of record. Plugin entries still must not land on `Workspace.Records`
(D-028). The same later SQLite FTS5 store that indexes wiki ingest (D-024,
Argus #55) will index plugin creatures/items too; embeddings stay optional and
are not required to reconstruct canon.

## D-033 — Classify before wiki write

Ingest always runs structural classify on new records **before** they are
appended to the workspace (front matter → notes, numbered rooms → locations).
The Import `H` cycle and `dungeon import -harness` clean those ingested
records and import the cleaned wiki (`off` / `on`). They do not write an
agent dump file (D-035). AI still must not silently rewrite canon (D-002).

## D-034 — 5e plugin lookups require an imported, campaign-enabled source

The D&D 5e plugin is not attached to every campaign. `@` peek, mention hops,
suggestions, and search only resolve plugin creatures, spells, items, and
terms from 5e.tools books that are in the library **and** enabled for the
current campaign. An empty enablement list loads nothing, even if MM/PHB
JSON is already on disk from another campaign. Adventures stay wiki records;
they do not auto-load the core bestiary. Import those mechanical books (and
enable them) when a campaign needs the stats.

## D-035 — Import harness cleans ingested records; it does not dump

The Import `H` cycle and `dungeon import -harness` are a cleanup pass, not an
agent dump. **On** retypes leftover ingested rows from the same structural
rules used at ingest, then those cleaned records are the import. No
`ingest-dump.json` is written. `dungeon dump` remains a separate inspect
command for reading the workspace.








