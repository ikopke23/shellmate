package screens

import "testing"

const testFilePath = "file path"

func TestDetectImportMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/game.pgn", testFilePath},
		{"~/Downloads/game.pgn", testFilePath},
		{"./game.pgn", testFilePath},
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
