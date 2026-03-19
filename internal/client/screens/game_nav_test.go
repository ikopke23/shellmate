package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

func newTestGame() *GameModel {
	tc := shared.TimeControl{InitialSeconds: 60, IncrementSeconds: 0}
	g := NewGameModel("id", "white", "black", chess.White, nil, "white", tc)
	g.SetMovesWithClock([]string{"e4", "e5", "Nf3"}, shared.ClockState{WhiteMs: 58000, BlackMs: 59000})
	return g
}

func TestViewIdx_InitialIsLive(t *testing.T) {
	g := newTestGame()
	if g.viewIdx != 3 {
		t.Fatalf("expected viewIdx==3 (len moves), got %d", g.viewIdx)
	}
}

func TestViewIdx_LeftArrowDecrements(t *testing.T) {
	g := newTestGame()
	updated, _ := g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	gm := updated.(*GameModel)
	if gm.viewIdx != 2 {
		t.Fatalf("expected viewIdx==2, got %d", gm.viewIdx)
	}
}

func TestViewIdx_ClampAtZero(t *testing.T) {
	var m tea.Model = newTestGame()
	for i := 0; i < 10; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	gm := m.(*GameModel)
	if gm.viewIdx != 0 {
		t.Fatalf("expected viewIdx clamped at 0, got %d", gm.viewIdx)
	}
}

func TestViewIdx_SnapOnNewMove(t *testing.T) {
	g := newTestGame()
	updated, _ := g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	gm := updated.(*GameModel)
	gm.SetMovesWithClock([]string{"e4", "e5", "Nf3", "Nc6"}, shared.ClockState{})
	if gm.viewIdx != 4 {
		t.Fatalf("expected viewIdx snapped to 4, got %d", gm.viewIdx)
	}
}

func TestClockState_SetFromMove(t *testing.T) {
	g := newTestGame()
	if g.whiteMs != 58000 || g.blackMs != 59000 {
		t.Fatalf("expected 58000/59000, got %d/%d", g.whiteMs, g.blackMs)
	}
}
