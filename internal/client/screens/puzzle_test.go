package screens

import (
	"testing"

	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// setupPuzzleModel builds a PuzzleModel already past the loading state using a real
// chess position. The FEN is the position after 1.e4 (black to move). solution[0] is
// "d7d5" — the player's (black's) move. No engine response follows, so the puzzle
// completes in one player move.
func setupPuzzleModel(t *testing.T) *PuzzleModel {
	t.Helper()
	record := shared.PuzzleRecord{
		ID:               "test1",
		FEN:              "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		Moves:            "d7d5",
		Rating:           1500,
		Themes:           []string{"opening"},
		UserPuzzleRating: 1500,
	}
	m := NewPuzzleModel("localhost:8080", "testuser")
	m.SetPuzzle(record)
	return m
}

func TestPuzzleModelInitLoadsPosition(t *testing.T) {
	m := setupPuzzleModel(t)
	// FEN already has e4 played — e4 should have a pawn, e2 should be empty.
	pos := m.game.Position()
	if pos.Board().Piece(chess.E4) == chess.NoPiece {
		t.Error("expected pawn on e4 from FEN, got empty square")
	}
	if pos.Board().Piece(chess.E2) != chess.NoPiece {
		t.Error("expected e2 to be empty (pawn already on e4)")
	}
	// solutionIdx should be 0 — player plays solution[0] next
	if m.solutionIdx != 0 {
		t.Errorf("solutionIdx = %d, want 0", m.solutionIdx)
	}
	if m.state != puzzleStatePlaying {
		t.Errorf("state = %v, want puzzleStatePlaying", m.state)
	}
}

func TestPuzzleValidateCorrectMove(t *testing.T) {
	m := setupPuzzleModel(t)
	// solution[0] is "d7d5" which in SAN is "d5"
	ok, _ := m.validateAndApply("d5")
	if !ok {
		t.Error("validateAndApply returned false for correct move d5")
	}
	// Single-move solution with no engine response — should be success
	if m.state != puzzleStateSuccess {
		t.Errorf("state = %v, want puzzleStateSuccess", m.state)
	}
}

func TestPuzzleValidateWrongMove(t *testing.T) {
	m := setupPuzzleModel(t)
	// "e5" is a valid chess move for black but not the expected solution move
	ok, _ := m.validateAndApply("e5")
	if ok {
		t.Error("validateAndApply returned true for wrong move e5")
	}
	if m.state != puzzleStateFailure {
		t.Errorf("state = %v, want puzzleStateFailure", m.state)
	}
}

func TestPuzzleValidateInvalidSAN(t *testing.T) {
	m := setupPuzzleModel(t)
	ok, _ := m.validateAndApply("zz9")
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
	// FEN position should be restored — e4 should still have a pawn
	pos := m.game.Position()
	if pos.Board().Piece(chess.E4) == chess.NoPiece {
		t.Error("after retry, e4 should still have a pawn (restored from FEN)")
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

func setupMultiMovePuzzle(t *testing.T) *PuzzleModel {
	t.Helper()
	record := shared.PuzzleRecord{
		ID:               "multi1",
		FEN:              "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		Moves:            "d7d5 e4d5 d8d5",
		Rating:           1500,
		UserPuzzleRating: 1500,
	}
	m := NewPuzzleModel("localhost:8080", "testuser")
	m.SetPuzzle(record)
	return m
}

func TestEngineResponseApplied(t *testing.T) {
	m := setupMultiMovePuzzle(t)
	ok, engineUCI := m.validateAndApply("d5")
	if !ok {
		t.Fatal("correct first move d5 was rejected")
	}
	if engineUCI != "e4d5" {
		t.Fatalf("expected engine UCI e4d5, got %q", engineUCI)
	}
	m.applyEngineResponse(engineUCI)
	if m.solutionIdx != 2 {
		t.Fatalf("solutionIdx = %d after engine response, want 2", m.solutionIdx)
	}
	if m.state != puzzleStatePlaying {
		t.Fatalf("state = %v after engine response, want puzzleStatePlaying", m.state)
	}
}

func TestEngineResponseCompletesOnLastMove(t *testing.T) {
	// Two-move puzzle: player plays, engine's reply is the last move → success
	record := shared.PuzzleRecord{
		ID:               "two1",
		FEN:              "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		Moves:            "d7d5 e4d5",
		Rating:           1500,
		UserPuzzleRating: 1500,
	}
	m := NewPuzzleModel("localhost:8080", "testuser")
	m.SetPuzzle(record)
	ok, engineUCI := m.validateAndApply("d5")
	if !ok {
		t.Fatal("correct move rejected")
	}
	m.applyEngineResponse(engineUCI)
	if m.state != puzzleStateSuccess {
		t.Fatalf("state = %v, want puzzleStateSuccess", m.state)
	}
}

func TestInitGameFlipsForBlack(t *testing.T) {
	// The existing setupPuzzleModel uses a black-to-move FEN
	m := setupPuzzleModel(t)
	if !m.board.Flipped() {
		t.Error("board should be flipped for black-to-move puzzle")
	}
	if !m.input.flipped {
		t.Error("input should be flipped for black-to-move puzzle")
	}
}

func TestInitGameNoFlipForWhite(t *testing.T) {
	record := shared.PuzzleRecord{
		ID:               "white1",
		FEN:              "rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 3",
		Moves:            "f3e5",
		Rating:           1500,
		UserPuzzleRating: 1500,
	}
	m := NewPuzzleModel("localhost:8080", "testuser")
	m.SetPuzzle(record)
	if m.board.Flipped() {
		t.Error("board should NOT be flipped for white-to-move puzzle")
	}
	if m.input.flipped {
		t.Error("input should NOT be flipped for white-to-move puzzle")
	}
}
