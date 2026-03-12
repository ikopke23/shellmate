package render

import (
	"strings"
	"testing"
)

func TestMoveListViewFormat(t *testing.T) {
	ml := NewMoveList(10)
	ml.SetMoves([]string{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6"}, 5)
	view := ml.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 rows, got %d: %q", len(lines), lines)
	}
	plain := stripANSI(view)
	if !strings.Contains(plain, "1.") {
		t.Errorf("expected move number '1.' in output: %q", plain)
	}
	if !strings.Contains(plain, "e4") {
		t.Errorf("expected 'e4' in output: %q", plain)
	}
	if !strings.Contains(plain, "e5") {
		t.Errorf("expected 'e5' in output: %q", plain)
	}
	if !strings.Contains(plain, "Nf3") {
		t.Errorf("expected 'Nf3' in output: %q", plain)
	}
	if !strings.Contains(plain, "Nc6") {
		t.Errorf("expected 'Nc6' in output: %q", plain)
	}
	if !strings.Contains(plain, "Bb5") {
		t.Errorf("expected 'Bb5' in output: %q", plain)
	}
	if !strings.Contains(plain, "a6") {
		t.Errorf("expected 'a6' in output: %q", plain)
	}
}

func TestMoveListIncompletePair(t *testing.T) {
	ml := NewMoveList(10)
	ml.SetMoves([]string{"e4"}, 0)
	view := ml.View()
	plain := stripANSI(view)
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 row for single move, got %d: %q", len(lines), lines)
	}
	if !strings.Contains(plain, "1.") || !strings.Contains(plain, "e4") {
		t.Errorf("unexpected output: %q", plain)
	}
}

func TestMoveListScrollDown(t *testing.T) {
	ml := NewMoveList(2)
	ml.SetMoves([]string{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6", "Ba4", "Nf6"}, 6)
	if ml.scrollOffset != 2 {
		t.Errorf("expected scrollOffset=2 after scroll-down, got %d", ml.scrollOffset)
	}
	plain := stripANSI(ml.View())
	if !strings.Contains(plain, "3.") {
		t.Errorf("expected row starting with '3.' in view: %q", plain)
	}
	if !strings.Contains(plain, "4.") {
		t.Errorf("expected row starting with '4.' in view: %q", plain)
	}
	if strings.Contains(plain, "1.") {
		t.Errorf("row 1 should be scrolled out: %q", plain)
	}
}

func TestMoveListScrollUp(t *testing.T) {
	ml := NewMoveList(2)
	ml.SetMoves([]string{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6", "Ba4", "Nf6"}, 6)
	ml.SetMoves([]string{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6", "Ba4", "Nf6"}, 0)
	if ml.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after scroll-up, got %d", ml.scrollOffset)
	}
	plain := stripANSI(ml.View())
	if !strings.Contains(plain, "1.") {
		t.Errorf("expected row 1 visible after scroll-up: %q", plain)
	}
}

func TestMoveListEmpty(t *testing.T) {
	ml := NewMoveList(4)
	view := ml.View()
	if view != "\n\n\n\n" {
		t.Errorf("expected 4 newlines for empty move list, got %q", view)
	}
}

func TestMoveListCurrentIdxNone(t *testing.T) {
	ml := NewMoveList(4)
	ml.SetMoves([]string{"e4", "e5"}, -1)
	view := ml.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "e4") {
		t.Errorf("expected moves rendered even with currentIdx=-1: %q", plain)
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
