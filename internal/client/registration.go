package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/server"
	"github.com/ikopke/shellmate/internal/client/screens"
)

// registrationStep tracks which input field is active.
type registrationStep int

const (
	stepUsername   registrationStep = iota
	stepInviteCode
)

// RegistrationModel handles first-time SSH key registration.
type RegistrationModel struct {
	hub         *server.Hub
	fingerprint string
	input       textinput.Model
	step        registrationStep
	username    string // saved after step 1 confirms name is taken
	err         string
	width       int
	height      int
}

// NewRegistrationModel creates the registration model for unknown SSH keys.
func NewRegistrationModel(hub *server.Hub, fingerprint string, w, h int) RegistrationModel {
	ti := textinput.New()
	ti.Placeholder = "username"
	ti.Focus()
	return RegistrationModel{hub: hub, fingerprint: fingerprint, input: ti, width: w, height: h}
}

// Init implements tea.Model.
func (r RegistrationModel) Init() tea.Cmd { return textinput.Blink }

// Update implements tea.Model.
func (r RegistrationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return r, tea.Quit
		case "enter":
			return r, r.submit()
		}
	case registeredMsg:
		c := msg.hub.Register(msg.user.Username)
		return NewModel(msg.hub, c, msg.user, r.width, r.height), func() tea.Msg {
			msg.hub.BroadcastLobby(context.Background())
			return nil
		}
	case usernameTakenMsg:
		r.step = stepInviteCode
		r.username = msg.username
		r.input.Reset()
		r.input.Placeholder = "invite code"
		r.err = ""
		return r, nil
	case screens.ErrMsg:
		r.err = msg.Err.Error()
		return r, nil
	}
	var cmd tea.Cmd
	r.input, cmd = r.input.Update(msg)
	return r, cmd
}

func (r RegistrationModel) submit() tea.Cmd {
	val := strings.TrimSpace(r.input.Value())
	if val == "" {
		return nil
	}
	switch r.step {
	case stepUsername:
		return r.submitUsername(val)
	case stepInviteCode:
		return r.submitInviteCode(val)
	}
	return nil
}

func (r *RegistrationModel) submitUsername(username string) tea.Cmd {
	hub := r.hub
	fp := r.fingerprint
	return func() tea.Msg {
		taken, err := hub.CheckUsername(context.Background(), username)
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		if !taken {
			user, err := hub.RegisterUser(context.Background(), username, fp)
			if err != nil {
				return screens.ErrMsg{Err: err}
			}
			return registeredMsg{hub: hub, user: user}
		}
		return usernameTakenMsg{username: username}
	}
}

func (r *RegistrationModel) submitInviteCode(code string) tea.Cmd {
	hub := r.hub
	fp := r.fingerprint
	username := r.username
	return func() tea.Msg {
		if code != hub.InviteCode() {
			return screens.ErrMsg{Err: fmt.Errorf("invalid invite code")}
		}
		if err := hub.LinkKey(context.Background(), username, fp); err != nil {
			return screens.ErrMsg{Err: err}
		}
		u, err := hub.GetUser(context.Background(), username)
		if err != nil || u == nil {
			return screens.ErrMsg{Err: fmt.Errorf("user lookup failed")}
		}
		return registeredMsg{hub: hub, user: u}
	}
}

type registeredMsg struct {
	hub  *server.Hub
	user *server.User
}

type usernameTakenMsg struct{ username string }

// View implements tea.Model.
func (r RegistrationModel) View() string {
	var sb strings.Builder
	switch r.step {
	case stepUsername:
		sb.WriteString("Welcome to Shellmate!\n\nEnter a username:\n")
	case stepInviteCode:
		sb.WriteString("That username is taken.\nEnter invite code to link your device:\n")
	}
	sb.WriteString(r.input.View())
	sb.WriteString("\n")
	if r.err != "" {
		sb.WriteString("\nError: " + r.err + "\n")
	}
	return sb.String()
}
