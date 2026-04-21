// Package client implements the Bubble Tea TUI model and screen routing
// for the shellmate client.
package client

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/client/screens"
	"github.com/ikopke/shellmate/internal/server"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

type (
	historyLoadedMsg       struct{ records []shared.HistoryRecord }
	leaderboardLoadedMsg   struct{ players []shared.PlayerInfo }
	importedGamesLoadedMsg struct{ records []shared.HistoryRecord }
	puzzleLoadedMsg        struct{ record shared.PuzzleRecord }
)

// Model is the root bubbletea model for an authenticated SSH session.
type Model struct {
	screen        screens.ScreenID
	hub           *server.Hub
	client        *server.Client
	user          *server.User
	lobby         *screens.LobbyModel
	game          *screens.GameModel
	history       *screens.HistoryModel
	replay        *screens.ReplayModel
	leaderboard   *screens.LeaderboardModel
	importScreen  *screens.ImportModel
	importedGames *screens.ImportedGamesModel
	puzzle        *screens.PuzzleModel
	createGame    *screens.CreateGameModel
	width, height int
}

// NewModel creates the root model starting at the lobby screen.
func NewModel(hub *server.Hub, client *server.Client, user *server.User, w, h int) Model {
	return Model{
		screen: screens.ScreenLobby,
		hub:    hub,
		client: client,
		user:   user,
		lobby:  screens.NewLobbyModel(user.Username),
		width:  w,
		height: h,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	hub := m.hub
	return tea.Batch(m.client.Recv(), m.lobby.Init(), func() tea.Msg {
		hub.BroadcastLobby(context.Background())
		return nil
	})
}

func (m Model) handleServerError(msg shared.ErrorMsg) (tea.Model, tea.Cmd) {
	updated, _ := m.updateActiveScreen(screens.ErrMsg{Err: errString(msg.Message)})
	if um, ok := updated.(Model); ok {
		m = um
	}
	return m, m.client.Recv()
}

func (m Model) handleGameLifecycleMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case shared.GameStart:
		var myColor chess.Color
		switch m.user.Username {
		case msg.White:
			myColor = chess.White
		case msg.Black:
			myColor = chess.Black
		default:
			myColor = chess.NoColor
		}
		m.game = screens.NewGameModel(msg.GameID, msg.White, msg.Black, myColor, m.user.Username, msg.TimeControl)
		m.screen = screens.ScreenGame
		return m, tea.Batch(m.game.Init(), m.client.Recv()), true
	case shared.MoveMsg:
		if m.game != nil {
			m.game.SetMovesWithClock(msg.Moves, msg.Clock)
			if m.screen != screens.ScreenGame {
				m.screen = screens.ScreenGame
			}
		}
		return m, m.client.Recv(), true
	case shared.GameOver:
		if m.game != nil {
			m.game.SetGameOver(msg.Result, msg.WhiteEloAfter, msg.BlackEloAfter)
		}
		return m, m.client.Recv(), true
	case shared.UndoRequest:
		if m.game != nil {
			m.game.SetPendingUndoPrompt(true)
		}
		return m, m.client.Recv(), true
	case shared.UndoResponse:
		if m.game != nil && !msg.Accept {
			m.game.ClearPendingUndo()
		}
		return m, m.client.Recv(), true
	case shared.UndoAccepted:
		if m.game != nil {
			m.game.SetMoves(msg.Moves)
		}
		return m, m.client.Recv(), true
	}
	return m, nil, false
}

func (m Model) handleHubActionMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case screens.JoinGameMsg:
		hub, client := m.hub, m.client
		gameID := msg.GameID
		return m, func() tea.Msg {
			hub.JoinGame(context.Background(), client, gameID)
			return nil
		}, true
	case screens.SpectateGameMsg:
		hub, client := m.hub, m.client
		gameID := msg.GameID
		return m, func() tea.Msg {
			hub.SpectateGame(context.Background(), client, gameID)
			return nil
		}, true
	case screens.CreateGameMsg:
		hub, client := m.hub, m.client
		tc := msg.TimeControl
		m.screen = screens.ScreenLobby
		return m, func() tea.Msg {
			hub.CreateGame(context.Background(), client, tc)
			return nil
		}, true
	case screens.MakeMoveMsg:
		hub, client := m.hub, m.client
		san := msg.SAN
		return m, func() tea.Msg {
			hub.MakeMove(context.Background(), client, san)
			return nil
		}, true
	case screens.ResignMsg:
		hub, client := m.hub, m.client
		return m, func() tea.Msg {
			hub.Resign(context.Background(), client)
			return nil
		}, true
	case screens.RequestUndoMsg:
		hub, client := m.hub, m.client
		return m, func() tea.Msg {
			hub.RequestUndo(client)
			return nil
		}, true
	case screens.RespondUndoMsg:
		hub, client := m.hub, m.client
		accept := msg.Accept
		return m, func() tea.Msg {
			hub.RespondUndo(context.Background(), client, accept)
			return nil
		}, true
	case screens.SubmitPuzzleAttemptMsg:
		hub := m.hub
		username := m.user.Username
		puzzleID, solved, skipped := msg.PuzzleID, msg.Solved, msg.Skipped
		return m, func() tea.Msg {
			newRating, err := hub.RecordPuzzleAttempt(context.Background(), username, puzzleID, solved, skipped)
			if skipped {
				return screens.ScreenChangeMsg{Screen: screens.ScreenPuzzle}
			}
			if err != nil {
				return screens.PuzzleAttemptMsg{Err: err}
			}
			return screens.PuzzleAttemptMsg{NewRating: newRating}
		}, true
	case screens.CheckUsernamesActionMsg:
		hub := m.hub
		white, black := msg.White, msg.Black
		return m, func() tea.Msg {
			var unknown []string
			for _, name := range []string{white, black} {
				exists, err := hub.CheckUsername(context.Background(), name)
				if err != nil {
					return screens.ErrMsg{Err: err}
				}
				if !exists {
					unknown = append(unknown, name)
				}
			}
			return screens.UsernameCheckDoneMsg{Unknown: unknown}
		}, true
	case screens.SaveImportedActionMsg:
		hub := m.hub
		white, black, pgn, forceCreate := msg.White, msg.Black, msg.PGN, msg.ForceCreate
		return m, func() tea.Msg {
			if err := hub.SaveImportedGame(context.Background(), white, black, pgn, forceCreate); err != nil {
				return screens.ErrMsg{Err: err}
			}
			return screens.SaveImportedDoneMsg{}
		}, true
	}
	return m, nil, false
}

func (m Model) handleDataLoadedMsg(msg tea.Msg) (Model, bool) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		if m.history != nil {
			m.history.SetGames(msg.records)
		}
		return m, true
	case leaderboardLoadedMsg:
		if m.leaderboard != nil {
			m.leaderboard.SetPlayers(msg.players)
		}
		return m, true
	case importedGamesLoadedMsg:
		if m.importedGames != nil {
			m.importedGames.SetGames(msg.records)
		}
		return m, true
	case puzzleLoadedMsg:
		if m.puzzle != nil {
			m.puzzle.SetPuzzle(msg.record)
		}
		return m, true
	}
	return m, false
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case shared.LobbyState:
		if m.lobby != nil {
			m.lobby.SetState(msg)
		}
		return m, m.client.Recv()
	case shared.ErrorMsg:
		return m.handleServerError(msg)
	case screens.ScreenChangeMsg:
		return m.handleScreenChange(msg)
	}
	if m2, cmd, ok := m.handleGameLifecycleMsg(msg); ok {
		return m2, cmd
	}
	if m2, cmd, ok := m.handleHubActionMsg(msg); ok {
		return m2, cmd
	}
	if m2, ok := m.handleDataLoadedMsg(msg); ok {
		return m2, nil
	}
	return m.updateActiveScreen(msg)
}

func (m Model) updateSecondaryScreen(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch m.screen {
	case screens.ScreenHistory:
		updated, cmd := m.history.Update(msg)
		if hm, ok := updated.(*screens.HistoryModel); ok {
			m.history = hm
		}
		return m, cmd, true
	case screens.ScreenLeaderboard:
		updated, cmd := m.leaderboard.Update(msg)
		if lm, ok := updated.(*screens.LeaderboardModel); ok {
			m.leaderboard = lm
		}
		return m, cmd, true
	case screens.ScreenImportedGames:
		updated, cmd := m.importedGames.Update(msg)
		if ig, ok := updated.(*screens.ImportedGamesModel); ok {
			m.importedGames = ig
		}
		return m, cmd, true
	case screens.ScreenCreateGame:
		updated, cmd := m.createGame.Update(msg)
		if cgm, ok := updated.(*screens.CreateGameModel); ok {
			m.createGame = cgm
		}
		return m, cmd, true
	case screens.ScreenLobby, screens.ScreenGame, screens.ScreenReplay, screens.ScreenImport, screens.ScreenPuzzle:
		return m, nil, false
	}
	return m, nil, false
}

func (m Model) updateActiveScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m2, cmd, ok := m.updateSecondaryScreen(msg); ok {
		return m2, cmd
	}
	switch m.screen {
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
	case screens.ScreenReplay:
		updated, cmd := m.replay.Update(msg)
		if rm, ok := updated.(*screens.ReplayModel); ok {
			m.replay = rm
		}
		return m, cmd
	case screens.ScreenPuzzle:
		updated, cmd := m.puzzle.Update(msg)
		if pm, ok := updated.(*screens.PuzzleModel); ok {
			m.puzzle = pm
		}
		return m, cmd
	case screens.ScreenImport:
		updated, cmd := m.importScreen.Update(msg)
		if im, ok := updated.(*screens.ImportModel); ok {
			m.importScreen = im
		}
		return m, cmd
	case screens.ScreenHistory, screens.ScreenLeaderboard, screens.ScreenImportedGames, screens.ScreenCreateGame:
		return m, nil
	}
	return m, nil
}

func (m Model) handleScreenChange(msg screens.ScreenChangeMsg) (tea.Model, tea.Cmd) {
	switch msg.Screen {
	case screens.ScreenLobby:
		if m.lobby == nil {
			m.lobby = screens.NewLobbyModel(m.user.Username)
		}
		m.screen = screens.ScreenLobby
	case screens.ScreenHistory:
		m.history = screens.NewHistoryModel(m.user.Username)
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
		m.leaderboard = screens.NewLeaderboardModel()
		m.screen = screens.ScreenLeaderboard
		return m, m.fetchLeaderboard()
	case screens.ScreenPuzzle:
		m.puzzle = screens.NewPuzzleModel(m.user.Username)
		m.screen = screens.ScreenPuzzle
		return m, m.fetchPuzzle()
	case screens.ScreenGame:
		m.screen = screens.ScreenGame
	case screens.ScreenCreateGame:
		m.createGame = screens.NewCreateGameModel()
		m.screen = screens.ScreenCreateGame
	}
	return m, nil
}

func (m Model) viewSecondaryScreen() string {
	switch m.screen {
	case screens.ScreenHistory:
		if m.history != nil {
			return m.history.View()
		}
	case screens.ScreenLeaderboard:
		if m.leaderboard != nil {
			return m.leaderboard.View()
		}
	case screens.ScreenImportedGames:
		if m.importedGames != nil {
			return m.importedGames.View()
		}
	case screens.ScreenCreateGame:
		if m.createGame != nil {
			return m.createGame.View()
		}
	case screens.ScreenLobby, screens.ScreenGame, screens.ScreenReplay, screens.ScreenImport, screens.ScreenPuzzle:
	}
	return ""
}

func (m Model) viewForScreen() string {
	switch m.screen {
	case screens.ScreenLobby:
		if m.lobby != nil {
			return m.lobby.View()
		}
	case screens.ScreenGame:
		if m.game != nil {
			return m.game.View()
		}
	case screens.ScreenReplay:
		if m.replay != nil {
			return m.replay.View()
		}
	case screens.ScreenPuzzle:
		if m.puzzle != nil {
			return m.puzzle.View()
		}
	case screens.ScreenImport:
		if m.importScreen != nil {
			return m.importScreen.View()
		}
	case screens.ScreenHistory, screens.ScreenLeaderboard, screens.ScreenImportedGames, screens.ScreenCreateGame:
	}
	return m.viewSecondaryScreen()
}

// View implements tea.Model.
func (m Model) View() string {
	return m.viewForScreen()
}

type errString string

func (e errString) Error() string { return string(e) }

func (m Model) fetchHistory() tea.Cmd {
	hub := m.hub
	username := m.user.Username
	return func() tea.Msg {
		records, err := hub.GetHistory(context.Background(), username)
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		return historyLoadedMsg{records: toSharedHistoryRecords(records)}
	}
}

func (m Model) fetchLeaderboard() tea.Cmd {
	hub := m.hub
	return func() tea.Msg {
		users, err := hub.GetLeaderboard(context.Background())
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		return leaderboardLoadedMsg{players: toSharedPlayers(users)}
	}
}

func (m Model) fetchPuzzle() tea.Cmd {
	hub := m.hub
	username := m.user.Username
	return func() tea.Msg {
		p, err := hub.GetPuzzle(context.Background(), username)
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		if p == nil {
			return screens.ErrMsg{Err: errString("no puzzles available")}
		}
		return puzzleLoadedMsg{record: *p}
	}
}

func (m Model) fetchImportedGames() tea.Cmd {
	hub := m.hub
	return func() tea.Msg {
		records, err := hub.GetImportedGames(context.Background())
		if err != nil {
			return screens.ErrMsg{Err: err}
		}
		return importedGamesLoadedMsg{records: toSharedHistoryRecords(records)}
	}
}

func toSharedHistoryRecords(records []server.HistoryRecord) []shared.HistoryRecord {
	result := make([]shared.HistoryRecord, len(records))
	for i := range records {
		r := &records[i]
		result[i] = shared.HistoryRecord{
			ID:             r.ID,
			White:          r.White,
			Black:          r.Black,
			Result:         r.Result,
			WhiteEloBefore: r.WhiteEloBefore,
			BlackEloBefore: r.BlackEloBefore,
			WhiteEloAfter:  r.WhiteEloAfter,
			BlackEloAfter:  r.BlackEloAfter,
			PGN:            r.PGN,
			PlayedAt:       r.PlayedAt,
			Imported:       r.Imported,
		}
	}
	return result
}

func toSharedPlayers(users []server.User) []shared.PlayerInfo {
	result := make([]shared.PlayerInfo, len(users))
	for i, u := range users {
		result[i] = shared.PlayerInfo{Username: u.Username, Elo: u.Elo, Online: true}
	}
	return result
}
