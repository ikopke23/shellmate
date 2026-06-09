package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

func zoomTestGame() *GameModel {
	tc := shared.TimeControl{InitialSeconds: 60, IncrementSeconds: 0}
	return NewGameModel("id", "white", "black", chess.White, "white", tc)
}

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func wantBoardResize(t *testing.T, cmd tea.Cmd, rows int) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg, ok := cmd().(BoardResizeMsg)
	if !ok {
		t.Fatalf("expected BoardResizeMsg, got %T", cmd())
	}
	if msg.Rows != rows {
		t.Fatalf("expected BoardResizeMsg{Rows:%d}, got %d", rows, msg.Rows)
	}
}

func TestGameModel_SetBoardCellSize(t *testing.T) {
	g := zoomTestGame()
	g.SetBoardCellSize(5)
	if g.BoardCellRows() != 5 {
		t.Fatalf("expected rows 5, got %d", g.BoardCellRows())
	}
	if g.board.CellCols() != 10 {
		t.Fatalf("expected cols 10, got %d", g.board.CellCols())
	}
	g.SetBoardCellSize(1)
	if g.BoardCellRows() != 5 {
		t.Fatalf("expected SetBoardCellSize(1) to be a no-op, got %d", g.BoardCellRows())
	}
	g.SetBoardCellSize(0)
	if g.BoardCellRows() != 5 {
		t.Fatalf("expected SetBoardCellSize(0) to be a no-op, got %d", g.BoardCellRows())
	}
}

func TestPuzzleModel_SetBoardCellSize(t *testing.T) {
	p := NewPuzzleModel("alice")
	p.SetBoardCellSize(6)
	if p.BoardCellRows() != 6 {
		t.Fatalf("expected rows 6, got %d", p.BoardCellRows())
	}
	p.SetBoardCellSize(1)
	if p.BoardCellRows() != 6 {
		t.Fatalf("expected no-op below minimum, got %d", p.BoardCellRows())
	}
}

func TestReplayModel_SetBoardCellSize(t *testing.T) {
	r := NewReplayModel()
	r.SetBoardCellSize(7)
	if r.BoardCellRows() != 7 {
		t.Fatalf("expected rows 7, got %d", r.BoardCellRows())
	}
	r.SetBoardCellSize(1)
	if r.BoardCellRows() != 7 {
		t.Fatalf("expected no-op below minimum, got %d", r.BoardCellRows())
	}
}

func TestGameModel_ResizeKeysEmitBoardResizeMsg(t *testing.T) {
	g := zoomTestGame() // default rows 3
	_, cmd := g.Update(runeKey(']'))
	wantBoardResize(t, cmd, 4)
	// Drive down to the floor; the handler clamps at 2.
	var m tea.Model = g
	for i := 0; i < 5; i++ {
		m, cmd = m.Update(runeKey('['))
	}
	wantBoardResize(t, cmd, 2)
}

func TestPuzzleModel_ResizeKeysEmitBoardResizeMsg(t *testing.T) {
	p := NewPuzzleModel("alice") // default rows 3
	_, cmd := p.Update(runeKey('['))
	wantBoardResize(t, cmd, 2)
}

func TestReplayModel_ResizeKeysEmitBoardResizeMsg(t *testing.T) {
	r := NewReplayModel() // default rows 3
	_, cmd := r.Update(runeKey(']'))
	wantBoardResize(t, cmd, 4)
}

// TestGameModel_BracketNotInsertedIntoMoveInput guards the requirement that the
// resize brackets are consumed before the move text input sees them.
func TestGameModel_BracketNotInsertedIntoMoveInput(t *testing.T) {
	g := zoomTestGame()
	updated, _ := g.Update(runeKey('['))
	gm := updated.(*GameModel)
	if got := gm.input.textInput.Value(); got != "" {
		t.Fatalf("expected move input to stay empty, got %q", got)
	}
}
