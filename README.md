# Dungeon

Dungeon is a personal, terminal-native campaign workstation for a human Dungeon
Master. Its factual campaign wiki remains strictly separate from AI-generated
answers and proposals.

The first implementation includes a persistent local workspace. A first run
opens an empty library picker (`n` creates a world). Draft entities are saved
as portable JSON in the platform user configuration directory after editing.

## Run

Dungeon requires Go 1.25 or newer. On launch you choose a world and campaign
(Obsidian-vault style); `b` returns to that library picker to switch context.
There, `e` renames a world or campaign and `d` then `y` deletes it.

```sh
go mod tidy
go run ./cmd/dungeon
make test
make build
```

## Import a sourcebook or adventure

Choose adventures in the app. Markdown files still work; **5e.tools** is the
structured catalog for **adventures only**. Enter on an adventure **caches** it
as a read-only reference book (no wiki NPCs or rooms) and enables it for the
**current campaign**. The campaign tree **Sources** branch lists those titles
and collapsible chapters; named people and rooms sit inside a chapter. Names
that share the same text collapse to one row (extra labels stay aliases for
`@` and `/`). The detail pane renders the selected document only. Local markdown
**FILES** still become owner wiki records. Monster Manual, Player's Handbook, and similar mechanical books
are not imported. The adapter fetches JSON at import time. Raw JSON is never
stored in this git repository.

```sh
go run ./cmd/dungeon import https://5e.tools/adventure.html#BOOKID,-1
go run ./cmd/dungeon import 5e:BOOKID
go run ./cmd/dungeon import -data /path/to/5etools https://5e.tools/adventure.html#BOOKID,-1
go run ./cmd/dungeon import -kind bestiary ./bestiary.md
go run ./cmd/dungeon import -remove src-5e-bookid

# Headless inspect of the workspace wiki
go run ./cmd/dungeon dump
go run ./cmd/dungeon dump -source src-markdown-title
go run ./cmd/dungeon export -o campaign.json
go run ./cmd/dungeon restore campaign.json
go run ./cmd/dungeon restore

```

Inside a campaign or the library picker, `I` opens **Import**: SOURCES |
5E.TOOLS | FILES. Type to filter the live adventure catalog. Enter caches the
selected adventure. `e` enables the source for the campaign, `d` then `y`
removes it. `Esc` returns to the picker or campaign. `dungeon dump` is a
separate inspect command.

`@Mira Holt` and other mentions resolve against campaign + world-shared wiki
first. Enabled adventure caches are **reference** only (never wiki); the
owner's wiki wins on a name collision. `@` peeks stay the small overlay.
Adventure sites from **markdown FILES** are classified from structure (front
matter, numbered rooms) **before** wiki rows are written.

Search is SQLite FTS5 over wiki, prep, sessions, transcripts, recon, and
enabled adventure **reference** caches (D-024, D-032). The workspace JSON is
still the inspectable campaign store; `search.sqlite` is a rebuildable index.
Adventure JSON lives in the local 5e.tools cache. Embeddings stay out.


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
| `e` | Edit the selected entity or prep notes (full-screen markdown; `@` suggests) |
| `I` | Open the Import screen (library sources, 5e.tools adventures, markdown files) |
| `d` | Delete selected entity or session; on Import SOURCES, remove the ingested book (`y` confirm / `n` cancel) |
| `x` | Supersede selected entity (`y` confirm / `n` cancel) |
| `?` | Show context-sensitive command help |
| `f` | Cycle tag filter on the current section list |
| `o` | Cycle list scope filter (campaign → world → library) |
| `b` | Back to world/campaign library picker (`e` rename, `d` then `y` delete) |
| `p` | Draft planned session notes (prep markdown, not live) |
| `s` | Start a live session (uses selected/latest planned notes as context) |
| `m` | File the selected sit (or folder group) into a session folder |
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
**nav | list | detail**. The left branch lists Sessions, Prep, Sources, and typed wiki
sections; the center list and right detail follow the selected branch. Detail,
prep notes, peeks, and link previews render markdown; `e` still edits source.
Sessions nest under collapsible folders (`m` files a sit; unfiled nights bucket by
month). Detail hops (`@` mentions, backlinks, CAST) open a preview buffer
first; Enter or `[open]` jumps, Esc or click outside dismisses. Older
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
internal/app      session/record/recon transactions and persistence rollback
internal/search   typed, scoped search service
internal/storage  JSON workspace, export, and restore
internal/tui      Bubble Tea presentation and interaction
```

Licensed under MIT. See [LICENSE](LICENSE).

See [the design document](docs/design.md) for the full product direction and
phased plan.
