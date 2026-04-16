package screens

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
)

func setupLobby(t *testing.T) *LobbyModel {
	t.Helper()
	m := NewLobbyModel("alice")
	m.SetState(shared.LobbyState{
		Players: []shared.PlayerInfo{{Username: "alice", Elo: 1500, Online: true}, {Username: "bob", Elo: 1600, Online: true}},
		Games:   []shared.GameInfo{{ID: "g1", White: "alice"}, {ID: "g2", White: "bob"}},
	})
	return m
}

func TestLobbyModel_SetState_ClampsCursor(t *testing.T) {
	m := NewLobbyModel("alice")
	m.cursor = 5
	m.SetState(shared.LobbyState{
		Games: []shared.GameInfo{{ID: "g1"}, {ID: "g2"}},
	})
	if m.cursor > 1 {
		t.Errorf("cursor = %d, want <= 1", m.cursor)
	}
}

func TestLobbyModel_CursorDown_Wraps(t *testing.T) {
	m := setupLobby(t)
	m.cursor = 1 // last index (len(games)==2)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	lm := updated.(*LobbyModel)
	if lm.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (stays at last)", lm.cursor)
	}
}

func TestLobbyModel_CursorUp_Wraps(t *testing.T) {
	m := setupLobby(t)
	m.cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	lm := updated.(*LobbyModel)
	if lm.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (stays at start)", lm.cursor)
	}
}

func TestLobbyModel_Enter_EmitsJoinGameMsg(t *testing.T) {
	m := setupLobby(t)
	m.cursor = 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd()
	jm, ok := msg.(JoinGameMsg)
	if !ok {
		t.Fatalf("msg type = %T, want JoinGameMsg", msg)
	}
	if jm.GameID != "g1" {
		t.Errorf("GameID = %q, want g1", jm.GameID)
	}
}

func TestLobbyModel_S_EmitsSpectateGameMsg(t *testing.T) {
	m := setupLobby(t)
	m.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	msg := cmd()
	sm, ok := msg.(SpectateGameMsg)
	if !ok {
		t.Fatalf("msg type = %T, want SpectateGameMsg", msg)
	}
	if sm.GameID != "g2" {
		t.Errorf("GameID = %q, want g2", sm.GameID)
	}
}

func TestLobbyModel_N_EmitsScreenChangeCreateGame(t *testing.T) {
	m := setupLobby(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenCreateGame {
		t.Errorf("Screen = %v, want ScreenCreateGame", sc.Screen)
	}
}

func TestLobbyModel_L_EmitsScreenChangeLeaderboard(t *testing.T) {
	m := setupLobby(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenLeaderboard {
		t.Errorf("Screen = %v, want ScreenLeaderboard", sc.Screen)
	}
}

func TestLobbyModel_H_EmitsScreenChangeHistory(t *testing.T) {
	m := setupLobby(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenHistory {
		t.Errorf("Screen = %v, want ScreenHistory", sc.Screen)
	}
}

func TestLobbyModel_I_EmitsScreenChangeImport(t *testing.T) {
	m := setupLobby(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenImport {
		t.Errorf("Screen = %v, want ScreenImport", sc.Screen)
	}
}

func TestLobbyModel_M_EmitsScreenChangeImportedGames(t *testing.T) {
	m := setupLobby(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenImportedGames {
		t.Errorf("Screen = %v, want ScreenImportedGames", sc.Screen)
	}
}

func TestLobbyModel_P_EmitsScreenChangePuzzle(t *testing.T) {
	m := setupLobby(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	sc, ok := cmd().(ScreenChangeMsg)
	if !ok {
		t.Fatalf("msg type wrong, got %T", cmd())
	}
	if sc.Screen != ScreenPuzzle {
		t.Errorf("Screen = %v, want ScreenPuzzle", sc.Screen)
	}
}

func TestLobbyModel_ErrMsg_StoresError(t *testing.T) {
	m := setupLobby(t)
	updated, _ := m.Update(ErrMsg{Err: errors.New("x")})
	lm := updated.(*LobbyModel)
	if lm.err != "x" {
		t.Errorf("err = %q, want %q", lm.err, "x")
	}
}
