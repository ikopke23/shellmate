package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/shared"
)

var (
	importedGamesTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	importedGamesCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	importedGamesHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// ImportedGamesOpenData carries a record from the imported games list to the replay screen.
// Exported so model.go (package client) can type-switch on it.
type ImportedGamesOpenData struct {
	Record shared.HistoryRecord
}

// ImportedGamesModel lists all imported games across all users.
type ImportedGamesModel struct {
	games  []shared.HistoryRecord
	cursor int
	err    string
}

// NewImportedGamesModel creates a new imported-games list screen.
func NewImportedGamesModel() *ImportedGamesModel {
	return &ImportedGamesModel{}
}

// SetGames replaces the list of imported games displayed on screen.
func (m *ImportedGamesModel) SetGames(games []shared.HistoryRecord) {
	m.games = games
	if m.cursor >= len(m.games) && len(m.games) > 0 {
		m.cursor = len(m.games) - 1
	}
}

// Init implements tea.Model.
func (m *ImportedGamesModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *ImportedGamesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ErrMsg:
		m.err = msg.Err.Error()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLobby} }
		case "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.games)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.games) > 0 {
				g := m.games[m.cursor]
				return m, func() tea.Msg {
					return ScreenChangeMsg{Screen: ScreenReplay, Data: ImportedGamesOpenData{Record: g}}
				}
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *ImportedGamesModel) View() string {
	var sb strings.Builder
	sb.WriteString(importedGamesTitleStyle.Render("Imported Games"))
	sb.WriteString("\n\n")
	if len(m.games) == 0 {
		sb.WriteString("  No imported games yet.\n")
	}
	for i := range m.games {
		g := &m.games[i]
		cursor := "  "
		if i == m.cursor {
			cursor = importedGamesCursorStyle.Render("> ")
		}
		fmt.Fprintf(&sb, "%s%-12s vs %-12s  %s\n",
			cursor, g.White, g.Black, g.PlayedAt.Format("2006-01-02 15:04"))
	}
	sb.WriteString("\n")
	if m.err != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.err))
		sb.WriteString("\n")
	}
	sb.WriteString(importedGamesHelpStyle.Render("enter:view  j/k:navigate  q/esc:back"))
	sb.WriteString("\n")
	return sb.String()
}
