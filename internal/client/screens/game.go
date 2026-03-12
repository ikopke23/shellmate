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

const screenLobby = 1

var (
	gameStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00"))
	gameHelpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// GameModel is the active game screen.
type GameModel struct {
	gameID    string
	board     *render.Board
	moveList  *render.MoveList
	chess     *chess.Game
	myColor   chess.Color
	moveInput textinput.Model
	statusMsg string
	conn      *websocket.Conn
	username  string
	gameOver  bool
	result    string
	moves     []string
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

// Init implements tea.Model.
func (m *GameModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m *GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m, m.sendMove()
		case "ctrl+u":
			return m, m.sendUndo()
		case "ctrl+r":
			return m, m.sendResign()
		case "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: screenLobby} }
		}
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
		data, err := shared.Encode(shared.MsgMove, shared.Move{GameID: m.gameID, SAN: "resign"})
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
	if !m.gameOver {
		sb.WriteString(m.moveInput.View())
		sb.WriteString("\n")
	}
	if m.statusMsg != "" {
		sb.WriteString(gameStatusStyle.Render(m.statusMsg))
		sb.WriteString("\n")
	}
	help := "enter:move  ctrl+u:undo  ctrl+r:resign  esc:lobby"
	if m.gameOver {
		help = "esc:back to lobby"
	}
	sb.WriteString(gameHelpStyle.Render(help))
	sb.WriteString("\n")
	return sb.String()
}
