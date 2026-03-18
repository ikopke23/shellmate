package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/client/screens"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

type historyLoadedMsg struct{ records []shared.HistoryRecord }
type leaderboardLoadedMsg struct{ players []shared.PlayerInfo }
type importedGamesLoadedMsg struct{ records []shared.HistoryRecord }
type puzzleLoadedMsg struct{ record shared.PuzzleRecord }

// Model is the root bubbletea model.
type Model struct {
	screen        screens.ScreenID
	conn          *websocket.Conn
	username      string
	serverAddr    string
	login         *screens.LoginModel
	lobby         *screens.LobbyModel
	game          *screens.GameModel
	history       *screens.HistoryModel
	replay        *screens.ReplayModel
	leaderboard   *screens.LeaderboardModel
	importScreen  *screens.ImportModel
	importedGames *screens.ImportedGamesModel
	puzzle        *screens.PuzzleModel
	width         int
	height        int
}

// NewModel creates the root model starting at the login screen.
func NewModel(serverAddr string) Model {
	return Model{
		screen:     screens.ScreenLogin,
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
		m.screen = screens.ScreenLobby
		cmds := []tea.Cmd{m.listenWS()}
		if msg.FirstMsg != nil {
			updated, cmd := m.handleWSMsg(screens.WSMsg{Env: *msg.FirstMsg})
			if um, ok := updated.(Model); ok {
				m = um
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	case screens.ScreenChangeMsg:
		return m.handleScreenChange(msg)
	case screens.WSMsg:
		return m.handleWSMsg(msg)
	case historyLoadedMsg:
		if m.history != nil {
			m.history.SetGames(msg.records)
		}
		return m, nil
	case leaderboardLoadedMsg:
		if m.leaderboard != nil {
			m.leaderboard.SetPlayers(msg.players)
		}
		return m, nil
	case importedGamesLoadedMsg:
		if m.importedGames != nil {
			m.importedGames.SetGames(msg.records)
		}
		return m, nil
	case puzzleLoadedMsg:
		if m.puzzle != nil {
			m.puzzle.SetPuzzle(msg.record)
		}
		return m, nil
	case screens.ErrMsg:
		// pass through to active screen
	}
	return m.updateActiveScreen(msg)
}

func (m Model) updateActiveScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screens.ScreenLogin:
		updated, cmd := m.login.Update(msg)
		if lm, ok := updated.(*screens.LoginModel); ok {
			m.login = lm
		}
		return m, cmd
	case screens.ScreenLobby:
		updated, cmd := m.lobby.Update(msg)
		if lm, ok := updated.(*screens.LobbyModel); ok {
			m.lobby = lm
		}
		return m, cmd
	case screens.ScreenGame:
		updated, cmd := m.game.Update(msg)
		if gm, ok := updated.(*screens.GameModel); ok {
			m.game = gm
		}
		return m, cmd
	case screens.ScreenHistory:
		updated, cmd := m.history.Update(msg)
		if hm, ok := updated.(*screens.HistoryModel); ok {
			m.history = hm
		}
		return m, cmd
	case screens.ScreenReplay:
		updated, cmd := m.replay.Update(msg)
		if rm, ok := updated.(*screens.ReplayModel); ok {
			m.replay = rm
		}
		return m, cmd
	case screens.ScreenLeaderboard:
		updated, cmd := m.leaderboard.Update(msg)
		if lm, ok := updated.(*screens.LeaderboardModel); ok {
			m.leaderboard = lm
		}
		return m, cmd
	case screens.ScreenImport:
		updated, cmd := m.importScreen.Update(msg)
		if im, ok := updated.(*screens.ImportModel); ok {
			m.importScreen = im
		}
		return m, cmd
	case screens.ScreenImportedGames:
		updated, cmd := m.importedGames.Update(msg)
		if ig, ok := updated.(*screens.ImportedGamesModel); ok {
			m.importedGames = ig
		}
		return m, cmd
	case screens.ScreenPuzzle:
		updated, cmd := m.puzzle.Update(msg)
		if pm, ok := updated.(*screens.PuzzleModel); ok {
			m.puzzle = pm
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) handleScreenChange(msg screens.ScreenChangeMsg) (tea.Model, tea.Cmd) {
	switch msg.Screen {
	case screens.ScreenLobby:
		if m.lobby == nil {
			m.lobby = screens.NewLobbyModel(m.username, m.conn)
		}
		m.screen = screens.ScreenLobby
	case screens.ScreenHistory:
		m.history = screens.NewHistoryModel(m.username, m.conn)
		m.screen = screens.ScreenHistory
		return m, m.fetchHistory()
	case screens.ScreenImport:
		m.importScreen = screens.NewImportModel()
		m.screen = screens.ScreenImport
	case screens.ScreenImportedGames:
		m.importedGames = screens.NewImportedGamesModel()
		m.screen = screens.ScreenImportedGames
		return m, m.fetchImportedGames()
	case screens.ScreenReplay:
		m.replay = screens.NewReplayModel()
		m.replay.SetServerAddr(m.serverAddr)
		switch d := msg.Data.(type) {
		case shared.HistoryRecord:
			if d.PGN != "" {
				_ = m.replay.LoadPGN(d.PGN)
				m.replay.SetMeta(d.White, d.Black, d.PlayedAt)
				m.replay.SetBackScreen(screens.ScreenHistory)
			}
		case screens.ImportPGNData:
			if d.Record.PGN != "" {
				_ = m.replay.LoadPGN(d.Record.PGN)
				m.replay.SetBackScreen(screens.ScreenImport)
			}
		case screens.ImportedGamesOpenData:
			if d.Record.PGN != "" {
				_ = m.replay.LoadPGN(d.Record.PGN)
				m.replay.SetMeta(d.Record.White, d.Record.Black, d.Record.PlayedAt)
				m.replay.SetBackScreen(screens.ScreenImportedGames)
			}
		}
		m.screen = screens.ScreenReplay
	case screens.ScreenLeaderboard:
		m.leaderboard = screens.NewLeaderboardModel(m.conn)
		m.screen = screens.ScreenLeaderboard
		return m, m.fetchLeaderboard()
	case screens.ScreenPuzzle:
		m.puzzle = screens.NewPuzzleModel(m.serverAddr, m.username)
		m.screen = screens.ScreenPuzzle
		return m, m.fetchPuzzle()
	case screens.ScreenGame:
		// game screen is set up by game_start messages
		m.screen = screens.ScreenGame
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
	case shared.MsgGameStart:
		var payload shared.GameStart
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return m, m.listenWS()
		}
		m.startGameFromMsg(payload)
		return m, m.listenWS()
	case shared.MsgMove:
		var payload struct {
			GameID string   `json:"game_id"`
			SAN    string   `json:"san"`
			Moves  []string `json:"moves"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil && m.game != nil {
			m.game.SetMoves(payload.Moves)
			if m.screen != screens.ScreenGame {
				m.screen = screens.ScreenGame
			}
		}
	case shared.MsgGameOver:
		var payload shared.GameOver
		if err := json.Unmarshal(env.Payload, &payload); err == nil && m.game != nil {
			m.game.SetGameOver(payload.Result, payload.WhiteEloAfter, payload.BlackEloAfter)
		}
	case shared.MsgUndoRequest:
		if m.game != nil {
			m.game.SetPendingUndoPrompt(true)
		}
	case shared.MsgUndoResponse:
		var payload shared.UndoResponse
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			if m.game != nil && !payload.Accept {
				m.game.ClearPendingUndo()
			}
		}
	case shared.MsgUndoAccepted:
		var payload shared.UndoAccepted
		if err := json.Unmarshal(env.Payload, &payload); err == nil && m.game != nil {
			m.game.SetMoves(payload.Moves)
		}
	case shared.MsgError:
		var payload shared.ErrorMsg
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			updated, _ := m.updateActiveScreen(screens.ErrMsg{Err: errString(payload.Message)})
			if um, ok := updated.(Model); ok {
				m = um
			}
		}
	}
	return m, m.listenWS()
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
	case screens.ScreenLogin:
		return m.login.View()
	case screens.ScreenLobby:
		if m.lobby != nil {
			return m.lobby.View()
		}
	case screens.ScreenGame:
		if m.game != nil {
			return m.game.View()
		}
	case screens.ScreenHistory:
		if m.history != nil {
			return m.history.View()
		}
	case screens.ScreenReplay:
		if m.replay != nil {
			return m.replay.View()
		}
	case screens.ScreenLeaderboard:
		if m.leaderboard != nil {
			return m.leaderboard.View()
		}
	case screens.ScreenImport:
		if m.importScreen != nil {
			return m.importScreen.View()
		}
	case screens.ScreenImportedGames:
		if m.importedGames != nil {
			return m.importedGames.View()
		}
	case screens.ScreenPuzzle:
		if m.puzzle != nil {
			return m.puzzle.View()
		}
	}
	return ""
}

type errString string

func (e errString) Error() string { return string(e) }

func (m *Model) fetchHistory() tea.Cmd {
	return func() tea.Msg {
		url := "http://" + m.serverAddr + "/history?user=" + m.username
		resp, err := http.Get(url)
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		defer resp.Body.Close()
		var records []shared.HistoryRecord
		if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
			return screens.ErrMsg{Err: err}
		}
		return historyLoadedMsg{records: records}
	}
}

func (m *Model) fetchImportedGames() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get("http://" + m.serverAddr + "/imported-games")
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		defer resp.Body.Close()
		var records []shared.HistoryRecord
		if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
			return screens.ErrMsg{Err: err}
		}
		return importedGamesLoadedMsg{records: records}
	}
}

func (m *Model) fetchLeaderboard() tea.Cmd {
	return func() tea.Msg {
		url := "http://" + m.serverAddr + "/leaderboard"
		resp, err := http.Get(url)
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		defer resp.Body.Close()
		var players []shared.PlayerInfo
		if err := json.NewDecoder(resp.Body).Decode(&players); err != nil {
			return screens.ErrMsg{Err: err}
		}
		return leaderboardLoadedMsg{players: players}
	}
}

func (m *Model) fetchPuzzle() tea.Cmd {
	return func() tea.Msg {
		url := "http://" + m.serverAddr + "/puzzle?user=" + m.username
		resp, err := http.Get(url)
		if err != nil {
			return screens.ErrMsg{Err: fmt.Errorf("fetch puzzle: %w", err)}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return screens.ErrMsg{Err: fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
		}
		var record shared.PuzzleRecord
		if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
			return screens.ErrMsg{Err: fmt.Errorf("decode puzzle response: %w", err)}
		}
		return puzzleLoadedMsg{record: record}
	}
}

// startGameFromMsg creates a new GameModel when the server notifies of a game start.
func (m *Model) startGameFromMsg(payload shared.GameStart) {
	var myColor chess.Color
	switch m.username {
	case payload.White:
		myColor = chess.White
	case payload.Black:
		myColor = chess.Black
	default:
		myColor = chess.NoColor
	}
	m.game = screens.NewGameModel(payload.GameID, payload.White, payload.Black, myColor, m.conn, m.username)
	m.screen = screens.ScreenGame
}
