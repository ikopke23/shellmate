package server

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// makeGame creates an untimed two-player game with buffered client channels.
func makeGame(t *testing.T) (*Game, *Client, *Client) {
	t.Helper()
	white := &Client{username: "white", send: make(chan tea.Msg, 256), done: make(chan struct{})}
	black := &Client{username: "black", send: make(chan tea.Msg, 256), done: make(chan struct{})}
	return NewGame("test-game", white, black, 0, 0), white, black
}

// drainMsg reads one message from c with a short timeout; fails if nothing arrives.
func drainMsg(t *testing.T, c *Client) tea.Msg {
	t.Helper()
	select {
	case msg := <-c.send:
		return msg
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected message on client channel, got none")
		return nil
	}
}

// noMsg asserts no message arrives within a short window.
func noMsg(t *testing.T, c *Client) {
	t.Helper()
	select {
	case msg := <-c.send:
		t.Fatalf("expected no message but got %T", msg)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestGame_RemoveSpectator(t *testing.T) {
	g, _, _ := makeGame(t)
	spec := &Client{username: "spec", send: make(chan tea.Msg, 4), done: make(chan struct{})}
	g.mu.Lock()
	g.spectators = append(g.spectators, spec)
	g.mu.Unlock()
	g.RemoveSpectator(spec)
	g.mu.Lock()
	remaining := len(g.spectators)
	g.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected 0 spectators after removal, got %d", remaining)
	}
	// removing again should not panic
	g.RemoveSpectator(spec)
}

func TestGame_RequestUndo_NoMoves(t *testing.T) {
	g, white, _ := makeGame(t)
	err := g.RequestUndo(white)
	if err == nil {
		t.Fatal("expected error requesting undo with no moves")
	}
}

func TestGame_RequestUndo_SetsState(t *testing.T) {
	g, white, _ := makeGame(t)
	if err := g.ApplyMove(white, "e4"); err != nil {
		t.Fatal(err)
	}
	if err := g.RequestUndo(white); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g.mu.Lock()
	pending := g.pendingUndo
	g.mu.Unlock()
	if pending != "white" {
		t.Fatalf("pendingUndo = %q, want %q", pending, "white")
	}
}

func TestGame_RequestUndo_AlreadyPending(t *testing.T) {
	g, white, _ := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	_ = g.RequestUndo(white)
	if err := g.RequestUndo(white); err == nil {
		t.Fatal("expected error on double undo request")
	}
}

func TestGame_RequestUndo_Spectator(t *testing.T) {
	g, white, _ := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	spec := &Client{username: "spec"}
	if err := g.RequestUndo(spec); err == nil {
		t.Fatal("expected error when spectator requests undo")
	}
}

func TestGame_AcceptUndo_NoPending(t *testing.T) {
	g, _, black := makeGame(t)
	if err := g.AcceptUndo(black); err == nil {
		t.Fatal("expected error accepting undo with no pending request")
	}
}

func TestGame_AcceptUndo_OwnRequest(t *testing.T) {
	g, white, _ := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	_ = g.RequestUndo(white)
	if err := g.AcceptUndo(white); err == nil {
		t.Fatal("expected error when requester accepts own undo")
	}
}

func TestGame_AcceptUndo_RemovesLastMove(t *testing.T) {
	g, white, black := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	_ = g.ApplyMove(black, "e5")
	_ = g.RequestUndo(black) // black requests undo of "e5"
	if err := g.AcceptUndo(white); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	moveCount := len(g.chess.Moves())
	if moveCount != 1 {
		t.Fatalf("expected 1 move after undo, got %d", moveCount)
	}
	g.mu.Lock()
	pending := g.pendingUndo
	g.mu.Unlock()
	if pending != "" {
		t.Fatalf("pendingUndo should be cleared after accept, got %q", pending)
	}
}

func TestGame_RejectUndo_NoPending(t *testing.T) {
	g, _, black := makeGame(t)
	if err := g.RejectUndo(black); err == nil {
		t.Fatal("expected error rejecting undo with no pending request")
	}
}

func TestGame_RejectUndo_OwnRequest(t *testing.T) {
	g, white, _ := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	_ = g.RequestUndo(white)
	if err := g.RejectUndo(white); err == nil {
		t.Fatal("expected error when requester rejects own undo")
	}
}

func TestGame_RejectUndo_ClearsPending(t *testing.T) {
	g, white, black := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	_ = g.RequestUndo(white)
	if err := g.RejectUndo(black); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g.mu.Lock()
	pending := g.pendingUndo
	g.mu.Unlock()
	if pending != "" {
		t.Fatalf("pendingUndo should be cleared after reject, got %q", pending)
	}
}

func TestGame_Broadcast(t *testing.T) {
	g, white, black := makeGame(t)
	spec := &Client{username: "spec", send: make(chan tea.Msg, 4), done: make(chan struct{})}
	g.mu.Lock()
	g.spectators = append(g.spectators, spec)
	g.mu.Unlock()
	msg := shared.ErrorMsg{Message: "test"}
	g.Broadcast(msg)
	for _, c := range []*Client{white, black, spec} {
		got := drainMsg(t, c)
		if e, ok := got.(shared.ErrorMsg); !ok || e.Message != "test" {
			t.Fatalf("client %s: got %T, want shared.ErrorMsg{test}", c.username, got)
		}
	}
}

func TestGame_BroadcastMove_SendsMoveMsg(t *testing.T) {
	g, white, black := makeGame(t)
	if err := g.ApplyMove(white, "e4"); err != nil {
		t.Fatal(err)
	}
	g.BroadcastMove()
	for _, c := range []*Client{white, black} {
		got := drainMsg(t, c)
		mm, ok := got.(shared.MoveMsg)
		if !ok {
			t.Fatalf("client %s: got %T, want shared.MoveMsg", c.username, got)
		}
		if len(mm.Moves) != 1 || mm.Moves[0] != "e4" {
			t.Fatalf("client %s: moves = %v, want [e4]", c.username, mm.Moves)
		}
	}
}

func TestGame_BroadcastUndoAccepted(t *testing.T) {
	g, white, black := makeGame(t)
	_ = g.ApplyMove(white, "e4")
	_ = g.ApplyMove(black, "e5")
	_ = g.RequestUndo(black)
	_ = g.AcceptUndo(white)
	g.BroadcastUndoAccepted()
	for _, c := range []*Client{white, black} {
		got := drainMsg(t, c)
		ua, ok := got.(shared.UndoAccepted)
		if !ok {
			t.Fatalf("client %s: got %T, want shared.UndoAccepted", c.username, got)
		}
		if len(ua.Moves) != 1 || ua.Moves[0] != "e4" {
			t.Fatalf("client %s: moves = %v, want [e4]", c.username, ua.Moves)
		}
	}
}

func TestGame_IsOver(t *testing.T) {
	g, _, _ := makeGame(t)
	if g.IsOver() {
		t.Fatal("new game should not be over")
	}
	g.mu.Lock()
	g.chess.Resign(chess.White)
	g.mu.Unlock()
	if !g.IsOver() {
		t.Fatal("game should be over after resignation")
	}
}

func TestGame_stopClock_Idempotent(t *testing.T) {
	g := makeTimedGame(30, 0)
	// clockTimer is nil until resetClock is called; stopClock on nil timer should not panic
	g.stopClock()
	g.stopClock()
	g.mu.Lock()
	timer := g.clockTimer
	g.mu.Unlock()
	if timer != nil {
		t.Fatal("expected clockTimer == nil after stopClock")
	}
}

func TestGame_stopClock_StopsActiveTimer(t *testing.T) {
	hub := &Hub{games: make(map[string]*Game)}
	g := makeTimedGame(30, 0)
	hub.games[g.id] = g
	g.resetClock(hub) // starts the timer
	g.mu.Lock()
	timerBefore := g.clockTimer
	g.mu.Unlock()
	if timerBefore == nil {
		t.Fatal("expected active timer after resetClock")
	}
	g.stopClock()
	g.mu.Lock()
	timerAfter := g.clockTimer
	g.mu.Unlock()
	if timerAfter != nil {
		t.Fatal("expected nil timer after stopClock")
	}
}

func TestGame_flagPlayer_ResignsAndRemovesFromHub(t *testing.T) {
	hub := &Hub{games: make(map[string]*Game), clients: make(map[string]*Client)}
	g := makeTimedGame(30, 0)
	gen := g.clockGeneration
	// Do NOT put game in hub.games so handleGameOver/BroadcastLobby are skipped
	g.flagPlayer(hub, chess.White, gen)
	if g.chess.Outcome() == chess.NoOutcome {
		t.Fatal("expected game resigned after flagPlayer")
	}
}

func TestGame_flagPlayer_StaleGenIgnored(t *testing.T) {
	hub := &Hub{games: make(map[string]*Game), clients: make(map[string]*Client)}
	g := makeTimedGame(30, 0)
	staleGen := g.clockGeneration - 1 // stale generation
	g.flagPlayer(hub, chess.White, staleGen)
	if g.chess.Outcome() != chess.NoOutcome {
		t.Fatal("flagPlayer with stale generation should not resign the game")
	}
}

func TestGame_BroadcastMove_NilSpectators(t *testing.T) {
	g, white, black := makeGame(t)
	// No moves played yet; BroadcastMove should still work (empty move list)
	_ = g.ApplyMove(white, "e4")
	g.BroadcastMove()
	drainMsg(t, white)
	drainMsg(t, black)
	noMsg(t, white)
}
