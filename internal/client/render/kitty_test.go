package render

import (
	"bytes"
	"image"
	"strings"
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

func TestBuildKittyUpload_Format(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	buildKittyUpload(img, 1, &buf)
	out := buf.String()

	checks := []string{"a=T", "f=32", "U=1", "i=1", "s=4", "v=4", "m=0"}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in upload sequence: %q", c, out[:min(len(out), 80)])
		}
	}
	if !strings.HasPrefix(out, "\033_G") {
		t.Errorf("expected APC start \\033_G, got: %q", out[:min(len(out), 10)])
	}
	if !strings.HasSuffix(out, "\033\\") {
		t.Errorf("expected APC end \\033\\\\, got: %q", out[max(0, len(out)-5):])
	}
}

func TestBuildKittyUpload_Chunking(t *testing.T) {
	// 120x120 image → 120*120*4=57600 bytes → base64 ~76800 chars → ~19 chunks
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	var buf bytes.Buffer
	buildKittyUpload(img, 2, &buf)
	out := buf.String()
	// First chunk must have m=1 (more data follows)
	firstEnd := strings.Index(out, "\033\\")
	firstChunk := out[:firstEnd]
	if !strings.Contains(firstChunk, "m=1") {
		t.Errorf("first chunk should have m=1: %q", firstChunk[:min(len(firstChunk), 80)])
	}
	// Last chunk must have m=0
	lastStart := strings.LastIndex(out, "\033_G")
	lastChunk := out[lastStart:]
	if !strings.Contains(lastChunk, "m=0") {
		t.Errorf("last chunk should have m=0: %q", lastChunk[:min(len(lastChunk), 80)])
	}
}

func TestBuildPlaceholderString_RowCount(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	b.cellCols = 6
	b.cellRows = 3
	s := buildPlaceholderString(b, 1)
	// 8 ranks × 3 lines + 1 file label line = 25 lines
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) != 25 {
		t.Errorf("expected 25 lines (8*3+1), got %d", len(lines))
	}
}

func TestBuildPlaceholderString_PlaceholderChars(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	b.cellCols = 6
	b.cellRows = 3
	s := buildPlaceholderString(b, 1)
	lines := strings.Split(s, "\n")
	// First content line (line 0): should have 8*6=48 placeholder chars
	count := strings.Count(lines[0], kittyPlaceholder)
	if count != 48 {
		t.Errorf("expected 48 placeholder chars per line, got %d", count)
	}
}

func TestBuildPlaceholderString_IDColorCode(t *testing.T) {
	b := NewBoard(chess.NewGame().Position(), false)
	b.cellCols = 6
	b.cellRows = 3
	s := buildPlaceholderString(b, 2)
	if !strings.Contains(s, "\033[38;5;2m") {
		t.Errorf("expected color code for id=2, not found in output")
	}
}
