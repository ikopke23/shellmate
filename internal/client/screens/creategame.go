package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/shared"
)

// CreateGameMsg is emitted when the player confirms a time control selection.
type CreateGameMsg struct {
	TimeControl shared.TimeControl
}

type preset struct {
	label          string
	initialSeconds int
	incSeconds     int
}

var gamePresets = []preset{
	{"Untimed", 0, 0},
	{"Bullet  1+0", 60, 0},
	{"Bullet  1+1", 60, 1},
	{"Bullet  2+1", 120, 1},
	{"Blitz   3+2", 180, 2},
	{"Blitz   5+0", 300, 0},
	{"Rapid   10+0", 600, 0},
	{"Classical  30+0", 1800, 0},
	{"Custom...", -1, -1},
}

// CreateGameModel is the time control selection screen.
type CreateGameModel struct {
	cursor     int
	customMode bool
	inputs     [2]textinput.Model // [0]=minutes, [1]=increment
	focusIdx   int
	err        string
}

// NewCreateGameModel creates a new create game screen.
func NewCreateGameModel() *CreateGameModel {
	minInput := textinput.New()
	minInput.Placeholder = "5"
	minInput.CharLimit = 4
	minInput.Width = 6
	incInput := textinput.New()
	incInput.Placeholder = "0"
	incInput.CharLimit = 4
	incInput.Width = 6
	return &CreateGameModel{inputs: [2]textinput.Model{minInput, incInput}}
}

func (m *CreateGameModel) selectedTimeControl() shared.TimeControl {
	if m.customMode {
		mins, _ := strconv.Atoi(m.inputs[0].Value())
		inc, _ := strconv.Atoi(m.inputs[1].Value())
		if inc < 0 {
			inc = 0
		}
		return shared.TimeControl{InitialSeconds: mins * 60, IncrementSeconds: inc}
	}
	p := gamePresets[m.cursor]
	return shared.TimeControl{InitialSeconds: p.initialSeconds, IncrementSeconds: p.incSeconds}
}

// Init implements tea.Model.
func (m *CreateGameModel) Init() tea.Cmd { return nil }

func (m *CreateGameModel) updateCustomMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.customMode = false
		m.err = ""
		return m, nil
	case "tab", "shift+tab":
		m.focusIdx = 1 - m.focusIdx
		m.inputs[m.focusIdx].Focus()
		m.inputs[1-m.focusIdx].Blur()
		return m, nil
	case "enter":
		tc := m.selectedTimeControl()
		if tc.InitialSeconds <= 0 {
			m.err = "minutes must be > 0"
			return m, nil
		}
		return m, func() tea.Msg { return CreateGameMsg{TimeControl: tc} }
	}
	var cmds [2]tea.Cmd
	m.inputs[0], cmds[0] = m.inputs[0].Update(msg)
	m.inputs[1], cmds[1] = m.inputs[1].Update(msg)
	return m, tea.Batch(cmds[0], cmds[1])
}

// Update implements tea.Model.
func (m *CreateGameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.customMode {
			return m.updateCustomMode(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLobby} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(gamePresets)-1 {
				m.cursor++
			}
		case "c":
			m.customMode = true
			m.inputs[0].Focus()
			m.inputs[1].Blur()
			m.focusIdx = 0
		case "enter":
			p := gamePresets[m.cursor]
			if p.initialSeconds == -1 {
				m.customMode = true
				m.inputs[0].Focus()
				m.inputs[1].Blur()
				m.focusIdx = 0
				return m, nil
			}
			tc := shared.TimeControl{InitialSeconds: p.initialSeconds, IncrementSeconds: p.incSeconds}
			return m, func() tea.Msg { return CreateGameMsg{TimeControl: tc} }
		}
	}
	return m, nil
}

var (
	createTitleStyle    = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	createPresetStyle   = lipgloss.NewStyle().PaddingLeft(2)
	createSelectedStyle = lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	createHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	createErrStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
)

// View implements tea.Model.
func (m *CreateGameModel) View() string {
	var sb strings.Builder
	sb.WriteString(createTitleStyle.Render("Create Game"))
	sb.WriteString("\n\nSelect time control:\n\n")
	if m.customMode {
		sb.WriteString("  Minutes: " + m.inputs[0].View() + "\n")
		sb.WriteString("  Increment (sec): " + m.inputs[1].View() + "\n")
		if m.err != "" {
			sb.WriteString(createErrStyle.Render(m.err) + "\n")
		}
		sb.WriteString("\n" + createHelpStyle.Render("tab:next field  enter:create  esc:back"))
	} else {
		for i, p := range gamePresets {
			if i == m.cursor {
				sb.WriteString(createSelectedStyle.Render(fmt.Sprintf("> %s", p.label)))
			} else {
				sb.WriteString(createPresetStyle.Render(p.label))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n" + createHelpStyle.Render("↑↓:navigate  enter:create  c:custom  esc:back"))
	}
	sb.WriteString("\n")
	return sb.String()
}
