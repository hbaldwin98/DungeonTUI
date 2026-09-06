# Dungeon

Dungeon is a personal, terminal-native campaign workstation for a human Dungeon
Master. Its factual campaign wiki remains strictly separate from AI-generated
answers and proposals.

The first implementation includes a persistent local workspace with
demonstration data for a first run. Draft entities are saved as portable JSON
in the platform user configuration directory after editing.

## Run

Dungeon requires Go 1.25 or newer.

```sh
go mod tidy
go run ./cmd/dungeon
```

## Current controls

| Key | Action |
|---|---|
| `/` | Open typed fuzzy search |
| `j` / `k` | Navigate records |
| `n` | Create a new draft entity |
| `e` | Edit the selected entity |
| `t` | Cycle the visible entity type |
| `s` | Start a session |
| `Tab` / `Shift+Tab` | Move between editor fields |
| `Ctrl+S` | Save the editor, or change search scope while searching |
| `Enter` / `Ctrl+Enter` | Capture a transcript entry during a session |
| `Shift+Enter` | Insert a newline in the transcript editor |
| `Ctrl+E` | End the active session |
| `Ctrl+P` | Cycle visible session panes |
| `Ctrl+A` | Include or exclude AI proposals from search |
| `Enter` | Open a search result |
| `Esc` | Close search |
| `q` | Quit |

Mouse support is enabled from day one. Left-click records and visible search
results to open them, and use the mouse wheel to navigate either the workspace
or search results.

The search overlay defaults to the current campaign and factual records only.
Its header always displays the active scope and whether AI proposals are
included. Results carry entity-type and authority labels.

The workspace stores the active campaign and records through a UI-independent
JSON storage boundary. The type filter keeps NPCs, locations, items, sessions,
and other entity kinds visibly separate while preserving one shared domain model
for future clients.

During a session, typing `@` opens entity autosuggestions. `Tab` inserts the
highlighted entity and opens its full details in the review pane above the
transcript. `Ctrl+Enter` captures the entry. `$npc Name: description` creates a
draft entity from the active session, while `#random item` creates a local draft
generator result and `#location ...` records the current location context.
The session log scrolls with the mouse wheel; clicking the context pane selects
an entity for review. Drag vertical gutters to reallocate left/right panes and
horizontal gutters to reallocate upper/transcript panes. Split positions are
currently session-local and will become persisted workspace preferences.

See [the decision record](docs/decisions.md) and [task snapshot](docs/tasks.md)
for the current implementation boundaries and queue.

## Architecture

```text
cmd/dungeon       executable entry point
internal/domain   UI-independent campaign concepts and invariants
internal/search   typed, scoped search service
internal/tui      Bubble Tea presentation and interaction
```

See [the design document](docs/design.md) for the full product direction and
phased plan.
