package render

import (
	"testing"
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
