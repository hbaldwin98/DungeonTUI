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
named panes (list, detail, campaign, context, transcript, input). Ratios and
leaf visibility persist in `preferences.json`. Legacy pane_layout/pane_split
integers migrate on load.

## D-010 — Planned sessions are draft documents live play starts from

Preparation is not a global app mode. A planned session is a draft session
document (markdown body, `@` associations, location markers). World/campaign
entities remain global; the session associates a subset for that night. Live
play starts from that planned context so the table sees organized scene
context rather than the whole wiki at once. One planned brief may span
multiple live timed runs; each run keeps its own immutable transcript.

## D-011 — Post-session playback is derived, not transcript rewrite

After play, viewing a session should support scrubbing what changed
(rewind/fast-forward over entity diffs and reconciliation items). That view is
derived from linked records and never mutates the original transcript.

