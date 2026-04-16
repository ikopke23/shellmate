package server

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
)

// newTestHub creates a Hub with nil DB. Safe when h.clients stays empty
// (BroadcastLobby only calls h.db.GetUser per entry in h.clients).
func newTestHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
		games:   make(map[string]*Game),
	}
}

// newTestClient creates a client with buffered channels but does NOT register it in the hub.
func newTestClient(hub *Hub, username string) *Client {
	return &Client{
		username: username,
		send:     make(chan tea.Msg, 256),
		done:     make(chan struct{}),
		hub:      hub,
	}
}

// drainHub reads one message from c with a 100ms timeout; fails if nothing arrives.
func drainHub(t *testing.T, c *Client) tea.Msg {
	t.Helper()
	select {
	case msg := <-c.send:
		return msg
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected message on client channel, got none")
		return nil
	}
}

// noMsgHub asserts no message arrives within 20ms.
func noMsgHub(t *testing.T, c *Client) {
	t.Helper()
	select {
	case msg := <-c.send:
		t.Fatalf("expected no message but got %T", msg)
	case <-time.After(20 * time.Millisecond):
	}
}

// setupGame manually places an untimed two-player game in hub.games.
func setupGame(h *Hub, gameID string, white, black *Client) *Game {
	g := NewGame(gameID, white, black, 0, 0)
	h.mu.Lock()
	h.games[gameID] = g
	h.mu.Unlock()
	white.game = gameID
	if black != nil {
		black.game = gameID
	}
	return g
}

// ── Register / Unregister ────────────────────────────────────────────────────

func TestHub_Register_Evicts_Existing(t *testing.T) {
	h := newTestHub()
	first := h.Register("alice")
	second := h.Register("alice")
	// first.done should be closed
	select {
	case <-first.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("old client.done not closed after eviction")
	}
	if second == first {
		t.Fatal("expected new client to be different from evicted client")
	}
}

func TestHub_Unregister_Removes_Client(t *testing.T) {
	h := newTestHub()
	c := h.Register("alice")
	h.Unregister(c)
	h.mu.RLock()
	stored := h.clients["alice"]
	h.mu.RUnlock()
	if stored != nil {
		t.Fatal("expected client removed from hub after Unregister")
	}
	select {
	case <-c.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client.done not closed after Unregister")
	}
}

func TestHub_Unregister_Removes_Spectator(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	spec := h.Register("spec")
	g := setupGame(h, "g1", white, black)
	g.mu.Lock()
	g.spectators = append(g.spectators, spec)
	g.mu.Unlock()
	h.Unregister(spec)
	g.mu.Lock()
	n := len(g.spectators)
	g.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 spectators after unregister, got %d", n)
	}
}

// ── CreateGame ───────────────────────────────────────────────────────────────

func TestHub_CreateGame(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	tc := shared.TimeControl{InitialSeconds: 0}
	h.CreateGame(context.Background(), white, tc)
	h.mu.RLock()
	n := len(h.games)
	h.mu.RUnlock()
	if n != 1 {
		t.Fatalf("expected 1 game after CreateGame, got %d", n)
	}
	if white.game == "" {
		t.Fatal("expected white.game to be set after CreateGame")
	}
}

// ── JoinGame ─────────────────────────────────────────────────────────────────

func TestHub_JoinGame_NotFound(t *testing.T) {
	h := newTestHub()
	c := newTestClient(h, "alice")
	h.JoinGame(context.Background(), c, "nonexistent")
	got := drainHub(t, c)
	em, ok := got.(shared.ErrorMsg)
	if !ok || em.Message != errGameNotFound {
		t.Fatalf("got %T %v, want ErrorMsg{game not found}", got, got)
	}
}

func TestHub_JoinGame_Full(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	carol := newTestClient(h, "carol")
	g := NewGame("g1", white, nil, 0, 0)
	h.mu.Lock()
	h.games["g1"] = g
	h.mu.Unlock()
	white.game = "g1"
	h.JoinGame(context.Background(), black, "g1")
	// drain the GameStart from white and black
	drainHub(t, white)
	drainHub(t, black)
	// now try carol
	h.JoinGame(context.Background(), carol, "g1")
	got := drainHub(t, carol)
	em, ok := got.(shared.ErrorMsg)
	if !ok || em.Message != "game is already full" {
		t.Fatalf("got %T %v, want ErrorMsg{game is already full}", got, got)
	}
}

func TestHub_JoinGame_Self(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	g := NewGame("g1", white, nil, 0, 0)
	h.mu.Lock()
	h.games["g1"] = g
	h.mu.Unlock()
	white.game = "g1"
	h.JoinGame(context.Background(), white, "g1")
	got := drainHub(t, white)
	em, ok := got.(shared.ErrorMsg)
	if !ok || em.Message != "cannot join your own game" {
		t.Fatalf("got %T %v, want ErrorMsg{cannot join your own game}", got, got)
	}
}

func TestHub_JoinGame_Success(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	g := NewGame("g1", white, nil, 0, 0)
	h.mu.Lock()
	h.games["g1"] = g
	h.mu.Unlock()
	white.game = "g1"
	h.JoinGame(context.Background(), black, "g1")
	// both should receive GameStart
	for _, c := range []*Client{white, black} {
		msg := drainHub(t, c)
		if _, ok := msg.(shared.GameStart); !ok {
			t.Fatalf("client %s: got %T, want shared.GameStart", c.username, msg)
		}
	}
	g.mu.Lock()
	gotBlack := g.black
	g.mu.Unlock()
	if gotBlack != black {
		t.Fatal("expected g.black == black after JoinGame")
	}
}

// ── closeOpenGamesForUsers ───────────────────────────────────────────────────

func TestHub_CloseOpenGamesForUsers(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "alice")
	// open game by alice (no black player)
	openGame := NewGame("open1", white, nil, 0, 0)
	// started game by alice (has black player) — this should NOT be removed
	black := newTestClient(h, "bob")
	startedGame := NewGame("started1", white, black, 0, 0)
	h.mu.Lock()
	h.games["open1"] = openGame
	h.games["started1"] = startedGame
	h.mu.Unlock()
	h.closeOpenGamesForUsers(context.Background(), "alice", "bob", "started1")
	h.mu.RLock()
	_, openExists := h.games["open1"]
	_, startedExists := h.games["started1"]
	h.mu.RUnlock()
	if openExists {
		t.Fatal("expected open game to be removed")
	}
	if !startedExists {
		t.Fatal("started game should not be removed")
	}
}

// ── SpectateGame ─────────────────────────────────────────────────────────────

func TestHub_SpectateGame_NotFound(t *testing.T) {
	h := newTestHub()
	spec := newTestClient(h, "spec")
	h.SpectateGame(context.Background(), spec, "nonexistent")
	got := drainHub(t, spec)
	em, ok := got.(shared.ErrorMsg)
	if !ok || em.Message != errGameNotFound {
		t.Fatalf("got %T %v, want ErrorMsg{game not found}", got, got)
	}
}

func TestHub_SpectateGame_NotStarted(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	spec := newTestClient(h, "spec")
	g := NewGame("g1", white, nil, 0, 0)
	h.mu.Lock()
	h.games["g1"] = g
	h.mu.Unlock()
	h.SpectateGame(context.Background(), spec, "g1")
	// no GameStart should be sent since game hasn't started
	noMsgHub(t, spec)
	g.mu.Lock()
	n := len(g.spectators)
	g.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 spectator, got %d", n)
	}
}

func TestHub_SpectateGame_Started(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	spec := newTestClient(h, "spec")
	g := NewGame("g1", white, black, 0, 0)
	h.mu.Lock()
	h.games["g1"] = g
	h.mu.Unlock()
	h.SpectateGame(context.Background(), spec, "g1")
	got := drainHub(t, spec)
	if _, ok := got.(shared.GameStart); !ok {
		t.Fatalf("got %T, want shared.GameStart", got)
	}
}

// ── MakeMove ─────────────────────────────────────────────────────────────────

func TestHub_MakeMove_GameNotFound(t *testing.T) {
	h := newTestHub()
	c := newTestClient(h, "alice")
	// c.game is "" so h.games[""] doesn't exist
	h.MakeMove(context.Background(), c, "e4")
	got := drainHub(t, c)
	em, ok := got.(shared.ErrorMsg)
	if !ok || em.Message != errGameNotFound {
		t.Fatalf("got %T %v, want ErrorMsg{game not found}", got, got)
	}
}

func TestHub_MakeMove_InvalidMove(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	setupGame(h, "g1", white, black)
	h.MakeMove(context.Background(), white, "zzz")
	got := drainHub(t, white)
	if _, ok := got.(shared.ErrorMsg); !ok {
		t.Fatalf("got %T, want shared.ErrorMsg for invalid move", got)
	}
}

func TestHub_MakeMove_Valid(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	setupGame(h, "g1", white, black)
	h.MakeMove(context.Background(), white, "e4")
	for _, c := range []*Client{white, black} {
		got := drainHub(t, c)
		if _, ok := got.(shared.MoveMsg); !ok {
			t.Fatalf("client %s: got %T, want shared.MoveMsg", c.username, got)
		}
	}
}

// ── handleTimeExpired ────────────────────────────────────────────────────────

func TestHub_HandleTimeExpired_RemovesGame(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	g := setupGame(h, "g1", white, black)
	// handleTimeExpired calls handleGameOver if game found in h.games.
	// To avoid DB dependency, remove the game from h.games first so
	// handleGameOver (which needs DB) is skipped.
	h.mu.Lock()
	delete(h.games, "g1")
	h.mu.Unlock()
	// Now handleTimeExpired: stillExists=false, so handleGameOver is NOT called.
	h.handleTimeExpired(context.Background(), white, g)
	h.mu.RLock()
	_, exists := h.games["g1"]
	h.mu.RUnlock()
	if exists {
		t.Fatal("game should not exist in hub after handleTimeExpired")
	}
}

// ── RequestUndo / RespondUndo ────────────────────────────────────────────────

func TestHub_RequestUndo_NotFound(t *testing.T) {
	h := newTestHub()
	c := newTestClient(h, "alice")
	h.RequestUndo(c)
	got := drainHub(t, c)
	em, ok := got.(shared.ErrorMsg)
	if !ok || em.Message != errGameNotFound {
		t.Fatalf("got %T %v, want ErrorMsg{game not found}", got, got)
	}
}

func TestHub_RequestUndo_SendsToOpponent(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	g := setupGame(h, "g1", white, black)
	// play one move so undo is possible
	_ = g.ApplyMove(white, "e4")
	h.RequestUndo(white)
	got := drainHub(t, black)
	if _, ok := got.(shared.UndoRequest); !ok {
		t.Fatalf("got %T, want shared.UndoRequest on opponent", got)
	}
}

func TestHub_RespondUndo_NotFound(t *testing.T) {
	h := newTestHub()
	c := newTestClient(h, "alice")
	h.RespondUndo(context.Background(), c, true)
	got := drainHub(t, c)
	if _, ok := got.(shared.ErrorMsg); !ok {
		t.Fatalf("got %T, want shared.ErrorMsg", got)
	}
}

func TestHub_RespondUndo_Accept(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	g := setupGame(h, "g1", white, black)
	_ = g.ApplyMove(white, "e4")
	_ = g.ApplyMove(black, "e5")
	_ = g.RequestUndo(black) // black requests undo of e5
	h.RespondUndo(context.Background(), white, true)
	for _, c := range []*Client{white, black} {
		got := drainHub(t, c)
		if _, ok := got.(shared.UndoAccepted); !ok {
			t.Fatalf("client %s: got %T, want shared.UndoAccepted", c.username, got)
		}
	}
}

func TestHub_RespondUndo_Reject(t *testing.T) {
	h := newTestHub()
	white := newTestClient(h, "white")
	black := newTestClient(h, "black")
	g := setupGame(h, "g1", white, black)
	_ = g.ApplyMove(white, "e4")
	_ = g.RequestUndo(white) // white requests undo
	h.RespondUndo(context.Background(), black, false)
	// white (the requester) should get UndoResponse{Accept: false}
	got := drainHub(t, white)
	ur, ok := got.(shared.UndoResponse)
	if !ok {
		t.Fatalf("got %T, want shared.UndoResponse", got)
	}
	if ur.Accept {
		t.Fatal("expected Accept == false")
	}
}
