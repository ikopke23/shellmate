package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/notnil/chess"
)

var (
	replayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	replayHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	replayStepStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
)

// ReplayModel provides step-through replay of a past game.
type ReplayModel struct {
	pgn       string
	white     string
	black     string
	playedAt  time.Time
	game      *chess.Game
	moves     []*chess.Move
	positions []*chess.Position
	sanMoves  []string
	stepIdx   int // current step (0 = start, len(moves) = end)
	board     *render.Board
	moveList  *render.MoveList
	exportMsg string
	err       string
}

// NewReplayModel creates an empty replay screen.
func NewReplayModel() *ReplayModel {
	return &ReplayModel{
		board:    render.NewBoard(chess.NewGame().Position(), false),
		moveList: render.NewMoveList(20),
	}
}

// LoadPGN parses a PGN string and sets up the replay.
func (m *ReplayModel) LoadPGN(pgn string) error {
	m.pgn = pgn
	reader := strings.NewReader(pgn)
	pgnFn, err := chess.PGN(reader)
	if err != nil {
		return err
	}
	g := chess.NewGame(pgnFn)
	m.game = g
	m.moves = g.Moves()
	m.positions = g.Positions()
	notation := chess.AlgebraicNotation{}
	m.sanMoves = make([]string, len(m.moves))
	for i, mv := range m.moves {
		m.sanMoves[i] = notation.Encode(m.positions[i], mv)
	}
	m.stepIdx = 0
	m.updateView()
	return nil
}

// SetMeta stores player names and date for export.
func (m *ReplayModel) SetMeta(white, black string, playedAt time.Time) {
	m.white = white
	m.black = black
	m.playedAt = playedAt
}

func (m *ReplayModel) updateView() {
	if m.stepIdx == 0 {
		m.board.SetPosition(m.positions[0], 0, 0)
		m.board.ClearHighlight()
		m.moveList.SetMoves(m.sanMoves, -1)
	} else {
		mv := m.moves[m.stepIdx-1]
		m.board.SetPosition(m.positions[m.stepIdx], mv.S1(), mv.S2())
		m.moveList.SetMoves(m.sanMoves, m.stepIdx-1)
	}
}

// Init implements tea.Model.
func (m *ReplayModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *ReplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ErrMsg:
		m.err = msg.Err.Error()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenHistory} }
		case "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			if m.stepIdx > 0 {
				m.stepIdx--
				m.updateView()
			}
		case "right", "l":
			if m.stepIdx < len(m.moves) {
				m.stepIdx++
				m.updateView()
			}
		case "e":
			path, err := exportPGN(m.white, m.black, m.playedAt, m.pgn)
			if err != nil {
				m.exportMsg = fmt.Sprintf("export error: %s", err)
			} else {
				m.exportMsg = fmt.Sprintf("exported: %s", path)
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *ReplayModel) View() string {
	var sb strings.Builder
	sb.WriteString(replayTitleStyle.Render("Replay"))
	sb.WriteString("\n\n")
	boardView := m.board.View()
	moveView := m.moveList.View()
	left := boardView
	right := lipgloss.NewStyle().Bold(true).Render("Moves") + "\n" + moveView
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	sb.WriteString("\n")
	sb.WriteString(replayStepStyle.Render(
		strings.Repeat(" ", 3) + stepInfo(m.stepIdx, len(m.moves)),
	))
	sb.WriteString("\n")
	if m.exportMsg != "" {
		sb.WriteString(replayStepStyle.Render(m.exportMsg))
		sb.WriteString("\n")
	}
	if m.err != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.err))
		sb.WriteString("\n")
	}
	sb.WriteString(replayHelpStyle.Render("left/h:back  right/l:forward  e:export  q/esc:back"))
	sb.WriteString("\n")
	return sb.String()
}

func stepInfo(current, total int) string {
	if total == 0 {
		return "No moves"
	}
	return fmt.Sprintf("Move %d/%d", current, total)
}
