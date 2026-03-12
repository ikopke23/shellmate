package screens

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/shared"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	focusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	noStyle     = lipgloss.NewStyle()
	submitStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
)

// LoginModel is the login screen with username and invite code inputs.
type LoginModel struct {
	usernameInput textinput.Model
	inviteInput   textinput.Model
	focused       int // 0 = username, 1 = invite code
	err           string
	serverAddr    string
	connecting    bool
}

// NewLoginModel creates a new login screen.
func NewLoginModel(serverAddr string) *LoginModel {
	ui := textinput.New()
	ui.Placeholder = "Username"
	ui.Focus()
	ui.CharLimit = 32
	ui.Width = 30
	ui.PromptStyle = focusStyle
	ui.TextStyle = focusStyle
	ii := textinput.New()
	ii.Placeholder = "Invite Code"
	ii.CharLimit = 64
	ii.Width = 30
	return &LoginModel{
		usernameInput: ui,
		inviteInput:   ii,
		serverAddr:    serverAddr,
	}
}

// Init implements tea.Model.
func (m *LoginModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m *LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.err = ""
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab":
			m.toggleFocus()
			return m, nil
		case "enter":
			if m.focused == 0 {
				m.toggleFocus()
				return m, nil
			}
			return m, m.connectCmd()
		}
	case ErrMsg:
		m.connecting = false
		m.err = msg.Err.Error()
		return m, nil
	}
	var cmd tea.Cmd
	if m.focused == 0 {
		m.usernameInput, cmd = m.usernameInput.Update(msg)
	} else {
		m.inviteInput, cmd = m.inviteInput.Update(msg)
	}
	return m, cmd
}

func (m *LoginModel) toggleFocus() {
	if m.focused == 0 {
		m.focused = 1
		m.usernameInput.Blur()
		m.inviteInput.Focus()
		m.usernameInput.PromptStyle = noStyle
		m.usernameInput.TextStyle = noStyle
		m.inviteInput.PromptStyle = focusStyle
		m.inviteInput.TextStyle = focusStyle
	} else {
		m.focused = 0
		m.inviteInput.Blur()
		m.usernameInput.Focus()
		m.inviteInput.PromptStyle = noStyle
		m.inviteInput.TextStyle = noStyle
		m.usernameInput.PromptStyle = focusStyle
		m.usernameInput.TextStyle = focusStyle
	}
}

func (m *LoginModel) connectCmd() tea.Cmd {
	username := strings.TrimSpace(m.usernameInput.Value())
	invite := strings.TrimSpace(m.inviteInput.Value())
	if username == "" {
		m.err = "username is required"
		return nil
	}
	if invite == "" {
		m.err = "invite code is required"
		return nil
	}
	m.connecting = true
	addr := m.serverAddr
	return func() tea.Msg {
		u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("connection failed: %w", err)}
		}
		data, err := shared.Encode(shared.MsgJoinLobby, shared.JoinLobby{
			InviteCode: invite,
			Username:   username,
		})
		if err != nil {
			conn.Close()
			return ErrMsg{Err: fmt.Errorf("encode error: %w", err)}
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			return ErrMsg{Err: fmt.Errorf("send error: %w", err)}
		}
		return ConnectedMsg{Conn: conn, Username: username}
	}
}

// View implements tea.Model.
func (m *LoginModel) View() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Shellmate Chess"))
	sb.WriteString("\n\n")
	sb.WriteString(m.usernameInput.View())
	sb.WriteString("\n")
	sb.WriteString(m.inviteInput.View())
	sb.WriteString("\n\n")
	if m.connecting {
		sb.WriteString(submitStyle.Render("Connecting..."))
	} else {
		sb.WriteString(submitStyle.Render("Press Enter to connect"))
	}
	if m.err != "" {
		sb.WriteString("\n")
		sb.WriteString(errStyle.Render(m.err))
	}
	sb.WriteString("\n")
	return sb.String()
}
