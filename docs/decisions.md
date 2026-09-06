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

