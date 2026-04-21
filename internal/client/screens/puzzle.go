package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

type puzzleEngineResponseMsg struct{ uci string }

type puzzleState int

const (
	puzzleStateLoading puzzleState = iota
	puzzleStatePlaying
	puzzleStateSuccess
	puzzleStateFailure
	puzzleStateSolution
)

var (
	puzzleTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	puzzleHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	puzzleDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	puzzleGoodStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CC66"))
	puzzleBadStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
)

// PuzzleAttemptMsg carries the result of a POST /puzzle/attempt call.
type PuzzleAttemptMsg struct {
	NewRating int
	Err       error
}

// puzzleViewPos holds a board position and the last-move squares for display.
type puzzleViewPos struct {
	pos  *chess.Position
	from chess.Square
	to   chess.Square
}

// PuzzleModel is the puzzle mode screen.
type PuzzleModel struct {
	username         string
	state            puzzleState
	record           *shared.PuzzleRecord
	game             *chess.Game
	initialFEN       string
	solution         []string // UCI move list; player plays [0] first
	solutionIdx      int      // next expected move index (starts at 0)
	board            *render.Board
	moveList         *render.MoveList
	input            *LocalMoveInput
	enginePending    bool
	contextHistory   []puzzleViewPos // pre-puzzle game positions; [0] = game start
	viewIdx          int             // position being viewed; totalViewPositions() = live end
	userPuzzleRating int
	lastDelta        int
	hasDelta         bool
	submitted        bool
	err              string
}

// NewPuzzleModel creates a puzzle screen in loading state.
func NewPuzzleModel(username string) *PuzzleModel {
	return &PuzzleModel{
		username: username,
		state:    puzzleStateLoading,
		board:    render.NewBoard(chess.NewGame().Position(), false),
		moveList: render.NewMoveList(14),
	}
}

// SetPuzzle initializes the puzzle state from a loaded record.
// Called by the root model when the fetch Cmd completes.
func (m *PuzzleModel) SetPuzzle(record shared.PuzzleRecord) {
	m.record = &record
	m.userPuzzleRating = record.UserPuzzleRating
	m.solution = strings.Fields(record.Moves)
	m.initialFEN = record.FEN
	m.hasDelta = false
	m.submitted = false
	m.err = ""
	m.initGame()
}

// initGame creates a chess.Game from the stored FEN. The FEN already encodes
// the position after the opponent's last move, so solution[0] is the player's
// first move — no setup move is applied here.
func (m *PuzzleModel) initGame() {
	fenOpt, err := chess.FEN(m.initialFEN)
	if err != nil {
		m.err = fmt.Sprintf("invalid FEN: %v", err)
		return
	}
	g := chess.NewGame(fenOpt)
	flipped := g.Position().Turn() == chess.Black
	m.board.SetFlipped(flipped)
	m.board.SetPosition(g.Position(), 0, 0)
	m.board.ClearHighlight()
	m.game = g
	m.solutionIdx = 0
	m.enginePending = false
	m.input = NewLocalMoveInput(flipped)
	m.submitted = false
	m.state = puzzleStatePlaying
	m.buildContextHistory()
	m.viewIdx = m.totalViewPositions()
	m.updateMoveList()
}

// retry resets the puzzle to its initial playing state without recording an attempt.
func (m *PuzzleModel) retry() {
	m.initGame()
}

// showSolution resets the game to the puzzle FEN, applies all solution moves, and sets
// viewIdx to the puzzle start so the user can step through with arrow keys.
func (m *PuzzleModel) showSolution() {
	fenOpt, err := chess.FEN(m.initialFEN)
	if err != nil {
		m.err = fmt.Sprintf("invalid FEN: %v", err)
		return
	}
	g := chess.NewGame(fenOpt)
	m.game = g
	uciN := chess.UCINotation{}
	for _, uci := range m.solution {
		pos := m.game.Position()
		mv, err := uciN.Decode(pos, uci)
		if err != nil {
			break
		}
		if err := m.game.Move(mv); err != nil {
			break
		}
	}
	ctxMoves := len(m.contextHistory) - 1
	m.viewIdx = ctxMoves
	m.syncBoardToView()
	m.updateMoveList()
	m.state = puzzleStateSolution
}

// updateMoveList syncs the move list widget from the current game state.
// Context moves (pre-puzzle game history) are shown dimmed; puzzle moves are highlighted.
func (m *PuzzleModel) updateMoveList() {
	if m.game == nil || m.moveList == nil {
		return
	}
	var contextSANs []string
	if m.record != nil && m.record.ContextMoves != "" {
		contextSANs = strings.Fields(m.record.ContextMoves)
	}
	moves := m.game.Moves()
	positions := m.game.Positions()
	notation := chess.AlgebraicNotation{}
	puzzleSANs := make([]string, len(moves))
	for i, mv := range moves {
		puzzleSANs[i] = notation.Encode(positions[i], mv)
	}
	all := make([]string, 0, len(contextSANs)+len(puzzleSANs))
	all = append(all, contextSANs...)
	all = append(all, puzzleSANs...)
	// highlight the move that was applied to reach viewIdx (i.e. the one before it)
	currentMoveIdx := m.viewIdx - 1
	if len(all) == 0 {
		currentMoveIdx = -1
	}
	m.moveList.SetMoves(all, currentMoveIdx)
	m.moveList.SetBranchPoint(len(contextSANs))
}

// buildContextHistory replays record.ContextMoves from the standard starting position
// and stores each position with its last-move squares for board display during navigation.
func (m *PuzzleModel) buildContextHistory() {
	m.contextHistory = []puzzleViewPos{{pos: chess.NewGame().Position()}}
	if m.record == nil || m.record.ContextMoves == "" {
		return
	}
	g := chess.NewGame()
	notation := chess.AlgebraicNotation{}
	for _, san := range strings.Fields(m.record.ContextMoves) {
		pos := g.Position()
		for _, mv := range g.ValidMoves() {
			if notation.Encode(pos, mv) == san {
				if err := g.Move(mv); err == nil {
					m.contextHistory = append(m.contextHistory, puzzleViewPos{
						pos:  g.Position(),
						from: mv.S1(),
						to:   mv.S2(),
					})
				}
				break
			}
		}
	}
}

// totalViewPositions returns the viewIdx corresponding to the live (latest) position.
func (m *PuzzleModel) totalViewPositions() int {
	ctxMoves := len(m.contextHistory) - 1
	if m.game == nil {
		return ctxMoves
	}
	return ctxMoves + len(m.game.Moves())
}

// syncBoardToView updates the board display to the position at m.viewIdx.
func (m *PuzzleModel) syncBoardToView() {
	if m.game == nil {
		return
	}
	ctxMoves := len(m.contextHistory) - 1
	if m.viewIdx <= ctxMoves {
		h := m.contextHistory[m.viewIdx]
		m.board.SetPosition(h.pos, h.from, h.to)
		if m.viewIdx == 0 {
			m.board.ClearHighlight()
		}
	} else {
		puzzleIdx := m.viewIdx - ctxMoves
		positions := m.game.Positions()
		moves := m.game.Moves()
		if puzzleIdx < len(positions) && puzzleIdx-1 < len(moves) {
			mv := moves[puzzleIdx-1]
			m.board.SetPosition(positions[puzzleIdx], mv.S1(), mv.S2())
		}
	}
	m.updateMoveList()
}

// validateAndApply checks userSAN against the expected solution move.
// If correct, applies the player's move and returns (true, engineUCI).
// engineUCI is non-empty when the engine has a response queued.
// Returns (false, "") on incorrect move.
func (m *PuzzleModel) validateAndApply(userSAN string) (bool, string) {
	if m.solutionIdx >= len(m.solution) || m.game == nil {
		return false, ""
	}
	expectedUCI := m.solution[m.solutionIdx]
	algN := chess.AlgebraicNotation{}
	uciN := chess.UCINotation{}
	pos := m.game.Position()
	var matchedMove *chess.Move
	for _, mv := range m.game.ValidMoves() {
		if algN.Encode(pos, mv) != userSAN {
			continue
		}
		if uciN.Encode(pos, mv) != expectedUCI {
			m.state = puzzleStateFailure
			return false, ""
		}
		matchedMove = mv
		break
	}
	if matchedMove == nil {
		m.state = puzzleStateFailure
		return false, ""
	}
	if err := m.game.Move(matchedMove); err != nil {
		m.state = puzzleStateFailure
		return false, ""
	}
	m.board.SetPosition(m.game.Position(), matchedMove.S1(), matchedMove.S2())
	m.solutionIdx++
	m.viewIdx = m.totalViewPositions()
	m.updateMoveList()
	if m.solutionIdx < len(m.solution) {
		return true, m.solution[m.solutionIdx]
	}
	m.state = puzzleStateSuccess
	return true, ""
}

// applyEngineResponse applies the engine's queued response move.
func (m *PuzzleModel) applyEngineResponse(uci string) {
	if m.game == nil {
		return
	}
	defer func() { m.enginePending = false }()
	pos := m.game.Position()
	mv, err := chess.UCINotation{}.Decode(pos, uci)
	if err != nil {
		m.state = puzzleStateFailure
		return
	}
	if err := m.game.Move(mv); err != nil {
		m.state = puzzleStateFailure
		return
	}
	m.board.SetPosition(m.game.Position(), mv.S1(), mv.S2())
	m.solutionIdx++
	m.viewIdx = m.totalViewPositions()
	m.updateMoveList()
	if m.solutionIdx >= len(m.solution) {
		m.state = puzzleStateSuccess
	}
}

// handleMoveInput processes a completed SAN move from the input widget.
func (m *PuzzleModel) handleMoveInput(san string, inputCmd tea.Cmd) (tea.Model, tea.Cmd) {
	correct, engineUCI := m.validateAndApply(san)
	if !correct {
		return m, tea.Batch(inputCmd, m.submitAttempt(false))
	}
	if m.state == puzzleStateSuccess {
		return m, tea.Batch(inputCmd, m.submitAttempt(true))
	}
	if engineUCI != "" {
		m.enginePending = true
		delay := tea.Tick(400*time.Millisecond, func(_ time.Time) tea.Msg {
			return puzzleEngineResponseMsg{uci: engineUCI}
		})
		return m, tea.Batch(inputCmd, delay)
	}
	return m, inputCmd
}

// Init implements tea.Model.
func (m *PuzzleModel) Init() tea.Cmd {
	return m.initCmd()
}

// Update implements tea.Model.
func (m *PuzzleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case puzzleEngineResponseMsg:
		m.applyEngineResponse(msg.uci)
		if m.state == puzzleStateSuccess {
			cmd := m.submitAttempt(true)
			return m, cmd
		}
		return m, nil
	case PuzzleAttemptMsg:
		if msg.Err == nil {
			delta := msg.NewRating - m.userPuzzleRating
			m.userPuzzleRating = msg.NewRating
			m.lastDelta = delta
			m.hasDelta = true
		}
		return m, nil
	case tea.KeyMsg:
		return m.handlePuzzleKey(msg)
	case tea.MouseMsg:
		if m.state == puzzleStatePlaying && !m.enginePending && m.input != nil && m.viewIdx == m.totalViewPositions() {
			san, _, cmd := m.input.HandleMsg(msg, m.board, m.game)
			if san != "" {
				return m.handleMoveInput(san, cmd)
			}
			return m, cmd
		}
	case ErrMsg:
		m.err = msg.Err.Error()
	}
	return m, nil
}

func (m *PuzzleModel) resizeBoardSmaller() {
	rows := m.board.CellRows()
	if rows > 2 {
		m.board.SetCellSize((rows-1)*2, rows-1)
	}
}

func (m *PuzzleModel) resizeBoardLarger() {
	rows := m.board.CellRows()
	if rows < 8 {
		m.board.SetCellSize((rows+1)*2, rows+1)
	}
}

func (m *PuzzleModel) navigateBack() {
	if m.state != puzzleStateLoading && !m.enginePending && m.viewIdx > 0 {
		m.viewIdx--
		m.syncBoardToView()
	}
}

func (m *PuzzleModel) navigateForward() {
	if m.state != puzzleStateLoading && !m.enginePending {
		if m.viewIdx < m.totalViewPositions() {
			m.viewIdx++
			m.syncBoardToView()
		}
	}
}

func (m *PuzzleModel) handleRetryKey() tea.Cmd {
	if m.state == puzzleStateFailure || m.state == puzzleStateSuccess || m.state == puzzleStateSolution {
		m.retry()
	}
	return m.initCmd()
}

func (m *PuzzleModel) handleNKey() (tea.Model, tea.Cmd) {
	if m.state == puzzleStatePlaying || m.state == puzzleStateFailure {
		cmd := m.skipAndSubmit()
		return m, cmd
	}
	if m.state == puzzleStateSuccess || m.state == puzzleStateSolution {
		return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenPuzzle} }
	}
	return m, nil
}

func (m *PuzzleModel) handleTextMoveInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == puzzleStatePlaying && !m.enginePending && m.input != nil && m.viewIdx == m.totalViewPositions() {
		san, _, cmd := m.input.HandleMsg(msg, m.board, m.game)
		if san != "" {
			return m.handleMoveInput(san, cmd)
		}
		return m, cmd
	}
	return m, nil
}

func (m *PuzzleModel) handlePuzzleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLobby} }
	case "ctrl+c":
		return m, tea.Quit
	case "r":
		cmd := m.handleRetryKey()
		return m, cmd
	case "s":
		if m.state == puzzleStateFailure {
			m.showSolution()
		}
		return m, nil
	case "n":
		return m.handleNKey()
	case "[":
		m.resizeBoardSmaller()
		return m, nil
	case "]":
		m.resizeBoardLarger()
		return m, nil
	case "left":
		m.navigateBack()
		return m, nil
	case "right":
		m.navigateForward()
		return m, nil
	}
	return m.handleTextMoveInput(msg)
}

func (m *PuzzleModel) initCmd() tea.Cmd {
	if m.input != nil {
		return m.input.Init()
	}
	return nil
}

func (m *PuzzleModel) submitAttempt(solved bool) tea.Cmd {
	if m.record == nil || m.submitted {
		return nil
	}
	m.submitted = true
	id := m.record.ID
	return func() tea.Msg { return SubmitPuzzleAttemptMsg{PuzzleID: id, Solved: solved} }
}

func (m *PuzzleModel) skipAndSubmit() tea.Cmd {
	if m.record == nil || m.submitted {
		return func() tea.Msg { return ScreenChangeMsg{Screen: ScreenPuzzle} }
	}
	m.submitted = true
	id := m.record.ID
	return func() tea.Msg { return SubmitPuzzleAttemptMsg{PuzzleID: id, Skipped: true} }
}

// View implements tea.Model.
func (m *PuzzleModel) View() string {
	if m.state == puzzleStateLoading {
		title := puzzleTitleStyle.Render("Puzzle Mode") + "\n\n"
		if m.err != "" {
			return title + puzzleBadStyle.Render("Error: "+m.err) + "\n\n" + puzzleHelpStyle.Render("q:back") + "\n"
		}
		return title + "Loading puzzle...\n"
	}
	var sb strings.Builder
	sb.WriteString(puzzleTitleStyle.Render("Puzzle Mode"))
	sb.WriteString("\n\n")
	boardView := m.board.View()
	right := m.rightPanel()
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, boardView, "  ", right))
	sb.WriteString("\n")
	switch m.state {
	case puzzleStatePlaying:
		inputY := m.board.CellRows()*8 + 3
		m.input.SetPromoPopupY(inputY)
		sb.WriteString(m.input.View(m.board, m.game))
	case puzzleStateSuccess:
		sb.WriteString(puzzleGoodStyle.Render("Correct! Puzzle solved."))
		sb.WriteString("\n")
	case puzzleStateFailure:
		sb.WriteString(puzzleBadStyle.Render("Wrong move."))
		sb.WriteString("\n")
	case puzzleStateSolution:
		sb.WriteString(puzzleDimStyle.Render("Solution:"))
		sb.WriteString("\n")
	case puzzleStateLoading:
	}
	if m.err != "" {
		sb.WriteString(puzzleBadStyle.Render(m.err))
		sb.WriteString("\n")
	}
	var help string
	switch m.state {
	case puzzleStatePlaying:
		help = "enter/click:move  ←→:navigate  [:smaller  ]:larger  n:skip  q:back"
	case puzzleStateSolution:
		help = "←→:navigate solution  r:retry  n:next  q:back"
	case puzzleStateFailure:
		help = "r:retry  s:solution  ←→:navigate  [:smaller  ]:larger  n:next  q:back"
	case puzzleStateLoading, puzzleStateSuccess:
		help = "r:retry  ←→:navigate  [:smaller  ]:larger  n:next  q:back"
	}
	sb.WriteString(puzzleHelpStyle.Render(help))
	sb.WriteString("\n")
	return sb.String()
}

func (m *PuzzleModel) rightPanel() string {
	if m.record == nil {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Puzzle #%s  ★ %d\n", m.record.ID, m.record.Rating)
	if len(m.record.Themes) > 0 {
		sb.WriteString(puzzleDimStyle.Render("Themes: " + strings.Join(m.record.Themes, ", ")))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(puzzleDimStyle.Render(strings.Repeat("─", 22)))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "Puzzle rating:  %d\n", m.record.Rating)
	fmt.Fprintf(&sb, "Your rating:    %d\n", m.userPuzzleRating)
	if m.hasDelta {
		sign := "+"
		if m.lastDelta < 0 {
			sign = ""
		}
		fmt.Fprintf(&sb, "Last change:    %s%d\n", sign, m.lastDelta)
	} else {
		sb.WriteString(puzzleDimStyle.Render("Last change:    —"))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(puzzleDimStyle.Render(strings.Repeat("─", 22)))
	sb.WriteString("\n")
	sb.WriteString(m.moveList.View())
	return sb.String()
}
