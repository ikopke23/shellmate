package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/notnil/chess"
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

func TestLocalMoveInput_NewLocalMoveInput_DefaultState(t *testing.T) {
	li := NewLocalMoveInput(false)
	if !li.textInput.Focused() {
		t.Error("expected textInput focused by default")
	}
	if li.hasSelected {
		t.Error("expected hasSelected=false by default")
	}
	if li.pendingPromo {
		t.Error("expected pendingPromo=false by default")
	}
	if li.flipped {
		t.Error("expected flipped=false")
	}
	li2 := NewLocalMoveInput(true)
	if !li2.flipped {
		t.Error("expected flipped=true when constructed with true")
	}
}

func TestLocalMoveInput_HandleMsg_EnterSubmitsValidSAN(t *testing.T) {
	game := chess.NewGame()
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.textInput.SetValue("e4")
	san, handled, _ := li.HandleMsg(tea.KeyMsg{Type: tea.KeyEnter}, board, game)
	if !handled {
		t.Fatalf("expected handled=true for enter with valid SAN")
	}
	if san != "e4" {
		t.Fatalf("expected san='e4', got %q", san)
	}
}

func TestLocalMoveInput_HandleMsg_EnterRejectsInvalidSAN(t *testing.T) {
	game := chess.NewGame()
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.textInput.SetValue("zzz")
	san, handled, _ := li.HandleMsg(tea.KeyMsg{Type: tea.KeyEnter}, board, game)
	if !handled {
		t.Fatalf("expected handled=true for enter")
	}
	if san != "" {
		t.Fatalf("expected empty san for invalid move, got %q", san)
	}
	// submitSAN clears the value after attempting
	if li.textInput.Value() != "" {
		t.Fatalf("expected textInput cleared after invalid submit, got %q", li.textInput.Value())
	}
}

func TestLocalMoveInput_HandleMsg_MouseClickSelectsPiece(t *testing.T) {
	game := chess.NewGame()
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	// Click e2 (white pawn): non-flipped, cellCol=4, cellRow=6 → x=26, y=18
	msg := tea.MouseMsg{
		X:      26,
		Y:      18,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	san, handled, _ := li.HandleMsg(msg, board, game)
	if !handled {
		t.Fatalf("expected handled=true for click on piece")
	}
	if san != "" {
		t.Fatalf("expected empty san on first click, got %q", san)
	}
	if !li.hasSelected {
		t.Fatalf("expected hasSelected=true after clicking on own piece")
	}
	if li.selectedSq != chess.E2 {
		t.Fatalf("expected selectedSq=E2, got %v", li.selectedSq)
	}
}

func TestLocalMoveInput_HandleMsg_BoardOriginY_OffsetsClick(t *testing.T) {
	// Screens like puzzle/replay draw a title before the board, so the board's
	// top cell is at terminal row 2. With cellRows=3, e2 (cellRow 6) is drawn at
	// rows 18,19,20 in board-local space, i.e. terminal rows 20,21,22. Clicking
	// the bottom of that cell (row 22) must still resolve to E2, not the square
	// below it. This is the off-by-one regression.
	game := chess.NewGame()
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.SetBoardOriginY(2)
	// Bottom row of the e2 cell: x=26, board-local y=20 → terminal y=22.
	msg := tea.MouseMsg{X: 26, Y: 22, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	san, handled, _ := li.HandleMsg(msg, board, game)
	if !handled {
		t.Fatalf("expected handled=true for click on board")
	}
	if san != "" {
		t.Fatalf("expected empty san on first click, got %q", san)
	}
	if !li.hasSelected || li.selectedSq != chess.E2 {
		t.Fatalf("expected selectedSq=E2 after clicking bottom of e2 cell, got selected=%v sq=%v", li.hasSelected, li.selectedSq)
	}
}

func TestLocalMoveInput_HandleMsg_MouseClick_ConvertsToSAN(t *testing.T) {
	game := chess.NewGame()
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	// First click: e2 (x=26, y=18)
	click1 := tea.MouseMsg{X: 26, Y: 18, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	if _, handled, _ := li.HandleMsg(click1, board, game); !handled {
		t.Fatalf("first click not handled")
	}
	// Second click: e4 (x=26, y=12)
	click2 := tea.MouseMsg{X: 26, Y: 12, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	san, handled, _ := li.HandleMsg(click2, board, game)
	if !handled {
		t.Fatalf("second click not handled")
	}
	if san != "e4" {
		t.Fatalf("expected san='e4', got %q", san)
	}
}

func TestLocalMoveInput_HandleMsg_FlippedBoard_MapsSquaresCorrectly(t *testing.T) {
	game := chess.NewGame()
	board := render.NewBoard(game.Position(), true)
	li := NewLocalMoveInput(true)
	// Same (x=26, y=18) as the non-flipped e2 test.
	// Flipped: cellCol=4, cellRow=6 → fileIdx=7-4=3, rankIdx=6 → D7.
	msg := tea.MouseMsg{X: 26, Y: 18, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	// D7 has a black pawn but it's white's turn; no selection should happen.
	_, handled, _ := li.HandleMsg(msg, board, game)
	if !handled {
		t.Fatalf("expected handled=true for click on board")
	}
	// Since D7 is a black piece and it's white's turn, no selection.
	if li.hasSelected {
		t.Fatalf("expected no selection on opponent's piece, but selected %v", li.selectedSq)
	}
	// Now verify with a click that should resolve to a friendly square on flipped board.
	// For flipped: click at e7 (black pawn) with black to move.
	// Build black-to-move starting position (just flip turns by a move).
	game2 := chess.NewGame()
	if err := game2.MoveStr("e4"); err != nil {
		t.Fatalf("e4: %v", err)
	}
	board2 := render.NewBoard(game2.Position(), true)
	li2 := NewLocalMoveInput(true)
	// Same coords (26, 18) → D7 on flipped board, which is black pawn and black to move
	_, handled2, _ := li2.HandleMsg(msg, board2, game2)
	if !handled2 {
		t.Fatalf("expected handled=true")
	}
	if !li2.hasSelected {
		t.Fatalf("expected selection on flipped board at d7")
	}
	if li2.selectedSq != chess.D7 {
		t.Fatalf("expected selectedSq=D7 on flipped, got %v", li2.selectedSq)
	}
}

func TestLocalMoveInput_HandleMsg_PromoMode_QSelectsQueen(t *testing.T) {
	fen, _ := chess.FEN("3k4/4P3/8/8/8/8/8/4K3 w - - 0 1")
	game := chess.NewGame(fen)
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.pendingPromo = true
	li.pendingPromoFrom = chess.E7
	li.pendingPromoTo = chess.E8
	san, handled, _ := li.HandleMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}, board, game)
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if san != "e8=Q+" && san != "e8=Q" {
		t.Fatalf("expected queen promo SAN, got %q", san)
	}
	if li.pendingPromo {
		t.Fatalf("expected pendingPromo cleared after selection")
	}
}

func TestLocalMoveInput_HandlePromoClick_TopPieceRowSelectsQueen(t *testing.T) {
	// The popup is laid out as: row promoPopupY = "Promote pawn:", then the
	// piece cells on rows promoPopupY+1 .. promoPopupY+cellRows. Clicking the
	// FIRST piece row (promoPopupY+1) on the queen (leftmost, x=2) must select
	// the queen. This is the vertical off-by-one regression.
	fen, _ := chess.FEN("3k4/4P3/8/8/8/8/8/4K3 w - - 0 1")
	game := chess.NewGame(fen)
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.pendingPromo = true
	li.pendingPromoFrom = chess.E7
	li.pendingPromoTo = chess.E8
	li.SetPromoPopupY(26)
	// Top piece row, queen column: x=2, y=27.
	click := tea.MouseMsg{X: 2, Y: 27, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	san, handled, _ := li.HandleMsg(click, board, game)
	if !handled {
		t.Fatalf("expected handled=true for click on promo popup")
	}
	if san != "e8=Q+" && san != "e8=Q" {
		t.Fatalf("expected queen promo SAN from top piece row, got %q", san)
	}
	if li.pendingPromo {
		t.Fatalf("expected pendingPromo cleared after selection")
	}
}

func TestLocalMoveInput_HandlePromoClick_LabelRowIsNotAPiece(t *testing.T) {
	// The key-label row below the pieces (promoPopupY+cellRows+1) must NOT
	// register as a piece selection.
	fen, _ := chess.FEN("3k4/4P3/8/8/8/8/8/4K3 w - - 0 1")
	game := chess.NewGame(fen)
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.pendingPromo = true
	li.pendingPromoFrom = chess.E7
	li.pendingPromoTo = chess.E8
	li.SetPromoPopupY(26)
	// cellRows defaults to 3, so the label row is at 26+3+1 = 30.
	click := tea.MouseMsg{X: 2, Y: 30, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	san, _, _ := li.HandleMsg(click, board, game)
	if san != "" {
		t.Fatalf("expected no promo selection on label row, got %q", san)
	}
	if !li.pendingPromo {
		t.Fatalf("expected pendingPromo to remain set after clicking label row")
	}
}

func TestLocalMoveInput_HandleMsg_PromoMode_EscCancels(t *testing.T) {
	fen, _ := chess.FEN("3k4/4P3/8/8/8/8/8/4K3 w - - 0 1")
	game := chess.NewGame(fen)
	board := render.NewBoard(game.Position(), false)
	li := NewLocalMoveInput(false)
	li.pendingPromo = true
	li.pendingPromoFrom = chess.E7
	li.pendingPromoTo = chess.E8
	san, handled, _ := li.HandleMsg(tea.KeyMsg{Type: tea.KeyEsc}, board, game)
	if !handled {
		t.Fatalf("expected handled=true for esc")
	}
	if san != "" {
		t.Fatalf("expected empty san on cancel, got %q", san)
	}
	if li.pendingPromo {
		t.Fatalf("expected pendingPromo cleared after esc")
	}
}
