package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// Game tracks an active chess game in memory.
type Game struct {
	id          string
	white       *Client
	black       *Client
	spectators  []*Client
	chess       *chess.Game
	mu          sync.Mutex
	pendingUndo string // username who requested undo, or ""
}

// NewGame creates a new game with the given white and black players.
func NewGame(id string, white, black *Client) *Game {
	return &Game{
		id:    id,
		white: white,
		black: black,
		chess: chess.NewGame(),
	}
}

// AddSpectator adds a spectator to the game.
func (g *Game) AddSpectator(c *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.spectators = append(g.spectators, c)
}

// RemoveSpectator removes a spectator.
func (g *Game) RemoveSpectator(c *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, s := range g.spectators {
		if s == c {
			g.spectators = append(g.spectators[:i], g.spectators[i+1:]...)
			return
		}
	}
}

// ApplyMove validates and applies a move in SAN notation.
// Returns an error if the move is invalid or it's not the player's turn.
func (g *Game) ApplyMove(c *Client, san string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.chess.Outcome() != chess.NoOutcome {
		return errors.New("game is already over")
	}
	turn := g.chess.Position().Turn()
	if turn == chess.White && c != g.white {
		return errors.New("it is not your turn")
	}
	if turn == chess.Black && c != g.black {
		return errors.New("it is not your turn")
	}
	if err := g.chess.MoveStr(san); err != nil {
		return fmt.Errorf("invalid move: %w", err)
	}
	g.pendingUndo = ""
	return nil
}

// RequestUndo records that c wants to undo. Returns error if undo is not allowed.
func (g *Game) RequestUndo(c *Client) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.chess.Moves()) == 0 {
		return errors.New("no moves to undo")
	}
	if g.pendingUndo != "" {
		return errors.New("undo already pending")
	}
	if c != g.white && c != g.black {
		return errors.New("spectators cannot request undo")
	}
	g.pendingUndo = c.username
	return nil
}

// AcceptUndo reverts the last move if the non-requesting player accepts.
func (g *Game) AcceptUndo(c *Client) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pendingUndo == "" {
		return errors.New("no pending undo request")
	}
	if c.username == g.pendingUndo {
		return errors.New("cannot accept your own undo request")
	}
	// Rebuild the game without the last move.
	moves := g.chess.Moves()
	if len(moves) == 0 {
		return errors.New("no moves to undo")
	}
	notation := chess.AlgebraicNotation{}
	newGame := chess.NewGame()
	positions := g.chess.Positions()
	for i, m := range moves[:len(moves)-1] {
		san := notation.Encode(positions[i], m)
		if err := newGame.MoveStr(san); err != nil {
			return fmt.Errorf("failed to rebuild game: %w", err)
		}
	}
	g.chess = newGame
	g.pendingUndo = ""
	return nil
}

// RejectUndo clears the pending undo request.
func (g *Game) RejectUndo(c *Client) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pendingUndo == "" {
		return errors.New("no pending undo request")
	}
	if c.username == g.pendingUndo {
		return errors.New("cannot reject your own undo request")
	}
	g.pendingUndo = ""
	return nil
}

// Outcome returns the current game outcome ("1-0", "0-1", "1/2-1/2", "*").
func (g *Game) Outcome() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return string(g.chess.Outcome())
}

// PGN returns the current PGN string for the game.
func (g *Game) PGN() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.chess.String()
}

// Broadcast sends a message to both players and all spectators.
func (g *Game) Broadcast(msg []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.broadcastLocked(msg)
}

func (g *Game) broadcastLocked(msg []byte) {
	if g.white != nil {
		g.white.Send(msg)
	}
	if g.black != nil {
		g.black.Send(msg)
	}
	for _, s := range g.spectators {
		s.Send(msg)
	}
}

// BroadcastMove sends the current board state to all participants after a move.
func (g *Game) BroadcastMove(san string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	notation := chess.AlgebraicNotation{}
	positions := g.chess.Positions()
	moves := g.chess.Moves()
	var moveList []string
	for i, m := range moves {
		moveList = append(moveList, notation.Encode(positions[i], m))
	}
	type moveMsg struct {
		GameID string   `json:"game_id"`
		SAN    string   `json:"san"`
		Moves  []string `json:"moves"`
	}
	data, err := shared.Encode(shared.MsgMove, moveMsg{GameID: g.id, SAN: san, Moves: moveList})
	if err != nil {
		slog.Error("failed to encode move broadcast", "error", err)
		return
	}
	g.broadcastLocked(data)
}

// BroadcastUndoAccepted sends an undo_accepted message with the current move list.
func (g *Game) BroadcastUndoAccepted() {
	g.mu.Lock()
	defer g.mu.Unlock()
	notation := chess.AlgebraicNotation{}
	positions := g.chess.Positions()
	moves := g.chess.Moves()
	var moveList []string
	for i, m := range moves {
		moveList = append(moveList, notation.Encode(positions[i], m))
	}
	data, err := shared.Encode(shared.MsgUndoAccepted, shared.UndoAccepted{GameID: g.id, Moves: moveList})
	if err != nil {
		slog.Error("failed to encode undo_accepted broadcast", "error", err)
		return
	}
	g.broadcastLocked(data)
}

// IsOver returns true if the game has ended (outcome != "*").
func (g *Game) IsOver() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.chess.Outcome() != chess.NoOutcome
}

// handleGameOver processes Elo changes and persists the game result.
func (g *Game) handleGameOver(ctx context.Context, hub *Hub) {
	g.mu.Lock()
	outcome := g.chess.Outcome()
	pgn := g.chess.String()
	g.mu.Unlock()
	var result float64
	switch outcome {
	case chess.WhiteWon:
		result = 1.0
	case chess.BlackWon:
		result = 0.0
	default:
		result = 0.5
	}
	whiteUser, err := hub.db.GetUser(ctx, g.white.username)
	if err != nil {
		slog.Error("failed to get white user", "error", err)
		return
	}
	blackUser, err := hub.db.GetUser(ctx, g.black.username)
	if err != nil {
		slog.Error("failed to get black user", "error", err)
		return
	}
	if whiteUser == nil || blackUser == nil {
		slog.Error("player not found in DB during game over")
		return
	}
	newWhite, newBlack := Calculate(whiteUser.Elo, blackUser.Elo, whiteUser.GamesPlayed, blackUser.GamesPlayed, result)
	rec := GameRecord{
		White:          g.white.username,
		Black:          g.black.username,
		Result:         string(outcome),
		WhiteEloBefore: whiteUser.Elo,
		BlackEloBefore: blackUser.Elo,
		WhiteEloAfter:  newWhite,
		BlackEloAfter:  newBlack,
		PGN:            pgn,
	}
	if err := hub.db.SaveGameAndUpdateElo(ctx, rec, newWhite, newBlack); err != nil {
		slog.Error("failed to save game", "error", err)
		return
	}
	overMsg, err := shared.Encode(shared.MsgGameOver, shared.GameOver{
		GameID:         g.id,
		Result:         string(outcome),
		WhiteEloBefore: whiteUser.Elo,
		BlackEloBefore: blackUser.Elo,
		WhiteEloAfter:  newWhite,
		BlackEloAfter:  newBlack,
		WhiteUsername:  g.white.username,
		BlackUsername:  g.black.username,
	})
	if err != nil {
		slog.Error("failed to encode game_over", "error", err)
		return
	}
	g.Broadcast(overMsg)
}

