package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

var (
	gameStatusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00"))
	gameHelpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	clockActiveStyle   = lipgloss.NewStyle().Bold(true).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FFCC00")).Padding(0, 1)
	clockInactiveStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#444444")).Padding(0, 1)
)

func formatMs(ms int) string {
	if ms < 0 {
		ms = 0
	}
	total := ms / 1000
	h := total / 3600
	min := (total % 3600) / 60
	sec := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, min, sec)
	}
	return fmt.Sprintf("%02d:%02d", min, sec)
}

type clockTickMsg time.Time

func clockTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockTickMsg(t) })
}

// GameModel is the active game screen.
type GameModel struct {
	gameID            string
	white             string
	black             string
	board             *render.Board
	moveList          *render.MoveList
	chess             *chess.Game
	myColor           chess.Color
	input             *LocalMoveInput
	statusMsg         string
	conn              *websocket.Conn
	username          string
	gameOver          bool
	result            string
	moves             []string
	pendingUndo       bool
	pendingUndoPrompt bool
	err               string
	viewIdx           int
	timed             bool
	whiteMs           int
	blackMs           int
}

// NewGameModel creates a new game screen.
func NewGameModel(gameID, white, black string, myColor chess.Color, conn *websocket.Conn, username string, tc shared.TimeControl) *GameModel {
	g := chess.NewGame()
	flipped := myColor == chess.Black
	return &GameModel{
		gameID:   gameID,
		white:    white,
		black:    black,
		board:    render.NewBoard(g.Position(), flipped),
		moveList: render.NewMoveList(20),
		chess:    g,
		myColor:  myColor,
		input:    NewLocalMoveInput(myColor == chess.Black),
		conn:     conn,
		username: username,
		timed:    tc.InitialSeconds > 0,
		whiteMs:  tc.InitialSeconds * 1000,
		blackMs:  tc.InitialSeconds * 1000,
	}
}

// ApplyMove updates the local game state and board after a move.
func (m *GameModel) ApplyMove(san string) {
	if err := m.chess.MoveStr(san); err != nil {
		m.statusMsg = fmt.Sprintf("invalid move: %s", err)
		return
	}
	m.moves = append(m.moves, san)
	moves := m.chess.Moves()
	positions := m.chess.Positions()
	if len(moves) > 0 {
		lastMove := moves[len(moves)-1]
		from := lastMove.S1()
		to := lastMove.S2()
		m.board.SetPosition(positions[len(positions)-1], from, to)
	}
	m.moveList.SetMoves(m.moves, len(m.moves)-1)
}

// SetMoves replaces the full move list (used after undo).
func (m *GameModel) SetMoves(moves []string) {
	m.chess = chess.NewGame()
	m.moves = nil
	for _, san := range moves {
		if err := m.chess.MoveStr(san); err != nil {
			break
		}
		m.moves = append(m.moves, san)
	}
	chessMoves := m.chess.Moves()
	positions := m.chess.Positions()
	if len(chessMoves) > 0 {
		lastMove := chessMoves[len(chessMoves)-1]
		m.board.SetPosition(positions[len(positions)-1], lastMove.S1(), lastMove.S2())
	} else {
		m.board.SetPosition(m.chess.Position(), 0, 0)
		m.board.ClearHighlight()
	}
	idx := len(m.moves) - 1
	m.moveList.SetMoves(m.moves, idx)
	m.viewIdx = len(m.moves)
}

// SetMovesWithClock replaces the full move list and updates clock state from server.
func (m *GameModel) SetMovesWithClock(moves []string, clock shared.ClockState) {
	m.SetMoves(moves)
	if m.timed {
		m.whiteMs = clock.WhiteMs
		m.blackMs = clock.BlackMs
	}
}

// renderAtViewIdx updates the board display to show the position at viewIdx.
func (m *GameModel) renderAtViewIdx() {
	if m.viewIdx == len(m.moves) {
		positions := m.chess.Positions()
		moves := m.chess.Moves()
		if len(moves) > 0 {
			last := moves[len(moves)-1]
			m.board.SetPosition(positions[len(positions)-1], last.S1(), last.S2())
		} else {
			m.board.SetPosition(m.chess.Position(), 0, 0)
			m.board.ClearHighlight()
		}
		m.moveList.SetMoves(m.moves, len(m.moves)-1)
		return
	}
	g := chess.NewGame()
	for _, san := range m.moves[:m.viewIdx] {
		_ = g.MoveStr(san)
	}
	positions := g.Positions()
	moves := g.Moves()
	if len(moves) > 0 {
		last := moves[len(moves)-1]
		m.board.SetPosition(positions[len(positions)-1], last.S1(), last.S2())
	} else {
		m.board.SetPosition(g.Position(), 0, 0)
		m.board.ClearHighlight()
	}
	m.moveList.SetMoves(m.moves, m.viewIdx-1)
}

// SetGameOver marks the game as over and shows the result.
func (m *GameModel) SetGameOver(result string, whiteElo, blackElo int) {
	m.gameOver = true
	m.result = result
	m.statusMsg = fmt.Sprintf("Game over: %s (White: %d, Black: %d)", result, whiteElo, blackElo)
}

// SetPendingUndoPrompt sets whether the opponent is requesting an undo.
func (m *GameModel) SetPendingUndoPrompt(v bool) {
	m.pendingUndoPrompt = v
}

// ClearPendingUndo clears the pending undo flag (e.g. when opponent rejects).
func (m *GameModel) ClearPendingUndo() {
	m.pendingUndo = false
}

// Init implements tea.Model.
func (m *GameModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.input.Init()}
	if m.timed {
		cmds = append(cmds, clockTick())
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m *GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if m.myColor != chess.NoColor && !m.gameOver && m.chess.Position().Turn() == m.myColor {
				san, handled, cmd := m.input.HandleMsg(msg, m.board, m.chess)
				if san != "" {
					return m, tea.Batch(cmd, m.sendMoveStr(san))
				}
				if handled {
					return m, cmd
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		// Always delegate to LocalMoveInput first — it handles enter, q/r/b/n promo keys, esc-promo.
		san, handled, inputCmd := m.input.HandleMsg(msg, m.board, m.chess)
		if san != "" {
			return m, tea.Batch(inputCmd, m.sendMoveStr(san))
		}
		if handled {
			return m, inputCmd
		}
		if m.pendingUndoPrompt {
			switch msg.String() {
			case "y":
				m.pendingUndoPrompt = false
				return m, m.sendUndoResponse(true)
			case "n":
				m.pendingUndoPrompt = false
				return m, m.sendUndoResponse(false)
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenLobby} }
		case "u":
			if m.myColor == chess.NoColor || len(m.moves) == 0 || m.pendingUndo {
				return m, nil
			}
			m.pendingUndo = true
			return m, m.sendUndo()
		case "ctrl+r":
			if m.myColor == chess.NoColor {
				return m, nil
			}
			return m, m.sendResign()
		case "ctrl+e":
			path, err := exportPGN(m.white, m.black, time.Now(), m.chess.String())
			if err != nil {
				m.statusMsg = fmt.Sprintf("export error: %s", err)
			} else {
				m.statusMsg = fmt.Sprintf("exported: %s", path)
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
		case "left":
			if m.viewIdx > 0 {
				m.viewIdx--
				m.renderAtViewIdx()
			}
		case "right":
			if m.viewIdx < len(m.moves) {
				m.viewIdx++
				m.renderAtViewIdx()
			}
		}
	case clockTickMsg:
		if m.timed && !m.gameOver {
			if m.chess.Position().Turn() == chess.White {
				if m.whiteMs > 0 {
					m.whiteMs -= 1000
				}
			} else {
				if m.blackMs > 0 {
					m.blackMs -= 1000
				}
			}
			return m, clockTick()
		}
	case ErrMsg:
		m.err = msg.Err.Error()
		return m, nil
	}
	return m, nil
}

func (m *GameModel) sendMoveStr(san string) tea.Cmd {
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgMove, shared.Move{GameID: m.gameID, SAN: san})
		if err != nil {
			return ErrMsg{Err: err}
		}
		if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

func (m *GameModel) sendUndo() tea.Cmd {
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgUndoRequest, shared.UndoRequest{GameID: m.gameID})
		if err != nil {
			return ErrMsg{Err: err}
		}
		if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

func (m *GameModel) sendResign() tea.Cmd {
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgResign, shared.Resign{GameID: m.gameID})
		if err != nil {
			return ErrMsg{Err: err}
		}
		if err := m.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return ErrMsg{Err: err}
		}
		return nil
	}
}

func (m *GameModel) sendUndoResponse(accept bool) tea.Cmd {
	return func() tea.Msg {
		data, err := shared.Encode(shared.MsgUndoResponse, shared.UndoResponse{GameID: m.gameID, Accept: accept})
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
func (m *GameModel) View() string {
	var sb strings.Builder
	boardView := m.board.View()
	moveView := m.moveList.View()
	left := boardView
	right := lipgloss.NewStyle().Bold(true).Render("Moves") + "\n" + moveView
	var columns []string
	columns = append(columns, left, "  ", right)
	if m.timed {
		turn := m.chess.Position().Turn()
		var blackStyle, whiteStyle lipgloss.Style
		if turn == chess.Black {
			blackStyle = clockActiveStyle
			whiteStyle = clockInactiveStyle
		} else {
			blackStyle = clockInactiveStyle
			whiteStyle = clockActiveStyle
		}
		blackClock := blackStyle.Render(formatMs(m.blackMs))
		whiteClock := whiteStyle.Render(formatMs(m.whiteMs))
		boardHeight := m.board.CellRows() * 8
		spacerHeight := boardHeight - lipgloss.Height(blackClock) - lipgloss.Height(whiteClock)
		if spacerHeight < 0 {
			spacerHeight = 0
		}
		spacer := strings.Repeat("\n", spacerHeight)
		clockCol := lipgloss.JoinVertical(lipgloss.Left, blackClock, spacer, whiteClock)
		columns = append(columns, "  ", clockCol)
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, columns...))
	sb.WriteString("\n")
	if m.viewIdx < len(m.moves) {
		sb.WriteString(gameStatusStyle.Render(fmt.Sprintf("Viewing move %d / %d  (\u2190 \u2192 to navigate)", m.viewIdx, len(m.moves))))
		sb.WriteString("\n")
	}
	if m.myColor == chess.NoColor {
		sb.WriteString(gameStatusStyle.Render("Spectating"))
		sb.WriteString("\n")
	} else if !m.gameOver {
		turn := m.chess.Position().Turn()
		var turnText string
		if turn == chess.White {
			turnText = "white's move"
		} else {
			turnText = "black's move"
		}
		sb.WriteString(gameStatusStyle.Render(turnText))
		sb.WriteString("\n")
		inputY := m.board.CellRows()*8 + 2
		m.input.SetPromoPopupY(inputY)
		sb.WriteString(m.input.View(m.board, m.chess))
	}
	if m.statusMsg != "" {
		sb.WriteString(gameStatusStyle.Render(m.statusMsg))
		sb.WriteString("\n")
	}
	if m.pendingUndoPrompt {
		sb.WriteString(gameStatusStyle.Render("Opponent requests undo. Accept? (y/n)"))
		sb.WriteString("\n")
	}
	if m.err != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.err))
		sb.WriteString("\n")
	}
	var help string
	switch {
	case m.myColor == chess.NoColor:
		help = "\u2190\u2192:navigate history  ctrl+e:export  esc:lobby"
	case m.gameOver:
		help = "\u2190\u2192:navigate history  ctrl+e:export  esc:lobby"
	default:
		help = "enter/click:move  u:undo  ctrl+r:resign  ctrl+e:export  [/]:resize  \u2190\u2192:history  esc:lobby"
	}
	sb.WriteString(gameHelpStyle.Render(help))
	sb.WriteString("\n")
	return sb.String()
}
