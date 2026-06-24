package render

import (
	"testing"

	"github.com/notnil/chess"
)

func TestBoard_SetPremoves_HighlightsSquares(t *testing.T) {
	g := chess.NewGame()
	b := NewBoard(g.Position(), false)
	b.SetPremoves([][2]chess.Square{{chess.E2, chess.E4}})
	// e2: fileIdx=4, rankIdx=1 → (4+1)%2=1 → isLight=true
	got := b.squareBgHex(chess.E2, 4, 1)
	if got != string(premoveLightBg) {
		t.Errorf("e2: expected premoveLightBg %q, got %q", premoveLightBg, got)
	}
	// e4: fileIdx=4, rankIdx=3 → (4+3)%2=1 → isLight=true
	got = b.squareBgHex(chess.E4, 4, 3)
	if got != string(premoveLightBg) {
		t.Errorf("e4: expected premoveLightBg %q, got %q", premoveLightBg, got)
	}
	// d4: fileIdx=3, rankIdx=3 → (3+3)%2=0 → isLight=false (not premove square)
	got = b.squareBgHex(chess.D4, 3, 3)
	if got != string(darkSquareBg) {
		t.Errorf("d4: expected default darkSquareBg, got %q", got)
	}
}

func TestBoard_PremoveColor_BelowCheck(t *testing.T) {
	g := chess.NewGame()
	b := NewBoard(g.Position(), false)
	// Set check on e1 and premove on e1 — check must win.
	b.SetCheck(chess.E1)
	b.SetPremoves([][2]chess.Square{{chess.E1, chess.E2}})
	// e1: fileIdx=4, rankIdx=0 → (4+0)%2=0 → isLight=false
	got := b.squareBgHex(chess.E1, 4, 0)
	if got != string(checkDarkBg) {
		t.Errorf("check should win over premove: expected checkDarkBg %q, got %q", checkDarkBg, got)
	}
}

func TestBoard_ClearPremoves_RestoresDefault(t *testing.T) {
	g := chess.NewGame()
	b := NewBoard(g.Position(), false)
	b.SetPremoves([][2]chess.Square{{chess.E2, chess.E4}})
	b.ClearPremoves()
	// e2 should now be default light
	got := b.squareBgHex(chess.E2, 4, 1)
	if got != string(lightSquareBg) {
		t.Errorf("after ClearPremoves: expected lightSquareBg %q, got %q", lightSquareBg, got)
	}
}
