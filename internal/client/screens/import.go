package screens

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/shared"
)

var (
	importTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	importHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	importModeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	importErrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
)

// ImportPGNData carries a PGN record from the import screen to the replay screen.
// Exported so model.go (package client) can type-switch on it.
type ImportPGNData struct {
	Record shared.HistoryRecord
}

// ImportModel is the PGN import screen.
type ImportModel struct {
	textarea textarea.Model
	err      string
}

// NewImportModel creates a new PGN import screen.
func NewImportModel() *ImportModel {
	ta := textarea.New()
	ta.Placeholder = "Paste PGN or enter file path..."
	ta.Focus()
	ta.SetWidth(60)
	ta.SetHeight(12)
	return &ImportModel{textarea: ta}
}

func detectImportMode(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "pgn text"
	}
	first := strings.Fields(trimmed)[0]
	if strings.HasPrefix(first, "/") || strings.HasPrefix(first, "~/") || strings.HasPrefix(first, "./") {
		return "file path"
	}
	return "pgn text"
}

// Init implements tea.Model.
func (m *ImportModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (m *ImportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+s":
			return m.submit()
		case "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLobby} }
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *ImportModel) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.textarea.Value())
	if text == "" {
		m.err = "nothing to import"
		return m, nil
	}
	var pgn string
	if detectImportMode(text) == "file path" {
		path := text
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = home + path[1:]
		}
		data, err := os.ReadFile(path) //nolint:gosec // path is user-supplied by design: this screen lets the user pick a local PGN to import
		if err != nil {
			m.err = "could not read file: " + err.Error()
			return m, nil
		}
		pgn = string(data)
	} else {
		pgn = text
	}
	rec := shared.HistoryRecord{PGN: pgn}
	return m, func() tea.Msg {
		return ScreenChangeMsg{Screen: ScreenReplay, Data: ImportPGNData{Record: rec}}
	}
}

// View implements tea.Model.
func (m *ImportModel) View() string {
	var sb strings.Builder
	sb.WriteString(importTitleStyle.Render("Import PGN"))
	sb.WriteString("\n\n")
	sb.WriteString(importModeStyle.Render("mode: " + detectImportMode(m.textarea.Value())))
	sb.WriteString("\n")
	sb.WriteString(m.textarea.View())
	sb.WriteString("\n")
	if m.err != "" {
		sb.WriteString(importErrStyle.Render(m.err))
		sb.WriteString("\n")
	}
	sb.WriteString(importHelpStyle.Render("ctrl+s:import  esc:back  enter:newline"))
	sb.WriteString("\n")
	return sb.String()
}
