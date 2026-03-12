package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/shared"
)

var (
	lbTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	lbHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// LeaderboardModel shows ranked leaderboard of all players.
type LeaderboardModel struct {
	players []shared.PlayerInfo
	conn    *websocket.Conn
}

// NewLeaderboardModel creates a new leaderboard screen.
func NewLeaderboardModel(conn *websocket.Conn) *LeaderboardModel {
	return &LeaderboardModel{conn: conn}
}

// SetPlayers sets the leaderboard data.
func (m *LeaderboardModel) SetPlayers(players []shared.PlayerInfo) {
	m.players = players
}

// Init implements tea.Model.
func (m *LeaderboardModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *LeaderboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: screenLobby} }
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *LeaderboardModel) View() string {
	var sb strings.Builder
	sb.WriteString(lbTitleStyle.Render("Leaderboard"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %-4s %-20s %s\n", "#", "Player", "Elo"))
	sb.WriteString(fmt.Sprintf("  %-4s %-20s %s\n", "---", "--------------------", "----"))
	for i, p := range m.players {
		sb.WriteString(fmt.Sprintf("  %-4d %-20s %d\n", i+1, p.Username, p.Elo))
	}
	if len(m.players) == 0 {
		sb.WriteString("  No players yet.\n")
	}
	sb.WriteString("\n")
	sb.WriteString(lbHelpStyle.Render("q/esc:back"))
	sb.WriteString("\n")
	return sb.String()
}
