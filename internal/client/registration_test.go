package client

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/client/screens"
	"github.com/ikopke/shellmate/internal/server"
)

func setupRegistration(t *testing.T) RegistrationModel {
	t.Helper()
	hub := &server.Hub{}
	return NewRegistrationModel(hub, "SHA256:fakefp", 80, 24)
}

func TestRegistration_NewRegistrationModel_DefaultStep(t *testing.T) {
	r := setupRegistration(t)
	if r.step != stepUsername {
		t.Errorf("step = %v, want stepUsername", r.step)
	}
	if r.width != 80 {
		t.Errorf("width = %d, want 80", r.width)
	}
	if r.height != 24 {
		t.Errorf("height = %d, want 24", r.height)
	}
	if !r.input.Focused() {
		t.Errorf("input not focused")
	}
	if r.input.Placeholder != "username" {
		t.Errorf("placeholder = %q, want %q", r.input.Placeholder, "username")
	}
	if r.fingerprint != "SHA256:fakefp" {
		t.Errorf("fingerprint = %q, want SHA256:fakefp", r.fingerprint)
	}
}

func TestRegistration_Init_ReturnsBlinkCmd(t *testing.T) {
	r := setupRegistration(t)
	cmd := r.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
}

func TestRegistration_Update_CtrlC_EmitsQuit(t *testing.T) {
	r := setupRegistration(t)
	_, cmd := r.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c returned nil cmd, want tea.Quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd produced %T, want tea.QuitMsg", msg)
	}
}

func TestRegistration_Update_Enter_EmptyInput_NoCmd(t *testing.T) {
	r := setupRegistration(t)
	_, cmd := r.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("empty-input enter returned non-nil cmd")
	}
}

func TestRegistration_Update_Enter_WithUsername_ReturnsNonNilCmd(t *testing.T) {
	r := setupRegistration(t)
	r.input.SetValue("alice")
	_, cmd := r.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter with username returned nil cmd")
	}
}

func TestRegistration_Update_UsernameTakenMsg_AdvancesToInviteStep(t *testing.T) {
	r := setupRegistration(t)
	next, cmd := r.Update(usernameTakenMsg{username: "alice"})
	if cmd != nil {
		t.Errorf("usernameTakenMsg returned non-nil cmd, want nil")
	}
	r2, ok := next.(RegistrationModel)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if r2.step != stepInviteCode {
		t.Errorf("step = %v, want stepInviteCode", r2.step)
	}
	if r2.username != "alice" {
		t.Errorf("username = %q, want alice", r2.username)
	}
	if r2.input.Placeholder != "invite code" {
		t.Errorf("placeholder = %q, want 'invite code'", r2.input.Placeholder)
	}
}

func TestRegistration_Update_ErrMsg_StoresError(t *testing.T) {
	r := setupRegistration(t)
	next, _ := r.Update(screens.ErrMsg{Err: errors.New("bad")})
	r2, ok := next.(RegistrationModel)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if r2.err != "bad" {
		t.Errorf("err = %q, want 'bad'", r2.err)
	}
}

func TestRegistration_Submit_DispatchesByStep(t *testing.T) {
	r := setupRegistration(t)
	r.step = stepInviteCode
	r.username = "alice"
	r.input.SetValue("X")
	cmd := r.submit()
	if cmd == nil {
		t.Fatal("submit() returned nil cmd")
	}
}
