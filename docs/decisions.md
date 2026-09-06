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

After play, viewing a session should support scrubbing what changed
(rewind/fast-forward over entity diffs and reconciliation items). That view is
derived from linked records and never mutates the original transcript.

## D-012 — Planned session notes are distinct prep documents

Planned notes are markdown prep docs (with `@` links and location markers) that
live play may consume as context. They are not live session records and do not
become one by flipping status. Each table sitting still gets its own immutable
transcript. One planned note may later follow or reference several prior sits.

## D-013 — Campaign organization is a hierarchical library tree

Primary navigation inside a campaign is a tree of first-class branches
(Sessions, Prep, typed wiki sections), not more flat type panes. The browser
layout is IDE-style **nav | list | detail**. Tags and collections are secondary
aids layered on that tree later.

## D-014 — Launch with a world/campaign picker

Dungeon opens like an Obsidian vault switcher: pick a world, then a campaign,
then work inside that context. `b` returns to the library picker to switch.
Preferences may restore the last-opened campaign on relaunch, but switching is
always available by going back. In-campaign hierarchy (sections / sessions /
prep) comes after that entry gate.

