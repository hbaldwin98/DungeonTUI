# Dungeon: Product and System Design

## Status

This document captures the current product direction for Dungeon, a personal,
terminal-native workstation for running tabletop role-playing campaigns. It is
an evolving design rather than a fixed implementation specification.

## Product thesis

Dungeon is a campaign operating system for a human Dungeon Master. It helps
prepare sessions, supports improvisation and reference work during play, and
turns session notes into an accurate, durable campaign record afterward.

The application combines two capabilities that must remain visibly and
architecturally distinct:

1. A factual campaign wiki and structured data store controlled by the DM.
2. AI assistance that can retrieve facts, reason about them, and offer ideas,
   but cannot silently make those ideas true.

The central rule is:

> AI may generate possibilities. Only the DM establishes facts.

Dungeon is built for its owner first. It does not need to anticipate every
table, rules system, deployment environment, or collaboration model. It should
be pleasant to modify, run locally, and fit the owner's actual habits. This
personal-first focus is compatible with good boundaries and portable data; it
does not require premature product generalization.

## Goals

- Provide one coherent workflow across preparation, live play, and review.
- Make factual information immediately distinguishable from AI output.
- Organize campaign information into clear, navigable sections.
- Support multiple worlds and multiple campaigns, including several campaigns
  sharing one world.
- Search quickly across entities, notes, sessions, rules, lore, and AI material,
  with the search scope and result type always apparent.
- Preserve source, authority, visibility, and history for important facts.
- Keep all important user data locally owned, inspectable, exportable, and
  independent of the TUI.
- Use AI to reduce clerical and recall burdens without diluting DM authorship.
- Allow a future web, desktop, or API interface to use the same domain model.

## Non-goals

- Replacing the DM or autonomously running the campaign.
- Automatically deciding plot developments or resolving deliberately unknown
  parts of the world.
- Treating generated prose as canon because it appeared in an answer.
- Building multi-user SaaS infrastructure before the personal workflow works.
- Supporting every RPG system in the first version.
- Shipping copyrighted commercial sourcebook content with the application.
- Making chat the primary organizing metaphor.

## Core product principles

### Facts and assistance are different systems

The factual system is the campaign record. Its contents are intentionally
authored or approved by the DM. It should remain useful with AI completely
disabled.

The assistance system consumes selected factual context and external knowledge.
It returns answers, interpretations, summaries, or proposals with provenance.
Its output lives outside the factual store until the DM performs an explicit
approval action.

Visual styling alone is not sufficient separation. The write path must enforce
the boundary: the AI subsystem has no direct `writeCanon` capability.

```text
AI -> read/search facts -> draft proposal
DM -> inspect/edit/approve proposal -> commit factual change
```

Direct manual editing remains available. The owner should never need AI merely
to add or correct a fact.

### Unknown is meaningful

An absent fact is not an invitation for the model to complete the pattern.
Unknown or deliberately unresolved questions should be representable directly.
When an answer depends on one, Dungeon should say so and may offer clearly
labelled possibilities.

### Provenance travels with information

Important information should answer:

- What is asserted?
- Who or what established it?
- When was it established?
- Where did it come from?
- Is it true, tentative, generated, or unknown?
- Who in the game knows it?
- Has a later event overridden it?

### One workspace, not global workflow modes

Preparation, play, and post-session reconciliation are activities within the
same stable workspace. Dungeon must not force the entire application into
Prep, Live, or Review modes. Search, notes, entities, rules, and assistance
remain available at all times.

Draftness belongs to an individual entity. A draft NPC or location can sit
beside established campaign facts while remaining visibly non-canon. Session
notes can similarly produce proposed changes without changing the state of the
whole application.

### Local-first and personal-first

The default experience should require no server operated by someone else. Local
storage, local indexing, direct filesystem backups, and configurable model
providers are preferred. Convenience for the owner wins over abstractions added
only for hypothetical customers.

Personal-first does not mean tightly coupling the domain to terminal rendering.
Clean boundaries make the personal tool easier to maintain and leave room for a
web interface later.

## Information model

### Library, worlds, and campaigns

The top-level library contains shared reference material and any number of
worlds.

```text
Library
|- Rules and sourcebook knowledge
|- World: The Ashen Realms
|  |- Shared world canon
|  |- Shared entities and chronology
|  |- Campaign: The Ashen Crown
|  `- Campaign: Embers in the North
`- World: Barovia
   `- Campaign: Mists of Ravenloft
```

A **world** owns facts intended to persist across campaigns: geography,
cosmology, factions, historical events, reusable NPCs, house rules, and setting
lore.

A **campaign** belongs to exactly one world. It owns the party, sessions,
campaign-specific events, active threads, deviations from the world's baseline,
and campaign knowledge state.

Campaigns in one world may share a baseline without automatically sharing all
changes. This avoids one campaign accidentally rewriting another. Cross-campaign
effects should be deliberate. The initial implementation can use a simple
policy:

- World records are the shared baseline.
- Campaign records and overrides are scoped to one campaign.
- Promoting a campaign event into shared world history is an explicit action.

The architecture should not assume that two campaigns occur at the same point
in the world's timeline.

### Information authority

Every relevant record or assertion has an authority state:

| State | Meaning |
|---|---|
| Canon | Established as true by the DM or an approved play event. |
| Secret canon | True, but not generally known to the players. |
| Draft | Created or prepared by the DM but not committed as truth. |
| AI proposal | Generated material with no factual authority. |
| Unknown | Explicitly undecided or currently unknowable. |
| Superseded | Previously valid but replaced by a later fact or event. |

“Secret” is also a visibility property, so the mature model may separate
authority from audience. The initial UI may present the combined labels above
because they match how a DM thinks during play.

Suggested consistent indicators:

```text
● canon        ◆ secret        ◇ draft
△ AI proposal  ? unknown       × superseded
```

AI-generated summaries and answers are not records of truth either. Even when
fully grounded in canon, they should retain an `AI answer` label and citations
to the facts they summarize.

### Entities and assertions

Core entity types are likely to include:

- World
- Campaign
- Session
- Scene
- Character
- NPC
- Location
- Faction
- Item
- Creature
- Thread or quest
- Event
- Rule or house rule
- Note
- Proposal
- Source document

Entities provide organization, but individual assertions need provenance. An
NPC should not be stored only as one opaque generated biography. Facts such as
current location, allegiance, possession, and attitude may change independently.

An assertion can conceptually contain:

```yaml
subject: npc.osric-pell
predicate: possesses
object: item.silver-key
authority: canon
scope: campaign.ashen-crown
source:
  kind: session-event
  ref: session.017.event.23
established_at: 2026-09-05
```

This does not require exposing triples or YAML in the normal UI. It defines the
semantics needed for continuity, citations, history, and safe AI retrieval.

Entity types remain visibly distinct in navigation, search results, and editing
forms. An NPC, session, event, rule, and note should never look like generic
untagged documents. Type-specific fields and actions may share implementation,
but the UI must preserve the user's understanding of what kind of thing they
are viewing or creating.

### Events and history

Session play produces events. Events can support one or more state changes and
retain the evidence from which they were derived.

For example, the quick note:

```text
Lena gave Osric the silver key
```

may produce a review item:

```text
+ Osric possesses the Silver Key          high confidence
+ Osric knows Lena possessed it           high confidence
? Lena no longer possesses the key        needs confirmation
```

Approved changes enter the factual system. The raw note and review decision
remain available as provenance. This produces an auditable campaign history
without requiring heavyweight version-control concepts in the interface.

### Knowledge and visibility

World truth must be separate from knowledge:

- What is true in the world?
- What does the party collectively know?
- What does an individual player character know?
- What does an NPC or faction know?

This enables safe questions such as “What can I remind the players about?” and
prevents secret context from leaking into player-facing output.

## Knowledge sources and precedence

Dungeon reasons over distinct bodies of information rather than blending them
into a single undifferentiated prompt.

Recommended precedence:

1. Actual events and approved campaign canon
2. Campaign-specific overrides and house rules
3. Shared world canon
4. Current session and scene state
5. Enabled rules edition and sourcebooks
6. General model knowledge

The precedence used for an answer should be inspectable. If campaign canon
overrides a published rule or adventure fact, the answer should call that out.

Rules answers should distinguish:

- **Rule:** directly supported by an enabled source.
- **Interpretation:** reasoning where the source does not determine the result.
- **House rule:** an explicit campaign or world override.

Sourcebook ingestion should preserve document identity, edition, page, heading
hierarchy, blocks, tables, and extracted entity type. Retrieval will eventually
combine structured filters, full-text search, and semantic search. The first
version should ship only open or appropriately licensed material; personally
owned sources may be imported locally and should never be redistributed by the
application.

## Information architecture

The stable sections should reflect data concepts, not AI features. A likely
campaign navigation tree is:

```text
Dashboard
Session
World
NPCs
Characters
Locations
Factions
Items
Threads
Timeline
Knowledge
Sessions
Rules
Sources
Proposals
```

AI is invoked contextually from these sections. There is no need for a permanent
“AI world” parallel to the factual wiki. Selecting Captain Vale and requesting
dialogue help should preserve Captain Vale as the visible subject, show exactly
which facts were provided, and place the result in a proposal or transient
answer panel.

The Proposals section acts as an inbox for generated possibilities and suggested
changes. It is explicitly outside canon.

## TUI interaction model

### Stable workspace and contextual actions

The application opens into one campaign workspace. The current selection
determines available actions; the user never has to switch the whole interface
into a workflow mode.

From that workspace the DM can, at any time:

- Review open threads, likely consequences, and stale entities.
- Create or edit draft entities for an upcoming session.
- Pin selected facts and references to a session brief.
- Capture terse session notes with minimal keystrokes.
- Search facts and sources quickly.
- Ask contextual questions about the selected subject.
- Generate clearly separate AI proposals.
- Convert notes into proposed events and state changes.
- Approve, edit, reject, or defer individual changes.

Creating content starts it as a draft unless the DM explicitly creates or
promotes it as canon. Draft status is shown on the entity itself and does not
restrict access to any other feature.

### Configurable panes

The workspace is composed of panes rather than a fixed dashboard layout. The
owner can show, hide, resize, reorder, and focus panes such as the entity list,
entity detail, session transcript, open threads, search results, dice/tool
output, and AI proposals. Pane configuration is a personal workspace
preference, not campaign data, and should be portable separately from factual
records. Every pane still reports its section and entity type clearly when it
has focus.

### Session capture

A campaign can have an explicit session lifecycle. Starting a session creates a
session record and opens a transcript pane with a focused multiline input. The
DM can type player actions, rulings, descriptions, and outcomes continuously
without leaving the campaign workspace. Ending the session timestamps and
closes capture while preserving the transcript as campaign data for later
reconciliation.

Transcript entry must be reversible. The input editor supports character-level
undo and redo before submission. After a message is submitted, the DM can
undo, edit, or restore that message without losing the original text. This
covers accidental submissions, bad rolls, duplicate entries, and corrections
made during a hectic table moment. Storage may represent this as revisions or
tombstones, but the normal view shows the current restored version and history
remains available when needed.

Transcript references are lightweight links resolved against the active
campaign and its shared world records:

```text
@Captain Vale searches the reliquary while @Mira checks the eastern transept.
The key opens the seal. #d20+5 #damage 2d6+3
```

`@` references should offer fuzzy completion and create durable links to
characters, NPCs, locations, items, threads, events, or other entities. A
reference must retain the displayed text and the resolved record ID so renamed
entities do not break old transcripts. `#` expressions invoke a local command
or calculation parser: at minimum dice notation, arithmetic, and common
modifiers. Results are recorded alongside the originating transcript entry,
with the roller, expression, and result visible; they never silently become
canon. Unsupported or ambiguous expressions remain plain text until the DM
confirms them.

The transcript also provides fast creation and generation commands:

```text
$npc Sister Elayne: A church investigator arriving after the reliquary incident.
$item Ashen Compass: Points toward the oldest unquiet dead.
#location Ruined Monastery
#random npc
#random item
```

`$` creates a new entity draft from the active session. The command parser
recognizes the entity type, name, and description, then fills campaign, world,
session, source, and timestamps from the current game context. The resulting
entity is immediately linked back to the transcript entry and remains a draft
until the DM promotes it. Invalid types or incomplete commands should open a
small correction prompt rather than silently creating a generic record.

`#random <entity-type>` creates a context-aware draft generator result for
things such as characters, NPCs, creatures, items, locations, or clues. The
generator uses the active campaign and current session context, including the
current location when available, so results should fit the game rather than be
random library-wide noise. `#location <entity-or-name>` sets or changes the
session's current location context; `#location CURRENTLOCATION` displays the
currently active location. Location changes are session state and do not by
themselves establish a campaign fact.

Generators have two explicit implementations: a deterministic local generator
that works offline, and an optional AI-guided generator that receives only the
assembled campaign context. Both produce drafts or AI proposals, never canon,
and the transcript records which generator produced the result, its context,
and its seed or provider metadata for reproducibility.

### Live entity review

During an active session, the transcript is paired with an entity review pane
above the chat input. Resolving an `@` reference, creating an entity with `$`,
or accepting a generator result can automatically open that entity in the pane
so the DM can immediately read its full details, description, authority, and
source without leaving the session. The pane should show the entity type and
scope prominently, preserving the factual-versus-draft distinction at the
table.

Automatic opening changes the review selection, not the transcript focus. The
DM can pin an entity, return to the previous entity, close the review pane, or
disable automatic opening for the rest of the session. Multiple recently
opened entities may be kept as a short stack or tab strip, with the layout
controlled by the configurable pane preferences.

Session capture is deliberately factual-data-first. AI may later summarize a
transcript or suggest events, but those outputs become labelled proposals and
must not alter the transcript or campaign facts automatically.

### Contextual AI

AI should feel like an action on selected data rather than a disconnected chat.
Examples:

- On an NPC: possible response, knowledge check, tactical behavior.
- On a location: sensory description, plausible inhabitants, continuity check.
- On a thread: missing clues, consequences, related events.
- On a creature: rules, tactics, encounter considerations.
- On a session: summarize notes, identify changes, produce a recap.

Each response should show its class, such as `FACTUAL ANSWER`, `RULE`,
`INTERPRETATION`, or `AI PROPOSAL`, along with the context and sources used.

## Search design

Search is a primary interaction, not a secondary form. It should combine the
speed and ergonomics associated with `fzf` and `rg` while maintaining domain
awareness.

### Search requirements

- Open instantly from anywhere.
- Fuzzy-match titles, aliases, tags, and identifiers.
- Full-text-search notes, descriptions, session logs, and imported sources.
- Filter by world, campaign, section, entity type, authority, visibility,
  source, and date/session.
- Search the current campaign by default, with obvious controls for the world,
  all campaigns in the world, or the entire library.
- Clearly indicate the active scope and search mode before results appear.
- Clearly label every result with type, scope, authority, and source.
- Never mix AI proposals into factual results without an unmistakable marker.
- Offer fast keyboard narrowing similar to fzf, without requiring the user to
  memorize query syntax.

Example overlay:

```text
SEARCH  scope: Ashen Crown  data: facts + sessions  types: all
> silver

NPC       ●  Silver-Eyed Maren       world: Ashen Realms
ITEM      ●  Silver Key              campaign: Ashen Crown
THREAD    ●  Missing Reliquary       campaign: Ashen Crown
SESSION   ●  "...found a silver key" S17 note
RULE      ●  Silvered Weapons        PHB 2024 p. ___
PROPOSAL  △  Silver key opens crypt  AI proposal, not factual

Tab scope  Ctrl+T types  Ctrl+A authority  Ctrl+S sources  Enter open
```

The header prevents ambiguity about what is being searched. Result prefixes and
authority glyphs prevent ambiguity about what was found.

Optional expert query tokens may accelerate repeated searches:

```text
type:npc vale
scope:world authority:canon reliquary
in:sessions silver key
source:phb restrained
is:proposal investigator
```

These tokens should complement interactive filters rather than replace them.

### Search implementation direction

Use a common search service returning typed results regardless of UI. Start with
SQLite FTS5 and structured metadata filters. Add embeddings only for queries
where semantic recall materially improves the result. Exact rules lookup and
mechanical searches should prefer structured and full-text retrieval.

The search result contract should contain enough metadata for any client:

```text
id, title, snippet, entity_type, scope, authority,
visibility, source, rank, matched_fields
```

## Portability and architecture

The TUI is the first and primary interface, not the owner of the domain.

```text
TUI
 |
Application services
 |
Domain model
 |
Repositories / search / AI adapters
 |
Portable local data
```

The Bubble Tea model should coordinate presentation and emit typed application
commands. It should not contain canonical business rules, directly manipulate
database tables, or be the only representation of state transitions.

A future web app should be another client of the same application semantics:

```text
                  +-- TUI
Domain services --+-- future HTTP/API -- web UI
                  `-- import/export tools
```

### Storage strategy

SQLite is a pragmatic eventual system of record: transactional, local,
portable, easy to back up, and capable of structured queries plus FTS5.
The first persisted vertical slice uses a documented JSON workspace because it
keeps the data inspectable while the entity model is still changing. The
storage boundary must allow that implementation to move to SQLite without
changing domain or TUI code. Portability does not require every internal record
to be a Markdown file.

The application should provide versioned, documented export and import formats.
A useful export bundle may contain:

- Human-readable Markdown for wiki content and session records.
- YAML or JSON for structured entities, assertions, relations, and provenance.
- Original user-supplied attachments or stable references to them.
- A manifest containing schema version, world/campaign IDs, and checksums.

SQLite is the operational representation; the export bundle is the durable
interchange representation. Both should remain accessible without an AI service.

Schema migrations must be explicit and backed up. AI provider IDs, prompt traces,
and cached embeddings are derived or operational data and should not be required
to reconstruct the factual campaign record.

### AI boundary

The AI adapter may receive only explicitly assembled context. Its capabilities
should look conceptually like:

```text
searchFacts
readEntity
readSession
searchRules
writeScratch
createProposal
```

It must not receive a direct mutation capability for canon. Proposal approval is
an application command initiated by the user and validated outside the model.

The model provider should be replaceable. The owner may choose a hosted or local
OpenAI-compatible endpoint, or disable assistance while retaining all wiki,
session, and search functionality.

## Phased delivery

Each phase should leave a usable personal tool rather than only infrastructure.

### Phase 0: Interaction and domain spike

Build a throwaway but runnable vertical scenario with a tiny fixed campaign:

1. Browse two NPCs, a location, and an open thread.
2. Capture several quick session notes without changing application mode.
3. Search across the fixed data with typed results.
4. Request one clearly labelled AI proposal.
5. Review proposed post-session changes.
6. Approve changes and see them in the next prep view.

The goal is to validate navigation, terminology, authority indicators, and the
full capture-to-canon loop before stabilizing schemas.

### Phase 1: Factual campaign workspace

Deliver value without requiring AI:

- Local library, world, campaign, and session creation.
- Multiple campaigns per world with explicit scopes.
- CRUD for the first core entity types.
- Canon, secret, draft, unknown, and superseded states.
- Manual cross-links and source/provenance fields.
- Fast typed fuzzy and full-text search.
- Portable backup/export and restore/import.
- One stable workspace with manual note capture and entity-level drafts.
- Explicitly separated entity-type sections and type-aware create/edit views.
- Configurable panes with persisted personal layout preferences.
- Start/end session lifecycle with a durable multiline transcript.
- `@` entity references with completion and durable record links.
- Local `#` dice and calculation expressions recorded with their results.
- Session `$` commands for quick, context-filled draft entity creation.
- Contextual `#random` generators and `#location` session context commands.

Likely first entity types: NPC, location, faction, item, thread, session, event,
and note. Additional types should be added when the owner's campaign needs them.

### Phase 2: Session reconciliation and campaign memory

- Convert live notes into review items.
- Manually create and approve factual diffs.
- Track event history and superseded state.
- Track party and individual-character knowledge.
- Show “what changed” by session.
- Generate next-session review surfaces from open threads and recent events.
- Reconcile transcript references, events, and roll results into reviewable
  factual changes without rewriting the original transcript.
- Review and approve entities created by `$` commands or contextual generators.

The workflow should be solid manually before AI automates extraction.

### Phase 3: Guardrailed AI assistance

- Add provider configuration and local prompt/context inspection.
- Add contextual actions for NPCs, locations, threads, and sessions.
- Extract proposed events and changes from live notes.
- Require explicit review for every factual mutation.
- Display answer class, retrieved context, and citations.
- Retain, pin, reject, or promote proposals through explicit actions.
- Add continuity and consequence analysis grounded in approved facts.

### Phase 4: Rules and open knowledge

- Ingest an open/licensed SRD corpus.
- Add source, edition, page/section, and entity metadata.
- Provide exact/full-text rules retrieval with citations.
- Distinguish rules, interpretations, and house-rule overrides.
- Enable campaign-specific ruleset and source selection.
- Add structured records for common mechanical concepts where useful.

This validates the sourcebook architecture without making commercial PDF parsing
a prerequisite for the core campaign tool.

### Phase 5: Personal source ingestion

- Import owner-supplied PDFs or structured exports locally.
- Preserve pages, headings, tables, stat blocks, and source identity.
- Provide import diagnostics and correction tools.
- Add hybrid structured, FTS, and semantic retrieval.
- Handle conflicting editions and campaign source enablement.
- Support published-adventure baselines with explicit campaign overrides.

This is a substantial subsystem and should follow proven demand from regular use.

### Phase 6: Deeper live-session support

Add only the capabilities that prove useful at the table:

- Scene state, participants, clocks, conditions, and initiative as needed.
- Pinned briefs and rapid contextual overlays.
- Encounter and creature assistance grounded in enabled sources.
- Optional audio transcription into unapproved session notes.
- Player-safe views or exports that respect knowledge boundaries.

### Phase 7: Additional interfaces

If a web interface becomes desirable:

- Expose existing application commands and queries through a local API.
- Reuse the same domain validation, authority model, and search service.
- Keep the TUI and web clients behaviorally consistent where appropriate.
- Introduce authentication or synchronization only if the actual deployment
  requires them.

No earlier phase should depend on this phase.

## Initial end-to-end scenario

A useful acceptance narrative for early development is:

1. The owner opens `The Ashen Crown`, one of two campaigns in `The Ashen
   Realms`.
2. The workspace shows established facts, draft material, and unresolved threads
   without mixing their authority states.
3. During play, the owner searches `silver`; the overlay visibly searches the
   current campaign and labels NPC, item, session, rule, and proposal results.
4. The owner records “Lena gave Osric the silver key.”
5. An AI action offers several possible consequences, all marked as proposals.
6. Session reconciliation extracts two confident changes and one ambiguity from
   the note.
7. The owner edits and approves the factual changes.
8. Another campaign in the same world remains unchanged.
9. The next prep session retrieves the approved possession and knowledge facts,
   cites Session 17, and keeps discarded AI ideas out of the factual answer.
10. The owner exports the campaign and can inspect its content without running
    Dungeon or contacting an AI provider.

## Open design questions

- How granular should assertions be before data entry becomes burdensome?
- Should world canon be forked at campaign creation or resolved as a live
  baseline plus campaign overlay?
- Which information should be stored as structured fields versus prose?
- How should deliberately conflicting accounts or unreliable narrators be
  represented?
- What is the minimum useful model for individual and group knowledge?
- Which live-session mechanics actually help rather than distract the DM?
- Should AI transcripts be retained by default, summarized, or treated as
  disposable after proposals are resolved?
- Which import/export representation is easiest to inspect and version by hand?
- How should aliases, renamed entities, and duplicate detection work in search?

These questions should be answered through use of the personal tool and concrete
campaign data rather than through speculative generalization.
