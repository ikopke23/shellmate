package screens

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
)

func setupImportedGames(t *testing.T) *ImportedGamesModel {
	t.Helper()
	m := NewImportedGamesModel()
	m.SetGames([]shared.HistoryRecord{
		{ID: "i1", White: "alice", Black: "bob", PlayedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)},
		{ID: "i2", White: "carol", Black: "dave", PlayedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)},
	})
	return m
}

func TestImportedGamesModel_SetGames_ClampsCursor(t *testing.T) {
	m := NewImportedGamesModel()
	m.cursor = 9
	m.SetGames([]shared.HistoryRecord{{ID: "a"}, {ID: "b"}})
	if m.cursor > 1 {
		t.Errorf("cursor = %d, want <= 1", m.cursor)
	}
}

func TestImportedGamesModel_CursorDownUp(t *testing.T) {
	m := setupImportedGames(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	im := updated.(*ImportedGamesModel)
	if im.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", im.cursor)
	}
	updated, _ = im.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	im = updated.(*ImportedGamesModel)
	if im.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", im.cursor)
	}
}

func TestImportedGamesModel_Enter_WrapsRecordInImportedGamesOpenData(t *testing.T) {
	m := setupImportedGames(t)
	m.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenReplay {
		t.Errorf("Screen = %v, want ScreenReplay", sc.Screen)
	}
	data, ok := sc.Data.(ImportedGamesOpenData)
	if !ok {
		t.Fatalf("Data type wrong, got %T", sc.Data)
	}
	if data.Record.ID != "i2" {
		t.Errorf("Record.ID = %q, want i2", data.Record.ID)
	}
}

func TestImportedGamesModel_Q_ReturnsToLobby(t *testing.T) {
	m := setupImportedGames(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenLobby {
		t.Errorf("Screen = %v, want ScreenLobby", sc.Screen)
	}
}
