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

Imported books are `SourceDocument`s on the workspace. Creature and rule
records from a bestiary or PHB are library-scoped and cited by `SourceID`.
Each campaign enables the sources it uses. `@` and campaign lists resolve
against campaign + world-shared + enabled sources, not the entire library.
Ingest is a domain module (`internal/ingest`) with a CLI adapter. The TUI
opens a dedicated Import screen (library sources, 5e.tools catalog, file
browser), not a path overlay on the campaign wiki. Parser output is canon with
provenance; empty stat-block exports stay unknown (no AI fill). 5e.tools JSON
is fetched at import time and never vendored in git (D-023).

## D-022 — Import is a full-screen library view

Import sits beside the world/campaign picker: `I` from the picker or campaign
browser leaves the wiki and opens SOURCES | 5E.TOOLS | FILES. Esc returns to
wherever the owner came from. Enabling a source is a campaign action on that
screen (`e`).

## D-023 — 5e.tools adapter fetches by book id

The owner chooses which 5e.tools books to ingest (XMM, XPHB, XDMG, LMoP, …).
One `SourceDocument` per source id pulls every collection that book publishes
(bestiary, spells, items, classes, races, feats, adventure/book text, …).
`{@creature}` / `{@spell}` tags become `@` mentions. Raw JSON stays out of the
repository; a local 5e.tools checkout can be passed with `-data`.

## D-024 — SQLite FTS5 and vectors are a later search backend

Ingest writes `domain.Record` values (title, summary, body, tags, source).
Those records are the indexable unit. A later SQLite store can add FTS5 and
optional embeddings for AI retrieval without changing ingest or requiring
vectors to reconstruct canon. JSON workspace remains the inspectable slice
until that storage move (design: Storage strategy).

## D-025 — Imported wiki records file under source folders

Enabled sourcebooks do not flatten into campaign type lists. Records carry a
slash `Folder` (`Monster Manual (2025)/Creatures`). The typed list shows
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

Imported adventures file numbered rooms under the parent site (`LMoP/Locations/Phandalin`)
with short titles (`1. Stonehill Inn`), sorted with the overview first then numeric
order. Named characters become a single NPC (table role plus their own writeup) instead
of repeating as `Phandalin - 1` locations, stub NPCs, and copies inside the parent body.
Markdown tables get a blank line after them so following stats are not parsed as table rows.





