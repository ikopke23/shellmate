package screens

import "testing"

func TestDetectImportMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/game.pgn", "file path"},
		{"~/Downloads/game.pgn", "file path"},
		{"./game.pgn", "file path"},
		{"[Event \"Test\"]\n1. e4 e5", "pgn text"},
		{"", "pgn text"},
	}
	for _, tt := range tests {
		got := detectImportMode(tt.input)
		if got != tt.want {
			t.Errorf("detectImportMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
