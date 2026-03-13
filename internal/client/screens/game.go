package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
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
	board             *render.Board
	moveList          *render.MoveList
	chess             *chess.Game
	myColor           chess.Color
	moveInput         textinput.Model
	statusMsg         string
	conn              *websocket.Conn
	username          string
	gameOver          bool
	result            string
	moves             []string
	pendingUndo       bool
	pendingUndoPrompt bool
	err               string
	selectedSq        chess.Square
	hasSelected       bool
}

// NewGameModel creates a new game screen.
func NewGameModel(gameID string, myColor chess.Color, conn *websocket.Conn, username string) *GameModel {
	g := chess.NewGame()
	flipped := myColor == chess.Black
	mi := textinput.New()
	mi.Placeholder = "Type move (e.g. e4)"
	mi.Focus()
	mi.CharLimit = 10
	mi.Width = 20
	return &GameModel{
		gameID:    gameID,
		board:     render.NewBoard(g.Position(), flipped),
		moveList:  render.NewMoveList(20),
		chess:     g,
		myColor:   myColor,
		moveInput: mi,
		conn:      conn,
		username:  username,
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
	return textinput.Blink
}

// Update implements tea.Model.
func (m *GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if m.myColor != chess.NoColor && !m.gameOver && m.chess.Position().Turn() == m.myColor {
				sq, ok := m.squareFromMouse(msg.X, msg.Y)
				if ok {
					pos := m.chess.Position()
					board := pos.Board()
					piece := board.Piece(sq)
					if !m.hasSelected {
						if piece != chess.NoPiece && piece.Color() == m.myColor {
							m.selectedSq = sq
							m.hasSelected = true
							m.board.SetSelected(sq)
						}
					} else if sq == m.selectedSq {
						m.hasSelected = false
						m.board.ClearSelected()
					} else if piece != chess.NoPiece && piece.Color() == m.myColor {
						m.selectedSq = sq
						m.board.SetSelected(sq)
					} else {
						san := m.mouseToSAN(m.selectedSq, sq)
						m.hasSelected = false
						m.board.ClearSelected()
						if san != "" {
							return m, m.sendMoveStr(san)
						}
					}
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
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
		case "enter":
			if m.myColor == chess.NoColor {
				return m, nil
			}
			return m, m.sendMove()
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
		}
	case ErrMsg:
		m.err = msg.Err.Error()
		return m, nil
	}
	var cmd tea.Cmd
	m.moveInput, cmd = m.moveInput.Update(msg)
	return m, cmd
}

func (m *GameModel) sendMove() tea.Cmd {
	san := strings.TrimSpace(m.moveInput.Value())
	if san == "" {
		return nil
	}
	m.moveInput.SetValue("")
	return m.sendMoveStr(san)
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

func (m *GameModel) squareFromMouse(x, y int) (chess.Square, bool) {
	if x < 2 || x > 49 || y < 0 || y > 23 {
		return 0, false
	}
	cellCol := (x - 2) / 6
	cellRow := y / 3
	if cellCol < 0 || cellCol > 7 {
		return 0, false
	}
	var rankIdx, fileIdx int
	if m.board.Flipped() {
		rankIdx = cellRow
		fileIdx = 7 - cellCol
	} else {
		rankIdx = 7 - cellRow
		fileIdx = cellCol
	}
	return chess.Square(rankIdx*8 + fileIdx), true
}

func (m *GameModel) mouseToSAN(from, to chess.Square) string {
	pos := m.chess.Position()
	var bestMove *chess.Move
	for _, mv := range m.chess.ValidMoves() {
		if mv.S1() == from && mv.S2() == to {
			if mv.Promo() == chess.Queen {
				bestMove = mv
				break
			}
			if bestMove == nil {
				bestMove = mv
			}
		}
	}
	if bestMove == nil {
		return ""
	}
	san := chess.AlgebraicNotation{}.Encode(pos, bestMove)
	return san
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
		sb.WriteString(m.moveInput.View())
		sb.WriteString("\n")
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
		help = "esc:back to lobby"
	case m.gameOver:
		help = "esc:back to lobby"
	default:
		help = "enter/click:move  u:undo  ctrl+r:resign  esc:lobby"
	}
	sb.WriteString(gameHelpStyle.Render(help))
	sb.WriteString("\n")
	return sb.String()
}
