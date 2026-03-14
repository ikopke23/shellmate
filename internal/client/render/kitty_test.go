package render

import (
	"testing"

	"github.com/notnil/chess"
)

func TestDetectKitty_Term(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("KITTY_WINDOW_ID", "")
	DetectKitty()
	if !kittyEnabled {
		t.Error("expected kittyEnabled=true with TERM=xterm-kitty")
	}
}

func TestDetectKitty_WindowID(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "3")
	DetectKitty()
	if !kittyEnabled {
		t.Error("expected kittyEnabled=true with KITTY_WINDOW_ID set")
	}
}

func TestDetectKitty_None(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "")
	DetectKitty()
	if kittyEnabled {
		t.Error("expected kittyEnabled=false with no Kitty env vars")
	}
}

func TestComposeBoard_Dimensions(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	img := composeBoard(b)
	if img.Bounds().Dx() != 480 || img.Bounds().Dy() != 480 {
		t.Errorf("expected 480x480, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestComposeBoard_EmptySquareBg(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	img := composeBoard(b)
	// e4 (rank index 3, file index 4): isLight=(4+3)%2=1 → light square
	// Not flipped: rankOrder[4]=3 → screenRow=4; fileOrder[4]=4 → screenCol=4
	cx := 4*pieceSize + pieceSize/2
	cy := 4*pieceSize + pieceSize/2
	got := img.RGBAAt(cx, cy)
	want := hexToRGBA(string(lightSquareBg))
	if got != want {
		t.Errorf("e4 bg: want %v, got %v", want, got)
	}
}

func TestComposeBoard_Flipped(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), true)
	img := composeBoard(b)
	if img.Bounds().Dx() != 480 || img.Bounds().Dy() != 480 {
		t.Errorf("expected 480x480, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}
