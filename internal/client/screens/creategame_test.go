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

func TestCreateGame_Enter_EmitsCreateGameMsg(t *testing.T) {
	m := NewCreateGameModel()
	m.cursor = 1 // Bullet 1+0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on enter")
	}
	msg := cmd()
	cg, ok := msg.(CreateGameMsg)
	if !ok {
		t.Fatalf("expected CreateGameMsg, got %T", msg)
	}
	if cg.TimeControl.InitialSeconds != 60 || cg.TimeControl.IncrementSeconds != 0 {
		t.Fatalf("expected 60+0, got %+v", cg.TimeControl)
	}
}

func TestCreateGame_C_EntersCustomMode(t *testing.T) {
	m := NewCreateGameModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	cgm := updated.(*CreateGameModel)
	if !cgm.customMode {
		t.Fatalf("expected customMode=true after pressing c")
	}
	if cgm.focusIdx != 0 {
		t.Fatalf("expected focusIdx=0, got %d", cgm.focusIdx)
	}
}

func TestCreateGame_Esc_ExitsCustomMode(t *testing.T) {
	m := NewCreateGameModel()
	m.customMode = true
	m.err = "something"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	cgm := updated.(*CreateGameModel)
	if cgm.customMode {
		t.Fatalf("expected customMode=false after esc")
	}
	if cgm.err != "" {
		t.Fatalf("expected err cleared, got %q", cgm.err)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd when exiting custom mode")
	}
}

func TestCreateGame_CustomMode_TabAdvancesField(t *testing.T) {
	m := NewCreateGameModel()
	m.customMode = true
	m.focusIdx = 0
	m.inputs[0].Focus()
	m.inputs[1].Blur()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	cgm := updated.(*CreateGameModel)
	if cgm.focusIdx != 1 {
		t.Fatalf("expected focusIdx=1 after tab, got %d", cgm.focusIdx)
	}
}

func TestCreateGame_CustomMode_EmptyMinutes_NoSubmit(t *testing.T) {
	m := NewCreateGameModel()
	m.customMode = true
	// leave both inputs empty
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected nil cmd when minutes empty, got %v", cmd)
	}
	if m.err == "" {
		t.Fatalf("expected err to be set for invalid submit")
	}
}

func TestCreateGame_CustomMode_ValidSubmit_EmitsCreateGameMsg(t *testing.T) {
	m := NewCreateGameModel()
	m.customMode = true
	m.inputs[0].SetValue("10")
	m.inputs[1].SetValue("5")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for valid custom submit")
	}
	msg := cmd()
	cg, ok := msg.(CreateGameMsg)
	if !ok {
		t.Fatalf("expected CreateGameMsg, got %T", msg)
	}
	if cg.TimeControl.InitialSeconds != 600 || cg.TimeControl.IncrementSeconds != 5 {
		t.Fatalf("expected 600+5, got %+v", cg.TimeControl)
	}
}

func TestCreateGame_BackToLobby(t *testing.T) {
	m := NewCreateGameModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for esc from preset mode")
	}
	msg := cmd()
	sc, ok := msg.(ScreenChangeMsg)
	if !ok {
		t.Fatalf("expected ScreenChangeMsg, got %T", msg)
	}
	if sc.Screen != ScreenLobby {
		t.Fatalf("expected Screen=ScreenLobby, got %v", sc.Screen)
	}
}
