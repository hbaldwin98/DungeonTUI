# Dungeon

Dungeon is a personal, terminal-native campaign workstation for a human Dungeon
Master. Its factual campaign wiki remains strictly separate from AI-generated
answers and proposals.

The first implementation includes a persistent local workspace with
demonstration data for a first run. Draft entities are saved as portable JSON
in the platform user configuration directory after editing.

## Run

Dungeon requires Go 1.25 or newer. On launch you choose a world and campaign
(Obsidian-vault style); `b` returns to that library picker to switch context.

```sh
go mod tidy
go run ./cmd/dungeon
```

## Validate / screencap

Agents and local checks can drive the TUI without a real tty:

```sh
go test ./internal/tui -run Harness
go run ./cmd/dungeon-harness -scenario all -out testdata/harness
```

Scenarios cover browser/search/session flows, mouse-wheel scrolling, and gutter
resizing. Each cap writes `.screen.txt` (readable), `.ansi.txt`, and `.meta.txt`.

## Current controls

| Key | Action |
|---|---|
| `/` | Open typed fuzzy search |
| `j` / `k` | Navigate the focused pane (campaign tree, list, or records) |
| `←` / `→` / `Tab` / `Shift+Tab` / `t` | Cycle focus across nav · list · detail |
| `n` | Create a new draft entity (markdown) |
| `e` | Edit the selected entity or prep notes |
| `d` | Delete selected entity or session (`y` confirm / `n` cancel) |
| `x` | Supersede selected entity (`y` confirm / `n` cancel) |
| `Enter` | Open the selected session, prep notes, or entity |
| `b` | Back to world/campaign library picker |
| `p` | Draft planned session notes (prep markdown, not live) |
| `s` | Start a live session (uses selected/latest planned notes as context) |
| `Tab` / `Shift+Tab` | In editor: move fields; in session: suggest/cycle focus |
| `Ctrl+S` | Save markdown editor / planned notes, or change search scope while searching |
| `Ctrl+T` | Cycle entity type while editing markdown |
| `Enter` / `Ctrl+Enter` | Capture a transcript entry during a session |
| `Shift+Enter` | Insert a newline in the transcript editor |
| `Ctrl+E` | End the active session |
| `Ctrl+P` | Cycle session campaign/context panes |
| `Ctrl+A` | Include or exclude AI proposals from search |
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
for future clients. Inside a campaign, the browser is an IDE-style tree:
**nav | list | detail**. The left branch lists Sessions, Prep, and typed wiki
sections; the center list and right detail follow the selected branch. Older
dense typed-pane and classic `list|detail` preferences upgrade automatically.

During a session, click any pane (campaign, context, transcript, or input) to
focus it — the focused pane gets an accent border. `Tab` / `Shift+Tab` cycle
focus; with input focused and suggestions visible, `Tab` inserts the highlighted
choice. Empty input always offers `@`, `$`, and `#` starters; typing one of those
filters entity references, draft create templates, or roll/random/location
commands. `j`/`k`/`Enter` navigate campaign and context lists when those panes
are focused. Click campaign sections or context entities to open review; use the
`[pin]` / `[clear]` controls on the selected entity. `Ctrl+Enter` captures the
entry. `$npc Name: description` creates a draft entity from the active session,
while `#random item` creates a local draft generator result and `#location ...`
records the current location context. Inline `#d20+5` / `#damage 2d6+3`
expressions are evaluated locally and stored with the transcript entry
(expression, faces, and total) without becoming canon. `#location Name` sets the
session's current location (linked when a matching location entity exists); the
CURRENT SCENE pane shows that location, present NPCs/characters, and open
threads from the live campaign record. The session log scrolls with the mouse
wheel. Drag vertical gutters to reallocate left/right panes and horizontal
gutters to reallocate upper/transcript panes. Split positions and the Ctrl+P
session pane layout are stored in `preferences.json` beside the campaign
workspace file as named pane split trees (ratios + campaign/context visibility).

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
