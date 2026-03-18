package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

type puzzleState int

const (
	puzzleStateLoading puzzleState = iota
	puzzleStatePlaying
	puzzleStateSuccess
	puzzleStateFailure
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

// PuzzleModel is the puzzle mode screen.
type PuzzleModel struct {
	serverAddr       string
	username         string
	state            puzzleState
	record           *shared.PuzzleRecord
	game             *chess.Game
	initialFEN       string
	solution         []string // UCI move list; [0] already applied on load
	solutionIdx      int      // next expected move index (starts at 1)
	board            *render.Board
	input            *LocalMoveInput
	userPuzzleRating int
	lastDelta        int
	hasDelta         bool
	err              string
}

// NewPuzzleModel creates a puzzle screen in loading state.
func NewPuzzleModel(serverAddr, username string) *PuzzleModel {
	return &PuzzleModel{
		serverAddr: serverAddr,
		username:   username,
		state:      puzzleStateLoading,
		board:      render.NewBoard(chess.NewGame().Position(), false),
	}
}

// SetPuzzle initialises the puzzle state from a loaded record.
// Called by the root model when the fetch Cmd completes.
func (m *PuzzleModel) SetPuzzle(record shared.PuzzleRecord) {
	m.record = &record
	m.userPuzzleRating = record.UserPuzzleRating
	m.solution = strings.Fields(record.Moves)
	m.initialFEN = record.FEN
	m.hasDelta = false
	m.err = ""
	m.initGame()
}

// initGame creates a chess.Game from the stored FEN and applies moves[0].
func (m *PuzzleModel) initGame() {
	fenOpt, err := chess.FEN(m.initialFEN)
	if err != nil {
		m.err = fmt.Sprintf("invalid FEN: %v", err)
		return
	}
	g := chess.NewGame(fenOpt)
	if len(m.solution) > 0 {
		uci := chess.LongAlgebraicNotation{}
		move, err := uci.Decode(g.Position(), m.solution[0])
		if err != nil {
			m.err = fmt.Sprintf("apply setup move: %v", err)
			return
		}
		if err := g.Move(move); err != nil {
			m.err = fmt.Sprintf("game.Move setup: %v", err)
			return
		}
		m.board.SetPosition(g.Position(), move.S1(), move.S2())
	} else {
		m.board.SetPosition(g.Position(), 0, 0)
	}
	m.game = g
	m.solutionIdx = 1
	m.input = NewLocalMoveInput(false)
	m.state = puzzleStatePlaying
}

// retry resets the puzzle to its initial playing state without recording an attempt.
func (m *PuzzleModel) retry() {
	m.initGame()
}

// validateAndApply checks userSAN against the expected solution move.
// If correct, applies the move (and any following opponent response).
// Returns true on correct move.
func (m *PuzzleModel) validateAndApply(userSAN string) bool {
	if m.solutionIdx >= len(m.solution) || m.game == nil {
		return false
	}
	expectedUCI := m.solution[m.solutionIdx]
	algN := chess.AlgebraicNotation{}
	uciN := chess.LongAlgebraicNotation{}
	pos := m.game.Position()
	var matchedMove *chess.Move
	for _, mv := range m.game.ValidMoves() {
		if algN.Encode(pos, mv) != userSAN {
			continue
		}
		if uciN.Encode(pos, mv) != expectedUCI {
			// SAN is valid but not the expected move
			m.state = puzzleStateFailure
			return false
		}
		matchedMove = mv
		break
	}
	if matchedMove == nil {
		// invalid SAN or no valid match
		m.state = puzzleStateFailure
		return false
	}
	if err := m.game.Move(matchedMove); err != nil {
		m.state = puzzleStateFailure
		return false
	}
	m.board.SetPosition(m.game.Position(), matchedMove.S1(), matchedMove.S2())
	m.solutionIdx++
	// auto-apply opponent response if present
	if m.solutionIdx < len(m.solution) {
		opUCI := m.solution[m.solutionIdx]
		opPos := m.game.Position()
		for _, opMv := range m.game.ValidMoves() {
			if uciN.Encode(opPos, opMv) == opUCI {
				if err := m.game.Move(opMv); err == nil {
					m.board.SetPosition(m.game.Position(), opMv.S1(), opMv.S2())
					m.solutionIdx++
				}
				break
			}
		}
		return true
	}
	// no more moves — puzzle complete
	m.state = puzzleStateSuccess
	return true
}

// Init implements tea.Model.
func (m *PuzzleModel) Init() tea.Cmd {
	if m.input != nil {
		return m.input.Init()
	}
	return nil
}

// Update implements tea.Model.
func (m *PuzzleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PuzzleAttemptMsg:
		if msg.Err == nil {
			delta := msg.NewRating - m.userPuzzleRating
			m.userPuzzleRating = msg.NewRating
			m.lastDelta = delta
			m.hasDelta = true
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLobby} }
		case "ctrl+c":
			return m, tea.Quit
		case "r":
			if m.state == puzzleStateFailure || m.state == puzzleStateSuccess {
				m.retry()
			}
			return m, m.initCmd()
		case "n":
			if m.state == puzzleStatePlaying || m.state == puzzleStateFailure {
				return m, m.skipAndSubmit()
			}
			if m.state == puzzleStateSuccess {
				return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenPuzzle} }
			}
		}
		if m.state == puzzleStatePlaying && m.input != nil {
			san, _, cmd := m.input.HandleMsg(msg, m.board, m.game)
			if san != "" {
				correct := m.validateAndApply(san)
				if correct && m.state == puzzleStateSuccess {
					return m, tea.Batch(cmd, m.submitAttempt(true))
				}
				if !correct {
					return m, tea.Batch(cmd, m.submitAttempt(false))
				}
			}
			return m, cmd
		}
	case tea.MouseMsg:
		if m.state == puzzleStatePlaying && m.input != nil {
			san, _, cmd := m.input.HandleMsg(msg, m.board, m.game)
			if san != "" {
				correct := m.validateAndApply(san)
				if correct && m.state == puzzleStateSuccess {
					return m, tea.Batch(cmd, m.submitAttempt(true))
				}
				if !correct {
					return m, tea.Batch(cmd, m.submitAttempt(false))
				}
			}
			return m, cmd
		}
	case ErrMsg:
		m.err = msg.Err.Error()
	}
	return m, nil
}

func (m *PuzzleModel) initCmd() tea.Cmd {
	if m.input != nil {
		return m.input.Init()
	}
	return nil
}

func (m *PuzzleModel) submitAttempt(solved bool) tea.Cmd {
	if m.record == nil {
		return nil
	}
	id := m.record.ID
	username := m.username
	addr := m.serverAddr
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{
			"username":  username,
			"puzzle_id": id,
			"solved":    solved,
		})
		resp, err := http.Post("http://"+addr+"/puzzle/attempt", "application/json", bytes.NewReader(body))
		if err != nil {
			return PuzzleAttemptMsg{Err: err}
		}
		defer resp.Body.Close()
		var result shared.PuzzleAttemptResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return PuzzleAttemptMsg{Err: err}
		}
		return PuzzleAttemptMsg{NewRating: result.PuzzleRating}
	}
}

// skipAndSubmit records the current puzzle as failed (if not yet attempted) then navigates to the next puzzle.
func (m *PuzzleModel) skipAndSubmit() tea.Cmd {
	attemptCmd := m.submitAttempt(false)
	return tea.Batch(attemptCmd, func() tea.Msg {
		return ScreenChangeMsg{Screen: ScreenPuzzle}
	})
}

// View implements tea.Model.
func (m *PuzzleModel) View() string {
	if m.state == puzzleStateLoading {
		return puzzleTitleStyle.Render("Puzzle Mode") + "\n\nLoading puzzle...\n"
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
	}
	if m.err != "" {
		sb.WriteString(puzzleBadStyle.Render(m.err))
		sb.WriteString("\n")
	}
	var help string
	switch m.state {
	case puzzleStatePlaying:
		help = "enter/click:move  n:skip  q:back"
	default:
		help = "r:retry  n:next  q:back"
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
	sb.WriteString(fmt.Sprintf("Puzzle #%s  ★ %d\n", m.record.ID, m.record.Rating))
	if len(m.record.Themes) > 0 {
		sb.WriteString(puzzleDimStyle.Render("Themes: " + strings.Join(m.record.Themes, ", ")))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(puzzleDimStyle.Render(strings.Repeat("─", 22)))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Puzzle rating:  %d\n", m.record.Rating))
	sb.WriteString(fmt.Sprintf("Your rating:    %d\n", m.userPuzzleRating))
	if m.hasDelta {
		sign := "+"
		if m.lastDelta < 0 {
			sign = ""
		}
		sb.WriteString(fmt.Sprintf("Last change:    %s%d\n", sign, m.lastDelta))
	} else {
		sb.WriteString(puzzleDimStyle.Render("Last change:    —"))
		sb.WriteString("\n")
	}
	return sb.String()
}
