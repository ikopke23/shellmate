# Shellmate — Design Document

## Overview

Shellmate is a terminal-based multiplayer chess client written in Go. It targets a small,
invite-only group of players. Games are played in real-time against other connected users,
with spectator support, a persistent Elo leaderboard, and full game history with in-app replay.

## Features

### Gameplay
- Real-time multiplayer chess via WebSocket
- Move list displayed alongside the board (two-column: move number, white move, black move)
- Standard SAN notation (e.g. Rxf3, Qg5) — no Unicode piece symbols
- Current move highlighted with a lipgloss border, list auto-scrolls to track it
- Spectators can watch any active game
- Undo offer system — a player may request to retract their last move; opponent must accept

### User Profile
- Win/loss history viewable in-app
- In-app game replay — step through any past game move by move
- Elo rating displayed on leaderboard and profile

### Leaderboard
- Global leaderboard showing all registered players and their Elo
- Elo system:
  - Starting rating: 1500
  - K-factor: 20 (standard), 40 provisional (first 15 games)
  - Max change per game: +/- 50
  - Elo floor: 800
  - Expected score formula handles upsets/favorites automatically

## Architecture

### Components
- `shellmate-server` — single Go binary (HTTP + WebSocket + Postgres)
- `shellmate` — TUI client binary

### Connection & Auth
1. Client connects to server via WebSocket
2. On first connect: client sends fixed invite code + chosen username
3. Server validates invite code (configured via env var/config), persists username to Postgres
4. Subsequent connections: username looked up by provided username
5. No passwords, no sessions

### WebSocket Message Types
- `join_lobby` / `lobby_state` — player list, active games
- `create_game` / `join_game` / `spectate_game`
- `move` — transmit move in SAN/UCI format
- `undo_request` / `undo_response` — offer/accept/reject undo
- `game_over` — result + Elo delta for both players

## TUI Stack

| Library | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | Elm-style Model/Update/View framework |
| `github.com/charmbracelet/lipgloss` | Board colors, borders, spacing |
| `github.com/notnil/chess` | Game logic, move validation, PGN parsing |

### Layout
```
┌─────────────────┬──────────────┐
│                 │  Move List   │
│   Chess Board   │  1. e4  e5   │
│   (8x8 grid)    │  2. Nf3 Nc6  │
│                 │  ...         │
├─────────────────┴──────────────┤
│  Status bar — turn, Elo, clock │
└────────────────────────────────┘
```

## Database

```sql
CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username     text UNIQUE NOT NULL,
    elo          int NOT NULL DEFAULT 1500,
    games_played int NOT NULL DEFAULT 0,
    created_at   timestamptz DEFAULT now()
);

CREATE TABLE games (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    white            text REFERENCES users(username),
    black            text REFERENCES users(username),
    result           text,  -- "1-0", "0-1", "1/2-1/2", "*"
    white_elo_before int,
    black_elo_before int,
    white_elo_after  int,
    black_elo_after  int,
    played_at        timestamptz DEFAULT now(),
    pgn              text
);

CREATE INDEX idx_games_white ON games(white);
CREATE INDEX idx_games_black ON games(black);
```

## Out of Scope (for now)
- Time controls / clocks
- AI opponent
- Chat
- Account deletion / username changes
