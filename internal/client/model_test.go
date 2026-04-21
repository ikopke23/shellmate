package client

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/client/screens"
	"github.com/ikopke/shellmate/internal/server"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

func setupModel(t *testing.T) Model {
	t.Helper()
	hub := &server.Hub{}
	c := &server.Client{}
	user := &server.User{Username: "alice", Elo: 1500}
	return NewModel(hub, c, user, 80, 24)
}

// --- Constructor, Init, View ---

func TestModel_NewModel_DefaultsToLobby(t *testing.T) {
	m := setupModel(t)
	if m.screen != screens.ScreenLobby {
		t.Errorf("screen = %v, want ScreenLobby", m.screen)
	}
	if m.lobby == nil {
		t.Error("lobby is nil, want non-nil")
	}
	if m.game != nil {
		t.Error("game should be nil on fresh model")
	}
	if m.history != nil {
		t.Error("history should be nil on fresh model")
	}
	if m.replay != nil {
		t.Error("replay should be nil on fresh model")
	}
	if m.leaderboard != nil {
		t.Error("leaderboard should be nil on fresh model")
	}
	if m.importScreen != nil {
		t.Error("importScreen should be nil on fresh model")
	}
	if m.importedGames != nil {
		t.Error("importedGames should be nil on fresh model")
	}
	if m.puzzle != nil {
		t.Error("puzzle should be nil on fresh model")
	}
	if m.createGame != nil {
		t.Error("createGame should be nil on fresh model")
	}
}

func TestModel_Init_ReturnsBatchCmd(t *testing.T) {
	m := setupModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
}

func TestModel_View_DelegatesToLobby(t *testing.T) {
	m := setupModel(t)
	out := m.View()
	if out == "" {
		t.Error("View() returned empty string")
	}
}

// --- Top-level Update routing ---

func TestModel_Update_WindowSizeMsg_StoresDimensions(t *testing.T) {
	m := setupModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.width != 100 || m2.height != 50 {
		t.Errorf("dims = (%d,%d), want (100,50)", m2.width, m2.height)
	}
}

func TestModel_Update_LobbyState_DelegatesToLobby(t *testing.T) {
	m := setupModel(t)
	next, cmd := m.Update(shared.LobbyState{
		Players: []shared.PlayerInfo{{Username: "alice", Elo: 1500, Online: true}},
		Games:   []shared.GameInfo{{ID: "g1", White: "alice"}},
	})
	if cmd == nil {
		t.Error("LobbyState returned nil cmd, want client.Recv()")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.lobby == nil {
		t.Error("lobby should still be non-nil after LobbyState")
	}
}

func TestModel_Update_ErrorMsg_RoutesToHandleServerError(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(shared.ErrorMsg{Message: "x"})
	if cmd == nil {
		t.Error("ErrorMsg returned nil cmd, want client.Recv()")
	}
}

// --- handleGameLifecycleMsg ---

func TestModel_GameStart_CreatesGameAndSwitchesScreen(t *testing.T) {
	m := setupModel(t)
	next, cmd := m.Update(shared.GameStart{
		GameID: "g1", White: "alice", Black: "bob",
		TimeControl: shared.TimeControl{},
	})
	if cmd == nil {
		t.Error("GameStart returned nil cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenGame {
		t.Errorf("screen = %v, want ScreenGame", m2.screen)
	}
	if m2.game == nil {
		t.Fatal("game should be non-nil after GameStart")
	}
}

func TestModel_MoveMsg_UpdatesGameMoves(t *testing.T) {
	m := setupModel(t)
	m.game = screens.NewGameModel("g1", "alice", "bob", chess.White, "alice", shared.TimeControl{})
	m.screen = screens.ScreenGame
	next, cmd := m.Update(shared.MoveMsg{GameID: "g1", Moves: []string{"e4"}})
	if cmd == nil {
		t.Error("MoveMsg returned nil cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.game == nil {
		t.Error("game should still be non-nil after MoveMsg")
	}
	if m2.screen != screens.ScreenGame {
		t.Errorf("screen = %v, want ScreenGame", m2.screen)
	}
}

func TestModel_GameOver_SetsGameOverState(t *testing.T) {
	m := setupModel(t)
	m.game = screens.NewGameModel("g1", "alice", "bob", chess.White, "alice", shared.TimeControl{})
	m.screen = screens.ScreenGame
	next, cmd := m.Update(shared.GameOver{GameID: "g1", Result: "1-0", WhiteEloAfter: 1510, BlackEloAfter: 1490})
	if cmd == nil {
		t.Error("GameOver returned nil cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.game == nil {
		t.Error("game should be non-nil after GameOver")
	}
}

func TestModel_UndoRequest_SetsPendingPrompt(t *testing.T) {
	m := setupModel(t)
	m.game = screens.NewGameModel("g1", "alice", "bob", chess.White, "alice", shared.TimeControl{})
	m.screen = screens.ScreenGame
	next, cmd := m.Update(shared.UndoRequest{GameID: "g1"})
	if cmd == nil {
		t.Error("UndoRequest returned nil cmd")
	}
	if _, ok := next.(Model); !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
}

func TestModel_UndoResponse_NotAccepted_ClearsPending(t *testing.T) {
	m := setupModel(t)
	m.game = screens.NewGameModel("g1", "alice", "bob", chess.White, "alice", shared.TimeControl{})
	m.screen = screens.ScreenGame
	next, cmd := m.Update(shared.UndoResponse{GameID: "g1", Accept: false})
	if cmd == nil {
		t.Error("UndoResponse returned nil cmd")
	}
	if _, ok := next.(Model); !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
}

func TestModel_UndoAccepted_UpdatesMoves(t *testing.T) {
	m := setupModel(t)
	m.game = screens.NewGameModel("g1", "alice", "bob", chess.White, "alice", shared.TimeControl{})
	m.screen = screens.ScreenGame
	next, cmd := m.Update(shared.UndoAccepted{GameID: "g1", Moves: []string{}})
	if cmd == nil {
		t.Error("UndoAccepted returned nil cmd")
	}
	if _, ok := next.(Model); !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
}

// --- handleHubActionMsg (assert non-nil cmd, do NOT execute) ---

func TestModel_JoinGameMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.JoinGameMsg{GameID: "g1"})
	if cmd == nil {
		t.Fatal("JoinGameMsg returned nil cmd")
	}
}

func TestModel_SpectateGameMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.SpectateGameMsg{GameID: "g1"})
	if cmd == nil {
		t.Fatal("SpectateGameMsg returned nil cmd")
	}
}

func TestModel_CreateGameMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.CreateGameMsg{TimeControl: shared.TimeControl{InitialSeconds: 300}})
	if cmd == nil {
		t.Fatal("CreateGameMsg returned nil cmd")
	}
}

func TestModel_MakeMoveMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.MakeMoveMsg{SAN: "e4"})
	if cmd == nil {
		t.Fatal("MakeMoveMsg returned nil cmd")
	}
}

func TestModel_ResignMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.ResignMsg{})
	if cmd == nil {
		t.Fatal("ResignMsg returned nil cmd")
	}
}

func TestModel_RequestUndoMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.RequestUndoMsg{})
	if cmd == nil {
		t.Fatal("RequestUndoMsg returned nil cmd")
	}
}

func TestModel_RespondUndoMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.RespondUndoMsg{Accept: true})
	if cmd == nil {
		t.Fatal("RespondUndoMsg returned nil cmd")
	}
}

func TestModel_SubmitPuzzleAttemptMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.SubmitPuzzleAttemptMsg{PuzzleID: "p1", Solved: true})
	if cmd == nil {
		t.Fatal("SubmitPuzzleAttemptMsg returned nil cmd")
	}
}

func TestModel_CheckUsernamesActionMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.CheckUsernamesActionMsg{White: "alice", Black: "bob"})
	if cmd == nil {
		t.Fatal("CheckUsernamesActionMsg returned nil cmd")
	}
}

func TestModel_SaveImportedActionMsg_ReturnsNonNilCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(screens.SaveImportedActionMsg{White: "alice", Black: "bob", PGN: "1. e4"})
	if cmd == nil {
		t.Fatal("SaveImportedActionMsg returned nil cmd")
	}
}

// --- handleDataLoadedMsg ---

func TestModel_HistoryLoadedMsg_PopulatesHistoryScreen(t *testing.T) {
	m := setupModel(t)
	m.history = screens.NewHistoryModel("alice")
	next, _ := m.Update(historyLoadedMsg{records: []shared.HistoryRecord{{ID: "g1", White: "alice", Black: "bob"}}})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.history == nil {
		t.Error("history should be non-nil after historyLoadedMsg")
	}
}

func TestModel_LeaderboardLoadedMsg_PopulatesLeaderboard(t *testing.T) {
	m := setupModel(t)
	m.leaderboard = screens.NewLeaderboardModel()
	next, _ := m.Update(leaderboardLoadedMsg{players: []shared.PlayerInfo{{Username: "alice", Elo: 1500, Online: true}}})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.leaderboard == nil {
		t.Error("leaderboard should be non-nil after leaderboardLoadedMsg")
	}
}

func TestModel_ImportedGamesLoadedMsg_PopulatesImportedGames(t *testing.T) {
	m := setupModel(t)
	m.importedGames = screens.NewImportedGamesModel()
	next, _ := m.Update(importedGamesLoadedMsg{records: []shared.HistoryRecord{{ID: "g1"}}})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.importedGames == nil {
		t.Error("importedGames should be non-nil after importedGamesLoadedMsg")
	}
}

func TestModel_PuzzleLoadedMsg_PopulatesPuzzle(t *testing.T) {
	m := setupModel(t)
	m.puzzle = screens.NewPuzzleModel("alice")
	next, _ := m.Update(puzzleLoadedMsg{record: shared.PuzzleRecord{ID: "p1", FEN: "startpos", Moves: "e4"}})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.puzzle == nil {
		t.Error("puzzle should be non-nil after puzzleLoadedMsg")
	}
}

// --- handleScreenChange ---

func TestModel_ScreenChange_ToHistory_CreatesScreenAndReturnsFetchCmd(t *testing.T) {
	m := setupModel(t)
	next, cmd := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenHistory})
	if cmd == nil {
		t.Error("ScreenHistory change returned nil cmd, want fetch cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenHistory {
		t.Errorf("screen = %v, want ScreenHistory", m2.screen)
	}
	if m2.history == nil {
		t.Error("history should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToImportedGames_CreatesScreenAndReturnsFetchCmd(t *testing.T) {
	m := setupModel(t)
	next, cmd := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenImportedGames})
	if cmd == nil {
		t.Error("ScreenImportedGames change returned nil cmd, want fetch cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenImportedGames {
		t.Errorf("screen = %v, want ScreenImportedGames", m2.screen)
	}
	if m2.importedGames == nil {
		t.Error("importedGames should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToLeaderboard_ReturnsFetchCmd(t *testing.T) {
	m := setupModel(t)
	next, cmd := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenLeaderboard})
	if cmd == nil {
		t.Error("ScreenLeaderboard change returned nil cmd, want fetch cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenLeaderboard {
		t.Errorf("screen = %v, want ScreenLeaderboard", m2.screen)
	}
	if m2.leaderboard == nil {
		t.Error("leaderboard should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToPuzzle_ReturnsFetchCmd(t *testing.T) {
	m := setupModel(t)
	next, cmd := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenPuzzle})
	if cmd == nil {
		t.Error("ScreenPuzzle change returned nil cmd, want fetch cmd")
	}
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenPuzzle {
		t.Errorf("screen = %v, want ScreenPuzzle", m2.screen)
	}
	if m2.puzzle == nil {
		t.Error("puzzle should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToReplay_WithHistoryRecord_LoadsPGN(t *testing.T) {
	m := setupModel(t)
	next, _ := m.Update(screens.ScreenChangeMsg{
		Screen: screens.ScreenReplay,
		Data:   shared.HistoryRecord{PGN: "1. e4 e5", White: "alice", Black: "bob", PlayedAt: time.Now()},
	})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenReplay {
		t.Errorf("screen = %v, want ScreenReplay", m2.screen)
	}
	if m2.replay == nil {
		t.Error("replay should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToReplay_WithImportPGNData_LoadsPGN(t *testing.T) {
	m := setupModel(t)
	next, _ := m.Update(screens.ScreenChangeMsg{
		Screen: screens.ScreenReplay,
		Data:   screens.ImportPGNData{Record: shared.HistoryRecord{PGN: "1. e4 e5"}},
	})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenReplay {
		t.Errorf("screen = %v, want ScreenReplay", m2.screen)
	}
	if m2.replay == nil {
		t.Error("replay should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToReplay_WithImportedGamesOpenData_LoadsPGN(t *testing.T) {
	m := setupModel(t)
	next, _ := m.Update(screens.ScreenChangeMsg{
		Screen: screens.ScreenReplay,
		Data:   screens.ImportedGamesOpenData{Record: shared.HistoryRecord{PGN: "1. e4 e5", White: "alice", Black: "bob", PlayedAt: time.Now()}},
	})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenReplay {
		t.Errorf("screen = %v, want ScreenReplay", m2.screen)
	}
	if m2.replay == nil {
		t.Error("replay should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToCreateGame_CreatesScreen(t *testing.T) {
	m := setupModel(t)
	next, _ := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenCreateGame})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenCreateGame {
		t.Errorf("screen = %v, want ScreenCreateGame", m2.screen)
	}
	if m2.createGame == nil {
		t.Error("createGame should be non-nil after screen change")
	}
}

func TestModel_ScreenChange_ToImport_CreatesScreen(t *testing.T) {
	m := setupModel(t)
	next, _ := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenImport})
	m2, ok := next.(Model)
	if !ok {
		t.Fatalf("type assertion failed: %T", next)
	}
	if m2.screen != screens.ScreenImport {
		t.Errorf("screen = %v, want ScreenImport", m2.screen)
	}
	if m2.importScreen == nil {
		t.Error("importScreen should be non-nil after screen change")
	}
}

// --- Pure converters ---

func TestToSharedHistoryRecords(t *testing.T) {
	playedAt := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	input := []server.HistoryRecord{
		{ID: "g1", White: "alice", Black: "bob", Result: "1-0", WhiteEloBefore: 1500, BlackEloBefore: 1500, WhiteEloAfter: 1510, BlackEloAfter: 1490, PGN: "1. e4", PlayedAt: playedAt, Imported: false},
		{ID: "g2", White: "carol", Black: "dave", Result: "0-1", PGN: "1. d4", PlayedAt: playedAt, Imported: true},
	}
	got := toSharedHistoryRecords(input)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "g1" || got[0].White != "alice" || got[0].Black != "bob" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[0].Result != "1-0" || got[0].WhiteEloBefore != 1500 || got[0].BlackEloAfter != 1490 {
		t.Errorf("got[0] elo/result = %+v", got[0])
	}
	if got[0].PGN != "1. e4" || !got[0].PlayedAt.Equal(playedAt) || got[0].Imported {
		t.Errorf("got[0] pgn/date/imported = %+v", got[0])
	}
	if got[1].ID != "g2" || !got[1].Imported {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestToSharedPlayers_SetsOnlineTrue(t *testing.T) {
	input := []server.User{
		{Username: "alice", Elo: 1500},
		{Username: "bob", Elo: 1600},
	}
	got := toSharedPlayers(input)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for i, p := range got {
		if !p.Online {
			t.Errorf("got[%d].Online = false, want true", i)
		}
	}
	if got[0].Username != "alice" || got[0].Elo != 1500 {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Username != "bob" || got[1].Elo != 1600 {
		t.Errorf("got[1] = %+v", got[1])
	}
}
