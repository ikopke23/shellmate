package screens

import (
	"github.com/notnil/chess"
	"testing"
)

func TestLocalMoveInput_SubmitValidSAN(t *testing.T) {
	game := chess.NewGame()
	li := NewLocalMoveInput(false)
	li.textInput.SetValue("e4")
	san := li.submitSAN(game)
	if san != "e4" {
		t.Errorf("expected san='e4', got %q", san)
	}
	if li.textInput.Value() != "" {
		t.Error("expected textInput cleared after submit")
	}
}

func TestLocalMoveInput_SubmitInvalidSAN(t *testing.T) {
	game := chess.NewGame()
	li := NewLocalMoveInput(false)
	li.textInput.SetValue("z9")
	san := li.submitSAN(game)
	if san != "" {
		t.Errorf("expected empty san for invalid move, got %q", san)
	}
}

func TestLocalMoveInput_IsPromotionMove(t *testing.T) {
	fen, _ := chess.FEN("3k4/4P3/8/8/8/8/8/4K3 w - - 0 1")
	game := chess.NewGame(fen)
	li := NewLocalMoveInput(false)
	if !li.isPromotionMove(chess.E7, chess.E8, game) {
		t.Error("expected e7→e8 to be a promotion move")
	}
	fen2, _ := chess.FEN("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")
	game2 := chess.NewGame(fen2)
	if li.isPromotionMove(chess.E4, chess.E5, game2) {
		t.Error("e4→e5 should not be a promotion move")
	}
}
