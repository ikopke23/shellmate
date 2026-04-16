package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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

func TestImport_NewImportModel_TextareaFocused(t *testing.T) {
	m := NewImportModel()
	if !m.textarea.Focused() {
		t.Fatalf("expected textarea to be focused")
	}
}

func TestImport_Submit_EmptyInput_NoCmd(t *testing.T) {
	m := NewImportModel()
	// textarea empty
	_, cmd := m.submit()
	if cmd != nil {
		t.Fatalf("expected nil cmd for empty input, got %v", cmd)
	}
	if m.err == "" {
		t.Fatalf("expected err to be set for empty input")
	}
}

func TestImport_Submit_TextMode_EmitsScreenChangeReplay(t *testing.T) {
	m := NewImportModel()
	m.textarea.SetValue("1. e4 e5 2. Nf3 Nc6")
	_, cmd := m.submit()
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
	msg := cmd()
	sc, ok := msg.(ScreenChangeMsg)
	if !ok {
		t.Fatalf("expected ScreenChangeMsg, got %T", msg)
	}
	if sc.Screen != ScreenReplay {
		t.Fatalf("expected ScreenReplay, got %v", sc.Screen)
	}
	data, ok := sc.Data.(ImportPGNData)
	if !ok {
		t.Fatalf("expected ImportPGNData, got %T", sc.Data)
	}
	if !strings.Contains(data.Record.PGN, "e4") {
		t.Fatalf("expected PGN to contain 'e4', got %q", data.Record.PGN)
	}
}

func TestImport_Submit_FileMode_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pgn")
	content := "1. e4 e5"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m := NewImportModel()
	m.textarea.SetValue(path)
	_, cmd := m.submit()
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for valid file, err=%q", m.err)
	}
	msg := cmd()
	sc, ok := msg.(ScreenChangeMsg)
	if !ok {
		t.Fatalf("expected ScreenChangeMsg, got %T", msg)
	}
	data := sc.Data.(ImportPGNData)
	if !strings.Contains(data.Record.PGN, "e4") {
		t.Fatalf("expected PGN from file, got %q", data.Record.PGN)
	}
}

func TestImport_Submit_FileMode_NonexistentFile_SetsErr(t *testing.T) {
	m := NewImportModel()
	m.textarea.SetValue("/nonexistent/path/to/file.pgn")
	_, cmd := m.submit()
	if cmd != nil {
		t.Fatalf("expected nil cmd for missing file")
	}
	if m.err == "" {
		t.Fatalf("expected err to be set")
	}
	if !strings.Contains(m.err, "could not read file") {
		t.Fatalf("expected err about reading file, got %q", m.err)
	}
}

func TestImport_Submit_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	m := NewImportModel()
	m.textarea.SetValue("~/definitely-nonexistent-shellmate-test.pgn")
	_, cmd := m.submit()
	if cmd != nil {
		t.Fatalf("expected nil cmd (missing file)")
	}
	if m.err == "" {
		t.Fatalf("expected err for missing file")
	}
	// The error should mention the expanded home path, proving ~/ expansion happened.
	if !strings.Contains(m.err, home) {
		t.Fatalf("expected err to contain expanded home path %q, got %q", home, m.err)
	}
}

func TestImport_Esc_ReturnsToLobby(t *testing.T) {
	m := NewImportModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on esc")
	}
	msg := cmd()
	sc, ok := msg.(ScreenChangeMsg)
	if !ok {
		t.Fatalf("expected ScreenChangeMsg, got %T", msg)
	}
	if sc.Screen != ScreenLobby {
		t.Fatalf("expected ScreenLobby, got %v", sc.Screen)
	}
}
