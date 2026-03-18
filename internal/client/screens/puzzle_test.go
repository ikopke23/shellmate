package screens

import (
	"testing"

	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// setupPuzzleModel builds a PuzzleModel already past the loading state using a real
// chess position and a two-move solution sequence (one opponent move + one user move).
// Position: standard opening after 1.e4 e5 2.Nf3 (ply 3).
// The FEN at that point is the starting point. moves[0] = "e2e4" (auto-played), moves[1] = "d7d5" (user's move).
func setupPuzzleModel(t *testing.T) *PuzzleModel {
	t.Helper()
	record := shared.PuzzleRecord{
		ID:               "test1",
		FEN:              "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		Moves:            "e2e4 d7d5",
		Rating:           1500,
		Themes:           []string{"opening"},
		UserPuzzleRating: 1500,
	}
	m := NewPuzzleModel("localhost:8080", "testuser")
	m.SetPuzzle(record)
	return m
}

func TestPuzzleModelInitAppliesFirstMove(t *testing.T) {
	m := setupPuzzleModel(t)
	// After SetPuzzle, moves[0] (e2e4) should be applied.
	// The board position should not be the starting position.
	pos := m.game.Position()
	// e4 pawn should be at e4 (square index 28), not e2.
	if pos.Board().Piece(chess.E4) == chess.NoPiece {
		t.Error("expected pawn on e4 after moves[0] applied, got empty square")
	}
	if pos.Board().Piece(chess.E2) != chess.NoPiece {
		t.Error("expected e2 to be empty after moves[0] applied")
	}
	// state should be playing
	if m.state != puzzleStatePlaying {
		t.Errorf("state = %v, want puzzleStatePlaying", m.state)
	}
}

func TestPuzzleValidateCorrectMove(t *testing.T) {
	m := setupPuzzleModel(t)
	// solution[1] is "d7d5" which in SAN is "d5"
	ok := m.validateAndApply("d5")
	if !ok {
		t.Error("validateAndApply returned false for correct move d5")
	}
	// After correct move + no further opponent moves, should be success
	if m.state != puzzleStateSuccess {
		t.Errorf("state = %v, want puzzleStateSuccess", m.state)
	}
}

func TestPuzzleValidateWrongMove(t *testing.T) {
	m := setupPuzzleModel(t)
	ok := m.validateAndApply("e5")
	if ok {
		t.Error("validateAndApply returned true for wrong move e5")
	}
	if m.state != puzzleStateFailure {
		t.Errorf("state = %v, want puzzleStateFailure", m.state)
	}
}

func TestPuzzleValidateInvalidSAN(t *testing.T) {
	m := setupPuzzleModel(t)
	ok := m.validateAndApply("zz9")
	if ok {
		t.Error("validateAndApply returned true for invalid SAN")
	}
	if m.state != puzzleStateFailure {
		t.Errorf("state = %v, want puzzleStateFailure", m.state)
	}
}

func TestPuzzleRetryResetsToPlaying(t *testing.T) {
	m := setupPuzzleModel(t)
	m.validateAndApply("e5") // wrong move → failure
	m.retry()
	if m.state != puzzleStatePlaying {
		t.Errorf("after retry state = %v, want puzzleStatePlaying", m.state)
	}
	// moves[0] should be re-applied: e4 pawn should be on e4
	pos := m.game.Position()
	if pos.Board().Piece(chess.E4) == chess.NoPiece {
		t.Error("after retry, e4 should still have a pawn")
	}
}

func TestPuzzleSkipMarksAsUnsolved(t *testing.T) {
	m := setupPuzzleModel(t)
	// skipAndSubmit should call submitAttempt(false) and set state to loading (navigates away)
	cmd := m.skipAndSubmit()
	if cmd == nil {
		t.Error("skipAndSubmit returned nil Cmd — expected a POST /puzzle/attempt command")
	}
}
