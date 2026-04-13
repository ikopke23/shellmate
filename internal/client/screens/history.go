package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/shared"
)

var historyExportStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC66"))

var (
	historyTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	historyCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	historyHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// HistoryModel shows past games for the logged-in user.
type HistoryModel struct {
	games        []shared.HistoryRecord
	cursor       int
	username     string
	exportMsg    string
	clipboardSeq string
	err          string
}

// NewHistoryModel creates a new history screen.
func NewHistoryModel(username string) *HistoryModel {
	return &HistoryModel{username: username}
}

// SetGames sets the game history records.
func (m *HistoryModel) SetGames(games []shared.HistoryRecord) {
	m.games = games
	if m.cursor >= len(m.games) && len(m.games) > 0 {
		m.cursor = len(m.games) - 1
	}
}

// Init implements tea.Model.
func (m *HistoryModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ErrMsg:
		m.err = msg.Err.Error()
		return m, nil
	case clearClipboardMsg:
		m.clipboardSeq = ""
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
					return ScreenChangeMsg{Screen: ScreenReplay, Data: g}
				}
			}
		case "e":
			if len(m.games) > 0 {
				g := m.games[m.cursor]
				osc, filename := pgnClipboardOSC(g.White, g.Black, g.PlayedAt, g.PGN)
				m.clipboardSeq = osc
				m.exportMsg = fmt.Sprintf("copied to clipboard: %s", filename)
				return m, func() tea.Msg { return clearClipboardMsg{} }
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *HistoryModel) View() string {
	var sb strings.Builder
	sb.WriteString(historyTitleStyle.Render(fmt.Sprintf("Game History - %s", m.username)))
	sb.WriteString("\n\n")
	if len(m.games) == 0 {
		sb.WriteString("  No games played yet.\n")
	}
	for i, g := range m.games {
		cursor := "  "
		if i == m.cursor {
			cursor = historyCursorStyle.Render("> ")
		}
		result := g.Result
		if g.Imported {
			result += " [imported]"
		}
		sb.WriteString(fmt.Sprintf("%s%-12s vs %-12s  %s  %s\n",
			cursor, g.White, g.Black, result, g.PlayedAt.Format("2006-01-02 15:04")))
	}
	sb.WriteString("\n")
	if m.exportMsg != "" {
		sb.WriteString(historyExportStyle.Render(m.exportMsg))
		sb.WriteString("\n")
	}
	if m.err != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.err))
		sb.WriteString("\n")
	}
	sb.WriteString(historyHelpStyle.Render("enter:replay  e:export  q/esc:back"))
	sb.WriteString("\n")
	return m.clipboardSeq + sb.String()
}
