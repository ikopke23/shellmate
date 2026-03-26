package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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
	// navigation
	backScreen ScreenID
	moveListX  int // X start of move list on screen, computed in View()
	moveListY  int // Y of first move row, computed in View()
	// branch mode
	branchMode      bool
	branchPointIdx  int               // stepIdx when branch was entered
	branchGame      *chess.Game       // game state at branch tip
	branchSAN       []string          // SAN moves played in branch
	branchPositions []*chess.Position // positions during branch (index 0 = position at branchPointIdx)
	branchStepIdx   int               // view cursor within combined original+branch moves
	input           *LocalMoveInput   // active during branch mode
	// save prompt
	savePromptActive bool
	saveStep         int // 0=white, 1=black, 2=confirm unknown
	saveWhiteInput   textinput.Model
	saveBlackInput   textinput.Model
	saveUnknownNames []string
	saveConfirmIdx   int
	saveMsg          string
}

// NewReplayModel creates an empty replay screen.
func NewReplayModel() *ReplayModel {
	return &ReplayModel{
		board:      render.NewBoard(chess.NewGame().Position(), false),
		moveList:   render.NewMoveList(20),
		backScreen: ScreenHistory,
	}
}

// SetBackScreen sets the screen to return to when the user exits replay.
func (m *ReplayModel) SetBackScreen(s ScreenID) {
	m.backScreen = s
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

// enterBranch starts branch mode from the current stepIdx.
func (m *ReplayModel) enterBranch() {
	bg := chess.NewGame()
	for i := 0; i < m.stepIdx; i++ {
		if err := bg.MoveStr(m.sanMoves[i]); err != nil {
			break
		}
	}
	m.branchGame = bg
	m.branchPointIdx = m.stepIdx
	m.branchSAN = nil
	m.branchPositions = []*chess.Position{bg.Position()}
	m.branchStepIdx = m.stepIdx
	m.branchMode = true
	m.input = NewLocalMoveInput(false)
	m.updateBranchView()
}

func (m *ReplayModel) exitBranch() {
	m.branchMode = false
	m.branchGame = nil
	m.branchSAN = nil
	m.branchPositions = nil
	m.input = nil
	m.moveList.SetBranchPoint(-1)
	m.updateView()
}

func (m *ReplayModel) atBranchTip() bool {
	return m.branchStepIdx == m.branchPointIdx+len(m.branchSAN)
}

func (m *ReplayModel) applyBranchMove(san string) {
	if err := m.branchGame.MoveStr(san); err != nil {
		return
	}
	m.branchSAN = append(m.branchSAN, san)
	m.branchPositions = append(m.branchPositions, m.branchGame.Position())
	m.branchStepIdx++
	m.updateBranchView()
}

func (m *ReplayModel) branchStepLeft() {
	if m.branchStepIdx > 0 {
		m.branchStepIdx--
		m.updateBranchView()
	}
}

func (m *ReplayModel) branchStepRight() {
	maxIdx := m.branchPointIdx + len(m.branchSAN)
	if m.branchStepIdx < maxIdx {
		m.branchStepIdx++
		m.updateBranchView()
	}
}

func (m *ReplayModel) updateBranchView() {
	allSAN := make([]string, m.branchPointIdx, m.branchPointIdx+len(m.branchSAN))
	copy(allSAN, m.sanMoves[:m.branchPointIdx])
	allSAN = append(allSAN, m.branchSAN...)
	var pos *chess.Position
	if m.branchStepIdx <= m.branchPointIdx {
		if m.branchStepIdx < len(m.positions) {
			pos = m.positions[m.branchStepIdx]
		}
	} else {
		bIdx := m.branchStepIdx - m.branchPointIdx
		if bIdx < len(m.branchPositions) {
			pos = m.branchPositions[bIdx]
		}
	}
	if pos != nil {
		m.board.SetPosition(pos, 0, 0)
		m.board.ClearHighlight()
	}
	m.moveList.SetMoves(allSAN, m.branchStepIdx-1)
	m.moveList.SetBranchPoint(m.branchPointIdx)
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
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if m.branchMode && m.atBranchTip() {
				san, handled, cmd := m.input.HandleMsg(msg, m.board, m.branchGame)
				if san != "" {
					m.applyBranchMove(san)
					return m, cmd
				}
				if handled {
					return m, cmd
				}
			}
			// Move list click (works in both replay and branch mode)
			relY := msg.Y - m.moveListY
			if relY >= 0 && msg.X >= m.moveListX {
				leftSide := msg.X < m.moveListX+11
				if m.branchMode {
					maxIdx := m.branchPointIdx + len(m.branchSAN)
					idx := m.moveList.ClickMoveIdx(relY, leftSide)
					if idx >= 0 && idx < maxIdx {
						m.branchStepIdx = idx + 1
						m.updateBranchView()
					}
				} else {
					idx := m.moveList.ClickMoveIdx(relY, leftSide)
					if idx >= 0 && idx < len(m.moves) {
						m.stepIdx = idx + 1
						m.updateView()
					}
				}
			}
		}
		return m, nil
	case UsernameCheckDoneMsg:
		if len(msg.Unknown) == 0 {
			return m, m.doSave(false)
		}
		m.saveUnknownNames = msg.Unknown
		m.saveConfirmIdx = 0
		m.saveStep = 2
		return m, nil
	case SaveImportedDoneMsg:
		m.savePromptActive = false
		m.saveMsg = "game saved"
		return m, nil
	case tea.KeyMsg:
		if m.savePromptActive {
			return m.updateSavePrompt(msg)
		}
		if m.branchMode {
			switch msg.String() {
			case "esc":
				m.exitBranch()
				return m, nil
			case "s":
				m.savePromptActive = true
				m.saveStep = 0
				wi := textinput.New()
				wi.Placeholder = "White player name"
				wi.Focus()
				m.saveWhiteInput = wi
				bi := textinput.New()
				bi.Placeholder = "Black player name"
				m.saveBlackInput = bi
				return m, nil
			case "left", "h":
				m.branchStepLeft()
				return m, nil
			case "right", "l":
				m.branchStepRight()
				return m, nil
			}
			if m.atBranchTip() {
				san, handled, cmd := m.input.HandleMsg(msg, m.board, m.branchGame)
				if san != "" {
					m.applyBranchMove(san)
					return m, cmd
				}
				if handled {
					return m, cmd
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: m.backScreen} }
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
		case "[":
			rows := m.board.CellRows()
			if rows > 2 {
				m.board.SetCellSize((rows-1)*2, rows-1)
			}
		case "]":
			rows := m.board.CellRows()
			if rows < 8 {
				m.board.SetCellSize((rows+1)*2, rows+1)
			}
		case "b":
			if m.game != nil {
				m.enterBranch()
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
	// Board: 2 left-pad cols + 8*cellCols board cols; "  " 2-col separator; then move list.
	// Title on line 0, blank on line 1, JoinHorizontal starts at line 2.
	// "Moves" header is the first line of the right side (line 2), move rows start at line 3.
	m.moveListX = 2 + 8*m.board.CellCols() + 2
	m.moveListY = 3
	if m.branchMode {
		sb.WriteString(replayStepStyle.Render("   [BRANCH] " + stepInfo(m.branchStepIdx, m.branchPointIdx+len(m.branchSAN))))
		sb.WriteString("\n")
		if m.atBranchTip() {
			inputY := 2 + m.board.CellRows()*8 + 2
			m.input.SetPromoPopupY(inputY)
			sb.WriteString(m.input.View(m.board, m.branchGame))
		} else {
			sb.WriteString(replayStepStyle.Render("   (navigate to tip to play)"))
			sb.WriteString("\n")
		}
		sb.WriteString(replayHelpStyle.Render("left/h:back  right/l:fwd  s:save  esc:exit branch"))
	} else {
		sb.WriteString(replayStepStyle.Render(
			strings.Repeat(" ", 3) + stepInfo(m.stepIdx, len(m.moves)),
		))
		sb.WriteString("\n")
		if m.exportMsg != "" {
			sb.WriteString(replayStepStyle.Render(m.exportMsg))
			sb.WriteString("\n")
		}
		sb.WriteString(replayHelpStyle.Render("left/h:back  right/l:fwd  [:smaller  ]:larger  b:branch  e:export  q/esc:back"))
	}
	sb.WriteString("\n")
	if m.savePromptActive {
		switch m.saveStep {
		case 0:
			sb.WriteString(gameStatusStyle.Render("White player: "))
			sb.WriteString(m.saveWhiteInput.View())
			sb.WriteString("\n")
			sb.WriteString(replayHelpStyle.Render("enter:next  esc:cancel"))
		case 1:
			sb.WriteString(gameStatusStyle.Render("Black player: "))
			sb.WriteString(m.saveBlackInput.View())
			sb.WriteString("\n")
			sb.WriteString(replayHelpStyle.Render("enter:save  esc:back"))
		case 2:
			name := m.saveUnknownNames[m.saveConfirmIdx]
			sb.WriteString(importErrStyle.Render(fmt.Sprintf(
				"Warning: '%s' not found — will create new user. Typos won't appear in the right history. Confirm? (y/n)", name)))
		}
		sb.WriteString("\n")
	}
	if m.saveMsg != "" {
		sb.WriteString(gameStatusStyle.Render(m.saveMsg))
		sb.WriteString("\n")
	}
	if m.err != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.err))
		sb.WriteString("\n")
	}
	return sb.String()
}

func stepInfo(current, total int) string {
	if total == 0 {
		return "No moves"
	}
	return fmt.Sprintf("Move %d/%d", current, total)
}

func (m *ReplayModel) updateSavePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.saveStep {
	case 0:
		if msg.String() == "enter" && strings.TrimSpace(m.saveWhiteInput.Value()) != "" {
			m.saveStep = 1
			m.saveBlackInput.Focus()
			return m, nil
		}
		if msg.String() == "esc" {
			m.savePromptActive = false
			return m, nil
		}
		var cmd tea.Cmd
		m.saveWhiteInput, cmd = m.saveWhiteInput.Update(msg)
		return m, cmd
	case 1:
		if msg.String() == "enter" && strings.TrimSpace(m.saveBlackInput.Value()) != "" {
			return m, m.checkUsernames()
		}
		if msg.String() == "esc" {
			m.saveStep = 0
			m.saveWhiteInput.Focus()
			return m, nil
		}
		var cmd tea.Cmd
		m.saveBlackInput, cmd = m.saveBlackInput.Update(msg)
		return m, cmd
	case 2:
		switch msg.String() {
		case "y":
			if m.saveConfirmIdx < len(m.saveUnknownNames)-1 {
				m.saveConfirmIdx++
				return m, nil
			}
			return m, m.doSave(true)
		case "n":
			m.saveStep = 1
			m.saveUnknownNames = nil
			m.saveBlackInput.Focus()
			return m, nil
		}
	}
	return m, nil
}

func (m *ReplayModel) checkUsernames() tea.Cmd {
	white := strings.TrimSpace(m.saveWhiteInput.Value())
	black := strings.TrimSpace(m.saveBlackInput.Value())
	return func() tea.Msg { return CheckUsernamesActionMsg{White: white, Black: black} }
}

func (m *ReplayModel) doSave(forceCreate bool) tea.Cmd {
	white := strings.TrimSpace(m.saveWhiteInput.Value())
	black := strings.TrimSpace(m.saveBlackInput.Value())
	pgn := m.buildBranchPGN()
	return func() tea.Msg { return SaveImportedActionMsg{White: white, Black: black, PGN: pgn, ForceCreate: forceCreate} }
}

func (m *ReplayModel) buildBranchPGN() string {
	allSAN := make([]string, m.branchPointIdx, m.branchPointIdx+len(m.branchSAN))
	copy(allSAN, m.sanMoves[:m.branchPointIdx])
	allSAN = append(allSAN, m.branchSAN...)
	g := chess.NewGame()
	for _, san := range allSAN {
		if err := g.MoveStr(san); err != nil {
			break
		}
	}
	return g.String()
}
