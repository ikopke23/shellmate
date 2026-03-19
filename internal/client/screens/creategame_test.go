package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
)

func TestCreateGame_PresetSelection(t *testing.T) {
	m := NewCreateGameModel()
	// navigate down twice (past Untimed and Bullet 1+0)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	cgm := m3.(*CreateGameModel)
	if cgm.cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", cgm.cursor)
	}
}

func TestCreateGame_PresetsCorrect(t *testing.T) {
	m := NewCreateGameModel()
	// Bullet 1+1 is index 2
	m.cursor = 2
	tc := m.selectedTimeControl()
	if tc.InitialSeconds != 60 || tc.IncrementSeconds != 1 {
		t.Fatalf("expected 60+1, got %+v", tc)
	}
}

func TestCreateGame_Untimed(t *testing.T) {
	m := NewCreateGameModel()
	m.cursor = 0
	tc := m.selectedTimeControl()
	if tc != (shared.TimeControl{}) {
		t.Fatalf("expected zero TimeControl for untimed, got %+v", tc)
	}
}

func TestCreateGame_NegativeIncrement(t *testing.T) {
	m := NewCreateGameModel()
	m.customMode = true
	m.inputs[0].SetValue("5")
	m.inputs[1].SetValue("-3")
	tc := m.selectedTimeControl()
	if tc.IncrementSeconds < 0 {
		t.Fatalf("expected increment clamped to >= 0, got %d", tc.IncrementSeconds)
	}
}
