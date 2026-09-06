# Dungeon

Dungeon is a personal, terminal-native campaign workstation for a human Dungeon
Master. Its factual campaign wiki remains strictly separate from AI-generated
answers and proposals.

The first implementation is an interaction spike for the core product loop. It
contains in-memory demonstration data while the domain and navigation model are
validated.

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
| `Ctrl+S` | Change search scope |
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

## Architecture

```text
cmd/dungeon       executable entry point
internal/domain   UI-independent campaign concepts and invariants
internal/search   typed, scoped search service
internal/tui      Bubble Tea presentation and interaction
```

See [the design document](docs/design.md) for the full product direction and
phased plan.
