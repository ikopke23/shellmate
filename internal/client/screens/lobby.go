package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/shared"
)

var lobbyKeybinds = []string{
	"n:new",
	"enter:join",
	"s:spectate",
	"h:history",
	"i:import",
	"m:imported",
	"l:leaderboard",
	"q:quit",
}

var (
	lobbyTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	lobbyCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	lobbyHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// LobbyModel shows active games and connected players.
type LobbyModel struct {
	players  []shared.PlayerInfo
	games    []shared.GameInfo
	cursor   int
	username string
	conn     *websocket.Conn
	err      string
}

// NewLobbyModel creates a new lobby screen.
func NewLobbyModel(username string, conn *websocket.Conn) *LobbyModel {
	return &LobbyModel{
		username: username,
		conn:     conn,
	}
}

// SetState updates players and games from a LobbyState message.
func (m *LobbyModel) SetState(state shared.LobbyState) {
	m.players = state.Players
	m.games = state.Games
	if m.cursor >= len(m.games) && len(m.games) > 0 {
		m.cursor = len(m.games) - 1
	}
}

// Init implements tea.Model.
func (m *LobbyModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *LobbyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ErrMsg:
		m.err = msg.Err.Error()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "n":
			return m, m.createGame()
		case "j", "down":
			if m.cursor < len(m.games)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			return m, m.joinGame()
		case "s":
			return m, m.spectateGame()
		case "l":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLeaderboard} }
		case "h":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenHistory} }
		case "i":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenImport} }
		case "m":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenImportedGames} }
		}
	}
	return m, nil
}

func (m *LobbyModel) createGame() tea.Cmd {
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgCreateGame, shared.CreateGame{})
		if err != nil {
			return ErrMsg{Err: err}
		}
		if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

func (m *LobbyModel) joinGame() tea.Cmd {
	if len(m.games) == 0 {
		return nil
	}
	gameID := m.games[m.cursor].ID
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgJoinGame, shared.JoinGame{GameID: gameID})
		if err != nil {
			return ErrMsg{Err: err}
		}
		if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

func (m *LobbyModel) spectateGame() tea.Cmd {
	if len(m.games) == 0 {
		return nil
	}
	gameID := m.games[m.cursor].ID
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgSpectateGame, shared.SpectateGame{GameID: gameID})
		if err != nil {
			return ErrMsg{Err: err}
		}
		if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

// View implements tea.Model.
func (m *LobbyModel) View() string {
	var sb strings.Builder
	sb.WriteString(lobbyTitleStyle.Render(fmt.Sprintf("Lobby - %s", m.username)))
	sb.WriteString("\n\n")
	// Players
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Players Online"))
	sb.WriteString("\n")
	if len(m.players) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, p := range m.players {
		status := " "
		if p.Online {
			status = "*"
		}
		sb.WriteString(fmt.Sprintf("  %s %-16s %d\n", status, p.Username, p.Elo))
	}
	sb.WriteString("\n")
	// Games
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Active Games"))
	sb.WriteString("\n")
	if len(m.games) == 0 {
		sb.WriteString("  (no games)\n")
	}
	for i, g := range m.games {
		cursor := "  "
		if i == m.cursor {
			cursor = lobbyCursorStyle.Render("> ")
		}
		black := g.Black
		if black == "" {
			black = "(waiting)"
		}
		sb.WriteString(fmt.Sprintf("%s%-12s vs %-12s  moves:%d  spectators:%d\n",
			cursor, g.White, black, g.Moves, g.Spectators))
	}
	sb.WriteString("\n")
	if m.err != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.err))
		sb.WriteString("\n")
	}
	sb.WriteString(lobbyHelpStyle.Render(strings.Join(lobbyKeybinds, "  ")))
	sb.WriteString("\n")
	return sb.String()
}
