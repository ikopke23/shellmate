package shared

import "time"

// PlayerInfo describes a player in the lobby.
type PlayerInfo struct {
	Username string `json:"username"`
	Elo      int    `json:"elo"`
	Online   bool   `json:"online"`
}

// GameInfo describes a game visible in the lobby.
type GameInfo struct {
	ID         string `json:"id"`
	White      string `json:"white"`
	Black      string `json:"black"`
	Spectators int    `json:"spectators"`
	Moves      int    `json:"moves"`
}

// LobbyState is broadcast to clients to describe current lobby membership and active games.
type LobbyState struct {
	Players []PlayerInfo `json:"players"`
	Games   []GameInfo   `json:"games"`
}

// TimeControl describes the time limit and increment for a game.
// InitialSeconds == 0 means untimed.
type TimeControl struct {
	InitialSeconds   int `json:"initial_seconds"`
	IncrementSeconds int `json:"increment_seconds"`
}

// ClockState carries both players' remaining time in milliseconds.
type ClockState struct {
	WhiteMs int `json:"white_ms"`
	BlackMs int `json:"black_ms"`
}

// MoveMsg is broadcast by the server after each legal move.
// SAN is intentionally omitted — Moves supersedes it.
type MoveMsg struct {
	GameID string     `json:"game_id"`
	Moves  []string   `json:"moves"`
	Clock  ClockState `json:"clock"` // zero value used for untimed games
}

// CreateGame is sent by a client requesting a new game with a chosen time control.
type CreateGame struct {
	TimeControl TimeControl `json:"time_control"`
}

// JoinGame is sent by a client to join an existing game as a player.
type JoinGame struct {
	GameID string `json:"game_id"`
}

// SpectateGame is sent by a client to observe a game without playing.
type SpectateGame struct {
	GameID string `json:"game_id"`
}

// Move is sent by a client to make a move in an active game.
// SAN is Standard Algebraic Notation (e.g. "e4", "Nf3", "O-O").
type Move struct {
	GameID string `json:"game_id"`
	SAN    string `json:"san"` // Standard Algebraic Notation
}

// UndoRequest is sent by a client to request that the last move be undone.
type UndoRequest struct {
	GameID string `json:"game_id"`
}

// UndoResponse is sent by a client to accept or reject an undo request.
type UndoResponse struct {
	GameID string `json:"game_id"`
	Accept bool   `json:"accept"`
}

// UndoAccepted is broadcast when an undo is accepted and the last move reverted.
type UndoAccepted struct {
	GameID string   `json:"game_id"`
	Moves  []string `json:"moves"` // full move list in SAN after undo
}

// ErrorMsg is sent by the server to communicate a protocol or application error.
type ErrorMsg struct {
	Message string `json:"message"`
}

// GameOver is broadcast when a game ends, including Elo changes.
type GameOver struct {
	GameID         string `json:"game_id"`
	Result         string `json:"result"`
	WhiteEloBefore int    `json:"white_elo_before"`
	BlackEloBefore int    `json:"black_elo_before"`
	WhiteEloAfter  int    `json:"white_elo_after"`
	BlackEloAfter  int    `json:"black_elo_after"`
	WhiteUsername  string `json:"white_username"`
	BlackUsername  string `json:"black_username"`
}

// HistoryRecord describes a completed game for the history/replay screens.
type HistoryRecord struct {
	ID             string    `json:"id"`
	White          string    `json:"white"`
	Black          string    `json:"black"`
	Result         string    `json:"result"`
	WhiteEloBefore int       `json:"white_elo_before"`
	BlackEloBefore int       `json:"black_elo_before"`
	WhiteEloAfter  int       `json:"white_elo_after"`
	BlackEloAfter  int       `json:"black_elo_after"`
	PGN            string    `json:"pgn,omitempty"`
	PlayedAt       time.Time `json:"played_at"`
	Imported       bool      `json:"imported"`
}

// GameStart is sent by the server when a game begins.
type GameStart struct {
	GameID      string      `json:"game_id"`
	White       string      `json:"white"`
	Black       string      `json:"black"`
	TimeControl TimeControl `json:"time_control"`
}

// Resign is sent by a player to forfeit the game.
type Resign struct {
	GameID string `json:"game_id"`
}

// PuzzleRecord describes a puzzle returned by GET /puzzle.
type PuzzleRecord struct {
	ID               string   `json:"id"`
	FEN              string   `json:"fen"`
	Moves            string   `json:"moves"`
	ContextMoves     string   `json:"context_moves"`
	Rating           int      `json:"rating"`
	Themes           []string `json:"themes"`
	GameURL          string   `json:"game_url"`
	UserPuzzleRating int      `json:"user_puzzle_rating"`
}

// PuzzleAttemptResult is returned by POST /puzzle/attempt.
type PuzzleAttemptResult struct {
	PuzzleRating int `json:"puzzle_rating"`
}
