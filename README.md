# Shellmate

Multiplayer chess you play over SSH. Connect with your terminal, get a full TUI
board, and play real-time games against other people — no client to install,
no passwords to remember.

```
ssh -p 2222 your.server
```

The server hosts the entire UI. Your SSH key *is* your account.

## Features

- **Real-time multiplayer** with chess clocks
- **Spectator mode** — watch any active game
- **Game history & in-app replay** — step through past games move by move
- **Elo leaderboard** — provisional K-factor for new players, rating floor, capped per-game swing
- **Puzzle mode** — rating-matched Lichess puzzles (±200 Elo band), computer plays the replies, board auto-flips for black-to-move, press `s` to reveal the solution after a miss
- **PGN import** — bring in external games and replay them
- **Graphical pieces** via the kitty graphics protocol (with ASCII fallback)

## How it works

Shellmate is a single Go binary, `shellmate-server`. It runs a
[Charm Wish](https://github.com/charmbracelet/wish) SSH server that serves a
[Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI to each connecting
client. State lives in PostgreSQL.

**Auth is keyless and passwordless.** On your first connection the server looks
up your SSH public-key fingerprint, doesn't find it, and drops you into a
registration screen: enter the invite code and pick a username. That binds your
key to the account. Every connection after that logs you straight in by
fingerprint.

```
internal/
  client/          # TUI: model, screens, board + move-list rendering
    render/         # board/piece rendering (kitty graphics, assets)
    screens/        # lobby, game, puzzle, replay, history, leaderboard, import
  server/          # hub, game logic, Elo, Lichess puzzle import, Postgres
  shared/          # message/protocol types between hub and client
cmd/shellmate-server/   # entry point + migration loader
migrations/        # numbered SQL migrations, applied on startup
```

## Running

Requires Go 1.26+ and a PostgreSQL database.

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/shellmate"
export INVITE_CODE="some-secret"
export SSH_PORT=":2222"          # optional, defaults to :2222

make build          # → bin/shellmate-server
./bin/shellmate-server
```

Or run it directly:

```bash
go run ./cmd/shellmate-server
```

Migrations in `migrations/` are applied automatically on startup. A host key is
generated at `.ssh/shellmate_host_key` on first run.

Then connect from anywhere:

```bash
ssh -p 2222 localhost
```

### Docker

```bash
docker build -t shellmate .
docker run -p 2222:2222 \
  -e DATABASE_URL="postgres://..." \
  -e INVITE_CODE="some-secret" \
  shellmate
```

### Importing puzzles

Bulk-load puzzles from a [Lichess puzzle CSV](https://database.lichess.org/#puzzles).
This runs the import and exits — the server does not start.

```bash
./bin/shellmate-server --import-puzzles lichess_db_puzzle.csv
```

## Development

```bash
make test        # go test -race ./...
make lint        # golangci-lint
make fmt         # gofumpt -w
make coverage    # coverage report + HTML
```

DB integration tests in `internal/server` are skipped unless you point them at a
test database. They connect to a database named `shellmate_test`, apply all
migrations, and truncate existing data on each run — use a dedicated database,
never production.

```bash
export SHELLMATE_TEST_POSTGRES_HOST=localhost
export SHELLMATE_TEST_POSTGRES_USER=shellmate
export SHELLMATE_TEST_POSTGRES_PASS=shellmate
```

## Stack

| Library | Purpose |
|---|---|
| `charmbracelet/wish` + `charmbracelet/ssh` | SSH server that serves the TUI |
| `charmbracelet/bubbletea` + `lipgloss` | Elm-style TUI framework & styling |
| `notnil/chess` | Move validation, SAN/PGN parsing |
| `jackc/pgx` | PostgreSQL driver |
| `eliukblau/pixterm` | Image → terminal piece rendering |
