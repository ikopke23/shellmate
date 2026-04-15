package server

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

const errGameNotFound = "game not found"

// Hub manages all active SSH session connections and routes messages.
type Hub struct {
	db         *DB
	clients    map[string]*Client // username -> client
	games      map[string]*Game   // game ID -> game
	mu         sync.RWMutex
	inviteCode string
}

// Client represents a connected SSH session client.
type Client struct {
	username string
	send     chan tea.Msg
	done     chan struct{}
	doneOnce sync.Once
	hub      *Hub
	game     string // game ID the client is currently in, or ""
}

// Send queues a message for delivery to the client's TUI. Safe to call after unregistration.
func (c *Client) Send(msg tea.Msg) {
	select {
	case c.send <- msg:
	case <-c.done:
	default:
		slog.Warn("client send buffer full, dropping message", "username", c.username)
	}
}

// Recv returns a tea.Cmd that blocks until a message arrives from the hub or the client disconnects.
func (c *Client) Recv() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-c.send:
			return msg
		case <-c.done:
			return tea.Quit
		}
	}
}

// NewHub creates a new Hub with the given DB and invite code.
func NewHub(db *DB, inviteCode string) *Hub {
	return &Hub{
		db:         db,
		clients:    make(map[string]*Client),
		games:      make(map[string]*Game),
		inviteCode: inviteCode,
	}
}

// GetUserByKeyFingerprint looks up a user by SSH public key fingerprint.
func (h *Hub) GetUserByKeyFingerprint(ctx context.Context, fingerprint string) (*User, error) {
	return h.db.GetUserByKeyFingerprint(ctx, fingerprint)
}

// RegisterUser creates a new user linked to the given SSH key fingerprint.
func (h *Hub) RegisterUser(ctx context.Context, username, fingerprint string) (*User, error) {
	return h.db.CreateUserWithKey(ctx, username, fingerprint)
}

// LinkKey adds a new SSH key fingerprint to an existing user account.
func (h *Hub) LinkKey(ctx context.Context, username, fingerprint string) error {
	return h.db.LinkKeyToUser(ctx, username, fingerprint)
}

// GetUser returns the user row for the given username.
func (h *Hub) GetUser(ctx context.Context, username string) (*User, error) {
	return h.db.GetUser(ctx, username)
}

// GetHistory returns the game history for the given user.
func (h *Hub) GetHistory(ctx context.Context, username string) ([]HistoryRecord, error) {
	return h.db.GetGameHistory(ctx, username)
}

// GetPuzzle returns the next puzzle for the given user as a shared.PuzzleRecord.
func (h *Hub) GetPuzzle(ctx context.Context, username string) (*shared.PuzzleRecord, error) {
	row, userRating, err := h.GetPuzzleForUser(ctx, username)
	if err != nil || row == nil {
		return nil, err
	}
	return &shared.PuzzleRecord{
		ID:               row.ID,
		FEN:              row.FEN,
		Moves:            row.Moves,
		ContextMoves:     row.ContextMoves,
		Rating:           row.Rating,
		Themes:           row.Themes,
		GameURL:          row.GameURL,
		UserPuzzleRating: userRating,
	}, nil
}

// InviteCode returns the hub's invite code.
func (h *Hub) InviteCode() string { return h.inviteCode }

// GetLeaderboard returns all users ordered by Elo DESC.
func (h *Hub) GetLeaderboard(ctx context.Context) ([]User, error) {
	return h.db.GetLeaderboard(ctx)
}

// GetGameHistory returns game history for a user.
func (h *Hub) GetGameHistory(ctx context.Context, username string) ([]HistoryRecord, error) {
	return h.db.GetGameHistory(ctx, username)
}

func (h *Hub) CheckUsername(ctx context.Context, username string) (bool, error) {
	return h.db.CheckUsername(ctx, username)
}

func (h *Hub) SaveImportedGame(ctx context.Context, white, black, pgn string, forceCreate bool) error {
	return h.db.SaveImportedGame(ctx, white, black, pgn, forceCreate)
}

func (h *Hub) GetImportedGames(ctx context.Context) ([]HistoryRecord, error) {
	return h.db.GetImportedGames(ctx)
}

// GetPuzzleForUser returns an unseen puzzle for the user, fetching from Lichess if needed.
// If the unseen count after serving drops below 3, a background goroutine prefetches today's puzzle.
func (h *Hub) GetPuzzleForUser(ctx context.Context, username string) (*PuzzleRow, int, error) {
	slog.Info("getting puzzle for user", "username", username)
	userRating, err := h.db.GetPuzzleRating(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	puzzle, err := h.db.GetNextPuzzle(ctx, username, userRating)
	if err != nil {
		slog.Error("GetNextPuzzle failed", "username", username, "error", err)
		return nil, 0, err
	}
	if puzzle == nil {
		slog.Info("no cached puzzle found, fetching from lichess", "username", username)
		puzzle, err = h.fetchAndCachePuzzle(ctx, username)
		if err != nil {
			return nil, 0, err
		}
	} else {
		slog.Info("serving cached puzzle", "puzzle_id", puzzle.ID, "username", username)
	}
	// background prefetch if buffer is low
	go func() {
		count, err := h.db.CountUnseenPuzzles(context.Background(), username)
		if err != nil || count >= 3 {
			return
		}
		resp, err := fetchDailyPuzzle(context.Background())
		if err != nil {
			slog.Warn("background puzzle prefetch failed", "error", err)
			return
		}
		row, err := toPuzzleRow(resp)
		if err != nil {
			return
		}
		if err := h.db.SavePuzzle(context.Background(), *row); err != nil {
			slog.Warn("background puzzle save failed", "error", err)
		}
	}()
	return puzzle, userRating, nil
}

func (h *Hub) fetchAndCachePuzzle(ctx context.Context, username string) (*PuzzleRow, error) {
	resp, err := fetchDailyPuzzle(ctx)
	if err != nil {
		slog.Error("lichess fetch failed", "username", username, "error", err)
		return nil, fmt.Errorf("lichess fetch: %w", err)
	}
	slog.Info("lichess fetch succeeded", "puzzle_id", resp.Puzzle.ID)
	row, err := toPuzzleRow(resp)
	if err != nil {
		slog.Error("toPuzzleRow failed", "puzzle_id", resp.Puzzle.ID, "error", err)
		return nil, err
	}
	if err := h.db.SavePuzzle(ctx, *row); err != nil {
		slog.Error("SavePuzzle failed", "puzzle_id", row.ID, "error", err)
		return nil, err
	}
	slog.Info("puzzle saved to cache", "puzzle_id", row.ID)
	return row, nil
}

// RecordPuzzleAttempt records the attempt and updates the user's puzzle rating atomically.
// When skipped is true the rating is unchanged and the current rating is returned.
// Returns the new (or unchanged) puzzle rating.
func (h *Hub) RecordPuzzleAttempt(ctx context.Context, username, puzzleID string, solved, skipped bool) (int, error) {
	puzzle, err := h.db.GetPuzzleByID(ctx, puzzleID)
	if err != nil || puzzle == nil {
		return 0, fmt.Errorf("puzzle not found: %s", puzzleID)
	}
	currentRating, err := h.db.GetPuzzleRating(ctx, username)
	if err != nil {
		return 0, err
	}
	newRating := currentRating
	if !skipped {
		newRating = PuzzleEloOutcome(currentRating, puzzle.Rating, solved)
	}
	if err := h.db.RecordAttemptAndUpdateRating(ctx, username, puzzleID, solved, skipped, newRating); err != nil {
		return 0, err
	}
	return newRating, nil
}

// Register adds an authenticated client to the hub, evicting any existing session for the same user.
func (h *Hub) Register(username string) *Client {
	c := &Client{
		username: username,
		send:     make(chan tea.Msg, 256),
		done:     make(chan struct{}),
		hub:      h,
	}
	h.mu.Lock()
	if existing, ok := h.clients[username]; ok {
		existing.doneOnce.Do(func() { close(existing.done) })
	}
	h.clients[username] = c
	h.mu.Unlock()
	return c
}

// Unregister removes a client and cleans up any games they were in.
// Only deletes the client entry if it still points to the same client (guards against eviction race).
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if stored, ok := h.clients[c.username]; ok && stored == c {
		delete(h.clients, c.username)
	}
	c.doneOnce.Do(func() { close(c.done) })
	// snapshot games to clean up spectators from, without holding lock
	var spectatorGames []*Game
	for _, g := range h.games {
		for _, sp := range g.spectators {
			if sp == c {
				spectatorGames = append(spectatorGames, g)
				break
			}
		}
	}
	h.mu.Unlock()
	// now remove spectator outside the hub lock
	for _, g := range spectatorGames {
		g.RemoveSpectator(c)
	}
}

// buildLobbyData builds the LobbyState and returns a snapshot of current clients.
// Caller must NOT hold h.mu.
func (h *Hub) buildLobbyData(ctx context.Context) (shared.LobbyState, []*Client, error) {
	h.mu.RLock()
	// snapshot everything we need from the hub while locked
	usernames := make([]string, 0, len(h.clients))
	clientSnapshot := make([]*Client, 0, len(h.clients))
	for u, c := range h.clients {
		usernames = append(usernames, u)
		clientSnapshot = append(clientSnapshot, c)
	}
	// snapshot game info (no DB needed for games)
	gameInfos := make([]shared.GameInfo, 0, len(h.games))
	for _, g := range h.games {
		g.mu.Lock()
		gi := shared.GameInfo{
			ID:         g.id,
			Spectators: len(g.spectators),
			Moves:      len(g.chess.Moves()),
		}
		if g.white != nil {
			gi.White = g.white.username
		}
		if g.black != nil {
			gi.Black = g.black.username
		}
		g.mu.Unlock()
		gameInfos = append(gameInfos, gi)
	}
	h.mu.RUnlock()
	// DB calls outside the lock
	playerInfos := make([]shared.PlayerInfo, 0, len(usernames))
	for _, u := range usernames {
		user, err := h.db.GetUser(ctx, u)
		if err != nil || user == nil {
			playerInfos = append(playerInfos, shared.PlayerInfo{Username: u, Elo: 1500, Online: true})
			continue
		}
		playerInfos = append(playerInfos, shared.PlayerInfo{Username: u, Elo: user.Elo, Online: true})
	}
	state := shared.LobbyState{Players: playerInfos, Games: gameInfos}
	return state, clientSnapshot, nil
}

func (h *Hub) sendLobbyTo(ctx context.Context, c *Client) {
	state, _, err := h.buildLobbyData(ctx)
	if err != nil {
		slog.Error("failed to build lobby data", "error", err)
		return
	}
	c.Send(state)
}

// BroadcastLobby sends the current lobby state to all connected clients.
func (h *Hub) BroadcastLobby(ctx context.Context) {
	state, clients, err := h.buildLobbyData(ctx)
	if err != nil {
		slog.Error("failed to build lobby data", "error", err)
		return
	}
	for _, c := range clients {
		c.Send(state)
	}
}

// CreateGame creates a new game with the given time control.
func (h *Hub) CreateGame(ctx context.Context, c *Client, tc shared.TimeControl) {
	id := generateID()
	h.mu.Lock()
	g := NewGame(id, c, nil, tc.InitialSeconds, tc.IncrementSeconds)
	h.games[id] = g
	c.game = id
	h.mu.Unlock()
	slog.Info("game created", "game_id", id, "white", c.username)
	h.BroadcastLobby(ctx)
}

// JoinGame joins an existing open game.
func (h *Hub) JoinGame(ctx context.Context, c *Client, gameID string) {
	h.mu.Lock()
	g, ok := h.games[gameID]
	if !ok {
		h.mu.Unlock()
		sendError(c, errGameNotFound)
		return
	}
	g.mu.Lock()
	if g.black != nil {
		g.mu.Unlock()
		h.mu.Unlock()
		sendError(c, "game is already full")
		return
	}
	if g.white == c {
		g.mu.Unlock()
		h.mu.Unlock()
		sendError(c, "cannot join your own game")
		return
	}
	g.black = c
	c.game = gameID
	g.turnStartedAt = time.Now()
	whiteUsername := g.white.username
	timeControl := shared.TimeControl{
		InitialSeconds:   int(g.whiteRemaining / time.Second),
		IncrementSeconds: int(g.increment / time.Second),
	}
	g.mu.Unlock()
	h.mu.Unlock()
	if g.timed {
		g.resetClock(h)
	}
	slog.Info("player joined game", "game_id", gameID, "black", c.username)
	go h.closeOpenGamesForUsers(ctx, whiteUsername, c.username, gameID)
	g.Broadcast(shared.GameStart{
		GameID:      gameID,
		White:       whiteUsername,
		Black:       c.username,
		TimeControl: timeControl,
	})
	h.BroadcastLobby(ctx)
}

// closeOpenGamesForUsers removes any open (waiting) games created by either player,
// excluding the game that just started. Runs in the background after a game begins.
func (h *Hub) closeOpenGamesForUsers(ctx context.Context, white, black, startedGameID string) {
	h.mu.Lock()
	var toRemove []string
	for id, g := range h.games {
		if id == startedGameID {
			continue
		}
		g.mu.Lock()
		isOpen := g.black == nil
		createdByPlayer := g.white != nil && (g.white.username == white || g.white.username == black)
		g.mu.Unlock()
		if isOpen && createdByPlayer {
			toRemove = append(toRemove, id)
		}
	}
	for _, id := range toRemove {
		delete(h.games, id)
		slog.Info("closed orphaned open game", "game_id", id)
	}
	h.mu.Unlock()
	if len(toRemove) > 0 {
		h.BroadcastLobby(ctx)
	}
}

// SpectateGame adds the client as a spectator of the given game.
func (h *Hub) SpectateGame(ctx context.Context, c *Client, gameID string) {
	h.mu.RLock()
	g, ok := h.games[gameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, errGameNotFound)
		return
	}
	g.mu.Lock()
	g.spectators = append(g.spectators, c)
	c.game = gameID
	started := g.black != nil
	var whiteUsername, blackUsername string
	var moveList []string
	var whiteMs, blackMs int
	var timeControl shared.TimeControl
	if started {
		whiteUsername = g.white.username
		blackUsername = g.black.username
		whiteMs = int(g.whiteRemaining.Milliseconds())
		blackMs = int(g.blackRemaining.Milliseconds())
		timeControl = shared.TimeControl{
			InitialSeconds:   int(g.whiteRemaining / time.Second),
			IncrementSeconds: int(g.increment / time.Second),
		}
		notation := chess.AlgebraicNotation{}
		positions := g.chess.Positions()
		for i, m := range g.chess.Moves() {
			moveList = append(moveList, notation.Encode(positions[i], m))
		}
	}
	g.mu.Unlock()
	slog.Info("spectator joined game", "game_id", gameID, "spectator", c.username)
	if started {
		c.Send(shared.GameStart{
			GameID:      gameID,
			White:       whiteUsername,
			Black:       blackUsername,
			TimeControl: timeControl,
		})
		if len(moveList) > 0 {
			c.Send(shared.MoveMsg{
				GameID: gameID,
				Moves:  moveList,
				Clock:  shared.ClockState{WhiteMs: whiteMs, BlackMs: blackMs},
			})
		}
	}
	h.BroadcastLobby(ctx)
}

// MakeMove applies a move in the client's current game.
func (h *Hub) MakeMove(ctx context.Context, c *Client, san string) {
	h.mu.RLock()
	g, ok := h.games[c.game]
	h.mu.RUnlock()
	if !ok {
		sendError(c, errGameNotFound)
		return
	}
	if err := g.ApplyMove(c, san); err != nil {
		if errors.Is(err, ErrTimeExpired) {
			h.handleTimeExpired(ctx, c, g)
			return
		}
		sendError(c, err.Error())
		return
	}
	g.BroadcastMove()
	if g.IsOver() {
		g.stopClock()
		h.mu.Lock()
		_, stillExists := h.games[c.game]
		if stillExists {
			delete(h.games, c.game)
		}
		h.mu.Unlock()
		if stillExists {
			g.handleGameOver(ctx, h)
			h.BroadcastLobby(ctx)
		}
	} else if g.timed {
		g.resetClock(h)
	}
}

func (h *Hub) handleTimeExpired(ctx context.Context, c *Client, g *Game) {
	g.stopClock()
	h.mu.Lock()
	_, stillExists := h.games[c.game]
	if stillExists {
		delete(h.games, c.game)
	}
	h.mu.Unlock()
	if stillExists {
		g.handleGameOver(ctx, h)
		h.BroadcastLobby(ctx)
	}
}

// RequestUndo sends an undo request to the opponent in the client's current game.
func (h *Hub) RequestUndo(c *Client) {
	h.mu.RLock()
	g, ok := h.games[c.game]
	h.mu.RUnlock()
	if !ok {
		sendError(c, errGameNotFound)
		return
	}
	if err := g.RequestUndo(c); err != nil {
		sendError(c, err.Error())
		return
	}
	g.mu.Lock()
	var opponent *Client
	if c == g.white {
		opponent = g.black
	} else {
		opponent = g.white
	}
	g.mu.Unlock()
	if opponent != nil {
		opponent.Send(shared.UndoRequest{GameID: c.game})
	}
}

// RespondUndo accepts or rejects a pending undo request in the client's current game.
func (h *Hub) RespondUndo(ctx context.Context, c *Client, accept bool) {
	h.mu.RLock()
	g, ok := h.games[c.game]
	h.mu.RUnlock()
	if !ok {
		sendError(c, errGameNotFound)
		return
	}
	if accept {
		if err := g.AcceptUndo(c); err != nil {
			sendError(c, err.Error())
			return
		}
		g.BroadcastUndoAccepted()
	} else {
		if err := g.RejectUndo(c); err != nil {
			sendError(c, err.Error())
			return
		}
		g.mu.Lock()
		var requester *Client
		if c == g.white {
			requester = g.black
		} else {
			requester = g.white
		}
		g.mu.Unlock()
		if requester != nil {
			requester.Send(shared.UndoResponse{GameID: c.game, Accept: false})
		}
	}
}

// Resign forfeits the client's current game.
func (h *Hub) Resign(ctx context.Context, c *Client) {
	h.mu.RLock()
	g, ok := h.games[c.game]
	h.mu.RUnlock()
	if !ok {
		sendError(c, errGameNotFound)
		return
	}
	g.mu.Lock()
	var resignColor chess.Color
	if c == g.white {
		resignColor = chess.White
	} else {
		resignColor = chess.Black
	}
	g.chess.Resign(resignColor)
	g.mu.Unlock()
	g.stopClock()
	h.mu.Lock()
	_, stillExists := h.games[c.game]
	if stillExists {
		delete(h.games, c.game)
	}
	h.mu.Unlock()
	if stillExists {
		g.handleGameOver(ctx, h)
		h.BroadcastLobby(ctx)
	}
}

func sendError(c *Client, message string) {
	c.Send(shared.ErrorMsg{Message: message})
}

// generateID creates a random hex string suitable for game IDs.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%x", b)
}
