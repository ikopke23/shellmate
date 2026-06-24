package render

import (
	"strings"
	"testing"
)

func TestPremoveList_ViewEmpty_ReturnsEmpty(t *testing.T) {
	pl := NewPremoveList()
	if pl.View() != "" {
		t.Errorf("expected empty View() for new PremoveList, got %q", pl.View())
	}
}

func TestPremoveList_ViewWithMoves_ShowsNumberedSAN(t *testing.T) {
	pl := NewPremoveList()
	pl.SetMoves([]string{"e4", "Nf3"})
	v := pl.View()
	if !strings.Contains(v, "1. e4") {
		t.Errorf("expected '1. e4' in View(), got %q", v)
	}
	if !strings.Contains(v, "2. Nf3") {
		t.Errorf("expected '2. Nf3' in View(), got %q", v)
	}
}

func TestPremoveList_ViewWithMoves_ShowsHeader(t *testing.T) {
	pl := NewPremoveList()
	pl.SetMoves([]string{"e4"})
	if !strings.Contains(pl.View(), "Premoves") {
		t.Errorf("expected 'Premoves' header in View(), got %q", pl.View())
	}
}

func TestPremoveList_SetMovesNil_ClearsView(t *testing.T) {
	pl := NewPremoveList()
	pl.SetMoves([]string{"e4"})
	pl.SetMoves(nil)
	if pl.View() != "" {
		t.Errorf("expected empty View() after SetMoves(nil), got %q", pl.View())
	}
}
