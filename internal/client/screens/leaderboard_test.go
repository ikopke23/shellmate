package screens

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
)

func TestLeaderboardModel_SetPlayers(t *testing.T) {
	m := NewLeaderboardModel()
	m.SetPlayers([]shared.PlayerInfo{
		{Username: "a", Elo: 1500},
		{Username: "b", Elo: 1600},
		{Username: "c", Elo: 1700},
	})
	if len(m.players) != 3 {
		t.Errorf("len(players) = %d, want 3", len(m.players))
	}
}

func TestLeaderboardModel_Q_ReturnsToLobby(t *testing.T) {
	m := NewLeaderboardModel()
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

func TestLeaderboardModel_ErrMsg_StoresError(t *testing.T) {
	m := NewLeaderboardModel()
	updated, _ := m.Update(ErrMsg{Err: errors.New("boom")})
	lm := updated.(*LeaderboardModel)
	if lm.err != "boom" {
		t.Errorf("err = %q, want %q", lm.err, "boom")
	}
}
