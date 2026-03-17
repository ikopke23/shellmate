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
	gameStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00"))
	gameHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

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
}

// NewGameModel creates a new game screen.
func NewGameModel(gameID, white, black string, myColor chess.Color, conn *websocket.Conn, username string) *GameModel {
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
	return m.input.Init()
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
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	sb.WriteString("\n")
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
		help = "ctrl+e:export  esc:back to lobby"
	case m.gameOver:
		help = "ctrl+e:export  esc:back to lobby"
	default:
		help = "enter/click:move  u:undo  ctrl+r:resign  ctrl+e:export  [/]:resize  esc:lobby"
	}
	sb.WriteString(gameHelpStyle.Render(help))
	sb.WriteString("\n")
	return sb.String()
}
