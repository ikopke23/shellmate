package screens

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
)

func setupHistory(t *testing.T) *HistoryModel {
	t.Helper()
	m := NewHistoryModel("alice")
	m.SetGames([]shared.HistoryRecord{
		{ID: "g1", White: "alice", Black: "bob", Result: "1-0", PGN: "1. e4 e5", PlayedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)},
		{ID: "g2", White: "bob", Black: "alice", Result: "0-1", PGN: "1. d4 d5", PlayedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)},
	})
	return m
}

func TestHistoryModel_SetGames_ClampsCursor(t *testing.T) {
	m := NewHistoryModel("alice")
	m.cursor = 7
	m.SetGames([]shared.HistoryRecord{{ID: "a"}, {ID: "b"}})
	if m.cursor > 1 {
		t.Errorf("cursor = %d, want <= 1", m.cursor)
	}
}

func TestHistoryModel_CursorDown(t *testing.T) {
	m := setupHistory(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	hm := updated.(*HistoryModel)
	if hm.cursor != 1 {
		t.Errorf("cursor = %d, want 1", hm.cursor)
	}
}

func TestHistoryModel_CursorUp(t *testing.T) {
	m := setupHistory(t)
	m.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	hm := updated.(*HistoryModel)
	if hm.cursor != 0 {
		t.Errorf("cursor = %d, want 0", hm.cursor)
	}
}

func TestHistoryModel_Enter_EmitsScreenChangeReplay(t *testing.T) {
	m := setupHistory(t)
	m.cursor = 0
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
	rec, ok := sc.Data.(shared.HistoryRecord)
	if !ok {
		t.Fatalf("Data type wrong, got %T", sc.Data)
	}
	if rec.ID != "g1" {
		t.Errorf("Data.ID = %q, want g1", rec.ID)
	}
}

func TestHistoryModel_E_PopulatesClipboardSeq(t *testing.T) {
	m := setupHistory(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	hm := updated.(*HistoryModel)
	if hm.clipboardSeq == "" {
		t.Error("clipboardSeq is empty, want non-empty OSC 52 sequence")
	}
}

func TestHistoryModel_ClearClipboardMsg_ResetsSeq(t *testing.T) {
	m := setupHistory(t)
	m.clipboardSeq = "X"
	updated, _ := m.Update(clearClipboardMsg{})
	hm := updated.(*HistoryModel)
	if hm.clipboardSeq != "" {
		t.Errorf("clipboardSeq = %q, want empty", hm.clipboardSeq)
	}
}

func TestHistoryModel_Q_ReturnsToLobby(t *testing.T) {
	m := setupHistory(t)
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
