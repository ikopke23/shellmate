package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// ErrTimeExpired is returned by ApplyMove when the moving player's clock runs out.
var ErrTimeExpired = errors.New("time expired")

// Game tracks an active chess game in memory.
type Game struct {
	id             string
	white          *Client
	black          *Client
	spectators     []*Client
	chess          *chess.Game
	mu             sync.Mutex
	pendingUndo    string // username who requested undo, or ""
	timed          bool
	whiteRemaining time.Duration
	blackRemaining time.Duration
	turnStartedAt  time.Time
	increment      time.Duration
}

// NewGame creates a new game. initialSec==0 means untimed.
func NewGame(id string, white, black *Client, initialSec, incrementSec int) *Game {
	timed := initialSec > 0
	initial := time.Duration(initialSec) * time.Second
	return &Game{
		id:             id,
		white:          white,
		black:          black,
		chess:          chess.NewGame(),
		timed:          timed,
		whiteRemaining: initial,
		blackRemaining: initial,
		increment:      time.Duration(incrementSec) * time.Second,
	}
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
// Returns ErrTimeExpired if the moving player's clock has run out (timed games only).
// Clock time is only deducted after the move is confirmed legal.
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
	// Check time expiry before applying move (but don't mutate clock yet).
	var elapsed time.Duration
	if g.timed {
		elapsed = time.Since(g.turnStartedAt)
		if turn == chess.White && elapsed >= g.whiteRemaining {
			g.chess.Resign(chess.White)
			return ErrTimeExpired
		}
		if turn == chess.Black && elapsed >= g.blackRemaining {
			g.chess.Resign(chess.Black)
			return ErrTimeExpired
		}
	}
	// Validate move legality.
	if err := g.chess.MoveStr(san); err != nil {
		return fmt.Errorf("invalid move: %w", err)
	}
	// Move is legal: now deduct time and add increment.
	if g.timed {
		if turn == chess.White {
			g.whiteRemaining -= elapsed
			g.whiteRemaining += g.increment
		} else {
			g.blackRemaining -= elapsed
			g.blackRemaining += g.increment
		}
		g.turnStartedAt = time.Now()
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
func (g *Game) Broadcast(msg tea.Msg) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.broadcastLocked(msg)
}

func (g *Game) broadcastLocked(msg tea.Msg) {
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
func (g *Game) BroadcastMove() {
	g.mu.Lock()
	defer g.mu.Unlock()
	notation := chess.AlgebraicNotation{}
	positions := g.chess.Positions()
	moves := g.chess.Moves()
	var moveList []string
	for i, m := range moves {
		moveList = append(moveList, notation.Encode(positions[i], m))
	}
	g.broadcastLocked(shared.MoveMsg{
		GameID: g.id,
		Moves:  moveList,
		Clock: shared.ClockState{
			WhiteMs: int(g.whiteRemaining.Milliseconds()),
			BlackMs: int(g.blackRemaining.Milliseconds()),
		},
	})
}

// CurrentClockState returns the current clock state (for spectate catch-up).
func (g *Game) CurrentClockState() shared.ClockState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return shared.ClockState{
		WhiteMs: int(g.whiteRemaining.Milliseconds()),
		BlackMs: int(g.blackRemaining.Milliseconds()),
	}
}

// Turn returns whose turn it currently is. Safe for concurrent use.
func (g *Game) Turn() chess.Color {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.chess.Position().Turn()
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
	g.broadcastLocked(shared.UndoAccepted{GameID: g.id, Moves: moveList})
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
	g.Broadcast(shared.GameOver{
		GameID:         g.id,
		Result:         string(outcome),
		WhiteEloBefore: whiteUser.Elo,
		BlackEloBefore: blackUser.Elo,
		WhiteEloAfter:  newWhite,
		BlackEloAfter:  newBlack,
		WhiteUsername:  g.white.username,
		BlackUsername:  g.black.username,
	})
}
