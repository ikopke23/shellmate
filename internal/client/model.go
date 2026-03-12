package client

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/client/screens"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// Screen identifies which screen is active.
type Screen int

const (
	ScreenLogin Screen = iota
	ScreenLobby
	ScreenGame
	ScreenHistory
	ScreenReplay
	ScreenLeaderboard
)

// Model is the root bubbletea model.
type Model struct {
	screen      Screen
	conn        *websocket.Conn
	username    string
	serverAddr  string
	login       *screens.LoginModel
	lobby       *screens.LobbyModel
	game        *screens.GameModel
	history     *screens.HistoryModel
	replay      *screens.ReplayModel
	leaderboard *screens.LeaderboardModel
	width       int
	height      int
}

// NewModel creates the root model starting at the login screen.
func NewModel(serverAddr string) Model {
	return Model{
		screen:     ScreenLogin,
		serverAddr: serverAddr,
		login:      screens.NewLoginModel(serverAddr),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.login.Init()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case screens.ConnectedMsg:
		m.conn = msg.Conn
		m.username = msg.Username
		m.lobby = screens.NewLobbyModel(msg.Username, msg.Conn)
		m.screen = ScreenLobby
		return m, m.listenWS()
	case screens.ScreenChangeMsg:
		return m.handleScreenChange(msg)
	case screens.WSMsg:
		return m.handleWSMsg(msg)
	case screens.ErrMsg:
		// pass through to active screen
	}
	return m.updateActiveScreen(msg)
}

func (m Model) updateActiveScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenLogin:
		updated, cmd := m.login.Update(msg)
		if lm, ok := updated.(*screens.LoginModel); ok {
			m.login = lm
		}
		return m, cmd
	case ScreenLobby:
		updated, cmd := m.lobby.Update(msg)
		if lm, ok := updated.(*screens.LobbyModel); ok {
			m.lobby = lm
		}
		return m, cmd
	case ScreenGame:
		updated, cmd := m.game.Update(msg)
		if gm, ok := updated.(*screens.GameModel); ok {
			m.game = gm
		}
		return m, cmd
	case ScreenHistory:
		updated, cmd := m.history.Update(msg)
		if hm, ok := updated.(*screens.HistoryModel); ok {
			m.history = hm
		}
		return m, cmd
	case ScreenReplay:
		updated, cmd := m.replay.Update(msg)
		if rm, ok := updated.(*screens.ReplayModel); ok {
			m.replay = rm
		}
		return m, cmd
	case ScreenLeaderboard:
		updated, cmd := m.leaderboard.Update(msg)
		if lm, ok := updated.(*screens.LeaderboardModel); ok {
			m.leaderboard = lm
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) handleScreenChange(msg screens.ScreenChangeMsg) (tea.Model, tea.Cmd) {
	switch Screen(msg.Screen) {
	case ScreenLobby:
		if m.lobby == nil {
			m.lobby = screens.NewLobbyModel(m.username, m.conn)
		}
		m.screen = ScreenLobby
	case ScreenHistory:
		m.history = screens.NewHistoryModel(m.username, m.conn)
		m.screen = ScreenHistory
	case ScreenReplay:
		m.replay = screens.NewReplayModel()
		if rec, ok := msg.Data.(shared.HistoryRecord); ok && rec.PGN != "" {
			_ = m.replay.LoadPGN(rec.PGN)
		}
		m.screen = ScreenReplay
	case ScreenLeaderboard:
		m.leaderboard = screens.NewLeaderboardModel(m.conn)
		m.screen = ScreenLeaderboard
	case ScreenGame:
		// game screen is set up by game_start messages
		m.screen = ScreenGame
	}
	return m, nil
}

func (m Model) handleWSMsg(msg screens.WSMsg) (tea.Model, tea.Cmd) {
	env := msg.Env
	switch env.Type {
	case shared.MsgLobbyState:
		var state shared.LobbyState
		if err := json.Unmarshal(env.Payload, &state); err == nil && m.lobby != nil {
			m.lobby.SetState(state)
		}
	case shared.MsgMove:
		var payload struct {
			GameID string   `json:"game_id"`
			SAN    string   `json:"san"`
			Moves  []string `json:"moves"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil && m.game != nil {
			m.game.SetMoves(payload.Moves)
			if m.screen != ScreenGame {
				m.screen = ScreenGame
			}
		}
	case shared.MsgGameOver:
		var payload shared.GameOver
		if err := json.Unmarshal(env.Payload, &payload); err == nil && m.game != nil {
			m.game.SetGameOver(payload.Result, payload.WhiteEloAfter, payload.BlackEloAfter)
		}
	case shared.MsgUndoRequest:
		// For now, auto-show a status message; proper UI would prompt
		if m.game != nil {
			// handled in game screen
		}
	case shared.MsgUndoAccepted:
		var payload shared.UndoAccepted
		if err := json.Unmarshal(env.Payload, &payload); err == nil && m.game != nil {
			m.game.SetMoves(payload.Moves)
		}
	case shared.MsgError:
		var payload shared.ErrorMsg
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			return m.updateActiveScreen(screens.ErrMsg{Err: errString(payload.Message)})
		}
	}
	return m, nil
}

// listenWS returns a Cmd that reads from the WebSocket and sends WSMsg.
func (m Model) listenWS() tea.Cmd {
	conn := m.conn
	return func() tea.Msg {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		env, err := shared.Decode(msg)
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		return screens.WSMsg{Env: env}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.screen {
	case ScreenLogin:
		return m.login.View()
	case ScreenLobby:
		if m.lobby != nil {
			return m.lobby.View()
		}
	case ScreenGame:
		if m.game != nil {
			return m.game.View()
		}
	case ScreenHistory:
		if m.history != nil {
			return m.history.View()
		}
	case ScreenReplay:
		if m.replay != nil {
			return m.replay.View()
		}
	case ScreenLeaderboard:
		if m.leaderboard != nil {
			return m.leaderboard.View()
		}
	}
	return ""
}

type errString string

func (e errString) Error() string { return string(e) }

// startGameFromMsg creates a new GameModel when the server notifies of a game start.
func (m *Model) startGameFromMsg(gameID string, myColor chess.Color) {
	m.game = screens.NewGameModel(gameID, myColor, m.conn, m.username)
	m.screen = ScreenGame
}
