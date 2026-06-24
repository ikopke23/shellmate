package screens

import (
	"testing"

	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// newPremoveTestGame returns a white GameModel with moves applied; caller controls turn.
func newPremoveTestGame(moves []string) *GameModel {
	tc := shared.TimeControl{}
	m := NewGameModel("id", "white", "black", chess.White, "white", tc)
	if len(moves) > 0 {
		m.SetMovesWithClock(moves, shared.ClockState{})
	}
	return m
}

// --- buildPremoveSimGame ---

func TestBuildPremoveSimGame_EmptyPremoves_PlayerTurn(t *testing.T) {
	g := chess.NewGame()
	sim := buildPremoveSimGame(g, chess.White, nil)
	if sim == nil {
		t.Fatal("expected non-nil sim")
	}
	if sim.Position().Turn() != chess.White {
		t.Errorf("expected white to move in sim, got %v", sim.Position().Turn())
	}
}

func TestBuildPremoveSimGame_BlackStarting_PlayerTurn(t *testing.T) {
	g := chess.NewGame()
	_ = g.MoveStr("e4") // now black's turn
	sim := buildPremoveSimGame(g, chess.Black, nil)
	if sim == nil {
		t.Fatal("expected non-nil sim")
	}
	if sim.Position().Turn() != chess.Black {
		t.Errorf("expected black to move in sim, got %v", sim.Position().Turn())
	}
}

func TestBuildPremoveSimGame_OnePremove_AppliesMove(t *testing.T) {
	g := chess.NewGame()
	_ = g.MoveStr("e4")                                        // black's turn now; white premoved e4
	sim := buildPremoveSimGame(g, chess.White, []string{"d4"}) // white premoves d4
	if sim == nil {
		t.Fatal("expected non-nil sim after one premove")
	}
	// d4 pawn should be at d4 in the simulated position
	piece := sim.Position().Board().Piece(chess.D4)
	if piece == chess.NoPiece {
		t.Errorf("expected pawn at d4 in simulated position after premove d4")
	}
	if sim.Position().Turn() != chess.White {
		t.Errorf("expected white to move after resetting turn, got %v", sim.Position().Turn())
	}
}

func TestBuildPremoveSimGame_ChainedPremoves(t *testing.T) {
	g := chess.NewGame()
	_ = g.MoveStr("e4") // black's turn; white premoved e4
	// White premoves: e4 was already applied above. We test chaining d4 + Nf3.
	// Use starting position so white can premove e4 then Nf3.
	g2 := chess.NewGame()
	_ = g2.MoveStr("e4") // black's turn
	sim := buildPremoveSimGame(g2, chess.White, []string{"d4", "Nf3"})
	if sim == nil {
		t.Fatal("expected non-nil sim for chained premoves")
	}
	if sim.Position().Turn() != chess.White {
		t.Errorf("expected white to move in sim, got %v", sim.Position().Turn())
	}
	// Both d4 pawn and Nf3 knight should be in place
	if sim.Position().Board().Piece(chess.D4) == chess.NoPiece {
		t.Error("expected pawn at d4 in chained sim")
	}
	if sim.Position().Board().Piece(chess.F3) == chess.NoPiece {
		t.Error("expected knight at f3 in chained sim")
	}
}

func TestBuildPremoveSimGame_InvalidPremove_ReturnsNil(t *testing.T) {
	g := chess.NewGame()
	_ = g.MoveStr("e4")
	// "Nf6" is g8→f6, a black knight move — white has no piece at g8.
	sim := buildPremoveSimGame(g, chess.White, []string{"Nf6"})
	if sim != nil {
		t.Error("expected nil sim for invalid premove chain")
	}
}

// --- tryAddPremove ---

func TestGameModel_TryAddPremove_Valid(t *testing.T) {
	// White's turn: add e4 as premove from starting position.
	m := newPremoveTestGame(nil)
	ok := m.tryAddPremove("e4")
	if !ok {
		t.Fatal("expected tryAddPremove to succeed for valid move")
	}
	if len(m.premoves) != 1 || m.premoves[0] != "e4" {
		t.Errorf("expected premoves=[e4], got %v", m.premoves)
	}
	if len(m.premovePairs) != 1 {
		t.Errorf("expected premovePairs len 1, got %d", len(m.premovePairs))
	}
}

func TestGameModel_TryAddPremove_Invalid(t *testing.T) {
	m := newPremoveTestGame(nil)
	ok := m.tryAddPremove("zzz")
	if ok {
		t.Fatal("expected tryAddPremove to fail for invalid SAN")
	}
	if len(m.premoves) != 0 {
		t.Errorf("expected premoves unchanged, got %v", m.premoves)
	}
}

func TestGameModel_TryAddPremove_Cap(t *testing.T) {
	m := newPremoveTestGame(nil)
	// Directly set 10 premoves to hit the cap without needing legal moves.
	m.premoves = make([]string, 10)
	m.premovePairs = make([][2]chess.Square, 10)
	ok := m.tryAddPremove("e4")
	if ok {
		t.Fatal("expected tryAddPremove to fail when queue is at cap")
	}
	if len(m.premoves) != 10 {
		t.Errorf("expected premoves len unchanged at 10, got %d", len(m.premoves))
	}
}

// --- TryExecutePremove ---

func TestGameModel_TryExecutePremove_EmptyQueue(t *testing.T) {
	m := newPremoveTestGame(nil)
	cmd := m.TryExecutePremove()
	if cmd != nil {
		t.Error("expected nil cmd for empty premove queue")
	}
}

func TestGameModel_TryExecutePremove_Legal(t *testing.T) {
	// White's turn: directly set a valid premove and execute it.
	m := newPremoveTestGame(nil) // white to move at starting position
	g := m.chess
	pos := g.Position()
	notation := chess.AlgebraicNotation{}
	// Find the chess.Move for e4 to get S1/S2.
	for _, mv := range g.ValidMoves() {
		if notation.Encode(pos, mv) == "e4" {
			m.premoves = []string{"e4"}
			m.premovePairs = [][2]chess.Square{{mv.S1(), mv.S2()}}
			break
		}
	}
	if len(m.premoves) == 0 {
		t.Fatal("could not set up premove e4")
	}
	cmd := m.TryExecutePremove()
	if cmd == nil {
		t.Fatal("expected non-nil cmd for legal premove")
	}
	if len(m.premoves) != 0 {
		t.Errorf("expected premove dequeued, got %v", m.premoves)
	}
}

func TestGameModel_TryExecutePremove_Illegal(t *testing.T) {
	// White's turn: queue an illegal premove (e5 is not valid for white at start).
	m := newPremoveTestGame(nil)
	m.premoves = []string{"e5"}
	m.premovePairs = [][2]chess.Square{{chess.E5, chess.E5}}
	cmd := m.TryExecutePremove()
	if cmd != nil {
		t.Error("expected nil cmd for illegal premove")
	}
	if len(m.premoves) != 0 {
		t.Errorf("expected premove queue cleared, got %v", m.premoves)
	}
}

func TestGameModel_TryExecutePremove_OpponentTurn(t *testing.T) {
	// After e4 it's black's turn; white's premoves should not execute.
	m := newPremoveTestGame([]string{"e4"}) // black's turn now
	m.premoves = []string{"d4"}
	m.premovePairs = [][2]chess.Square{{chess.D2, chess.D4}}
	cmd := m.TryExecutePremove()
	if cmd != nil {
		t.Error("expected nil cmd: premove should not execute on opponent's turn")
	}
	if len(m.premoves) != 1 {
		t.Errorf("expected premoves unchanged, got %v", m.premoves)
	}
}

func TestGameModel_ClearPremoves_WipesAll(t *testing.T) {
	m := newPremoveTestGame(nil)
	m.premoves = []string{"e4"}
	m.premovePairs = [][2]chess.Square{{chess.E2, chess.E4}}
	m.premoveList.SetMoves(m.premoves)
	m.board.SetPremoves(m.premovePairs)
	m.clearPremoves()
	if m.premoves != nil {
		t.Errorf("expected premoves nil, got %v", m.premoves)
	}
	if m.premovePairs != nil {
		t.Errorf("expected premovePairs nil, got %v", m.premovePairs)
	}
	if m.premoveList.View() != "" {
		t.Errorf("expected premoveList empty after clear, got %q", m.premoveList.View())
	}
}
