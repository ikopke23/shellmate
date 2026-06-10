package client

import (
	"testing"

	"github.com/ikopke/shellmate/internal/client/screens"
	"github.com/ikopke/shellmate/internal/server"
	"github.com/ikopke/shellmate/internal/shared"
)

func modelWithBoardRows(rows int) Model {
	hub := &server.Hub{}
	c := &server.Client{}
	user := &server.User{Username: "alice", Elo: 1500, BoardRows: rows}
	return NewModel(hub, c, user, 80, 24)
}

func TestNewModel_ClampsBoardRows(t *testing.T) {
	cases := []struct{ stored, want int }{
		{0, 3}, {1, 3}, {2, 2}, {5, 5}, {8, 8}, {99, 3},
	}
	for _, tc := range cases {
		m := modelWithBoardRows(tc.stored)
		if m.boardRows != tc.want {
			t.Errorf("stored %d: expected boardRows %d, got %d", tc.stored, tc.want, m.boardRows)
		}
	}
}

func TestModel_BoardResizeMsgUpdatesBoardRows(t *testing.T) {
	m := modelWithBoardRows(3)
	updated, _ := m.Update(screens.BoardResizeMsg{Rows: 6})
	um := updated.(Model)
	if um.boardRows != 6 {
		t.Fatalf("expected boardRows 6, got %d", um.boardRows)
	}
}

func TestModel_NewGameInheritsBoardRows(t *testing.T) {
	m := modelWithBoardRows(3)
	m.boardRows = 7
	updated, _ := m.Update(shared.GameStart{GameID: "g1", White: "alice", Black: "bob"})
	um := updated.(Model)
	if um.game == nil {
		t.Fatal("expected game to be constructed")
	}
	if got := um.game.BoardCellRows(); got != 7 {
		t.Fatalf("expected game board rows 7, got %d", got)
	}
}

func TestModel_NewPuzzleInheritsBoardRows(t *testing.T) {
	m := modelWithBoardRows(3)
	m.boardRows = 7
	updated, _ := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenPuzzle})
	um := updated.(Model)
	if um.puzzle == nil {
		t.Fatal("expected puzzle to be constructed")
	}
	if got := um.puzzle.BoardCellRows(); got != 7 {
		t.Fatalf("expected puzzle board rows 7, got %d", got)
	}
}

func TestModel_NewReplayInheritsBoardRows(t *testing.T) {
	m := modelWithBoardRows(3)
	m.boardRows = 7
	updated, _ := m.Update(screens.ScreenChangeMsg{Screen: screens.ScreenReplay})
	um := updated.(Model)
	if um.replay == nil {
		t.Fatal("expected replay to be constructed")
	}
	if got := um.replay.BoardCellRows(); got != 7 {
		t.Fatalf("expected replay board rows 7, got %d", got)
	}
}
