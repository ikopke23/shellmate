package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/shared"
	"github.com/notnil/chess"
)

// Hub manages all active WebSocket connections and routes messages.
type Hub struct {
	db         *DB
	clients    map[string]*Client // username -> client
	games      map[string]*Game   // game ID -> game
	mu         sync.RWMutex
	inviteCode string
}

// Client represents a connected WebSocket client.
type Client struct {
	username string
	conn     *websocket.Conn
	send     chan []byte
	done     chan struct{}
	doneOnce sync.Once
	hub      *Hub
	game     string // game ID the client is currently in, or ""
}

// closeDone closes the done channel exactly once.
func (c *Client) closeDone() {
	c.doneOnce.Do(func() { close(c.done) })
}

// Send queues a message for writing. Safe to call after the client is unregistered.
func (c *Client) Send(msg []byte) {
	select {
	case c.send <- msg:
	case <-c.done:
	default:
		slog.Warn("client send buffer full, dropping message", "username", c.username)
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
	puzzle, err := h.db.GetNextPuzzle(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	if puzzle == nil {
		// no unseen puzzles — fetch today's from Lichess
		resp, err := fetchDailyPuzzle(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("lichess fetch: %w", err)
		}
		row, err := toPuzzleRow(resp)
		if err != nil {
			return nil, 0, err
		}
		if err := h.db.SavePuzzle(ctx, *row); err != nil {
			return nil, 0, err
		}
		puzzle = row
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
	rating, err := h.db.GetPuzzleRating(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	return puzzle, rating, nil
}

// RecordPuzzleAttempt records the attempt and updates the user's puzzle rating atomically.
// Returns the new puzzle rating.
func (h *Hub) RecordPuzzleAttempt(ctx context.Context, username, puzzleID string, solved bool) (int, error) {
	puzzle, err := h.db.GetPuzzleByID(ctx, puzzleID)
	if err != nil || puzzle == nil {
		return 0, fmt.Errorf("puzzle not found: %s", puzzleID)
	}
	currentRating, err := h.db.GetPuzzleRating(ctx, username)
	if err != nil {
		return 0, err
	}
	newRating := PuzzleEloOutcome(currentRating, puzzle.Rating, solved)
	if err := h.db.RecordAttemptAndUpdateRating(ctx, username, puzzleID, solved, newRating); err != nil {
		return 0, err
	}
	return newRating, nil
}

// Register adds a new authenticated client to the hub.
// If there is already a connection for this username, the old connection is closed.
func (h *Hub) Register(username string, conn *websocket.Conn) *Client {
	c := &Client{
		username: username,
		conn:     conn,
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
		hub:      h,
	}
	h.mu.Lock()
	if existing, ok := h.clients[username]; ok {
		existing.conn.Close()
		existing.closeDone()
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
	c.closeDone()
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

// buildLobbyData builds the encoded LobbyState message and returns a snapshot of current clients.
// Caller must NOT hold h.mu.
func (h *Hub) buildLobbyData(ctx context.Context) ([]byte, []*Client, error) {
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
	data, err := shared.Encode(shared.MsgLobbyState, state)
	return data, clientSnapshot, err
}

// sendLobbyTo sends the current lobby state to a single client.
func (h *Hub) sendLobbyTo(ctx context.Context, c *Client) {
	data, _, err := h.buildLobbyData(ctx)
	if err != nil {
		slog.Error("failed to build lobby data", "error", err)
		return
	}
	if data != nil {
		c.Send(data)
	}
}

// BroadcastLobby sends the current lobby state to all connected clients.
func (h *Hub) BroadcastLobby(ctx context.Context) {
	data, clients, err := h.buildLobbyData(ctx)
	if err != nil {
		slog.Error("failed to build lobby data", "error", err)
		return
	}
	if data == nil {
		return
	}
	for _, c := range clients {
		c.Send(data)
	}
}

// Route dispatches an incoming Envelope to the correct handler.
func (h *Hub) Route(ctx context.Context, c *Client, env shared.Envelope) {
	switch env.Type {
	case shared.MsgJoinLobby:
		// Already connected; re-send lobby state.
		h.sendLobbyTo(ctx, c)
	case shared.MsgCreateGame:
		h.handleCreateGame(ctx, c)
	case shared.MsgJoinGame:
		var payload shared.JoinGame
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			sendError(c, "invalid join_game payload")
			return
		}
		h.handleJoinGame(ctx, c, payload)
	case shared.MsgSpectateGame:
		var payload shared.SpectateGame
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			sendError(c, "invalid spectate_game payload")
			return
		}
		h.handleSpectateGame(ctx, c, payload)
	case shared.MsgMove:
		var payload shared.Move
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			sendError(c, "invalid move payload")
			return
		}
		h.handleMove(ctx, c, payload)
	case shared.MsgUndoRequest:
		var payload shared.UndoRequest
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			sendError(c, "invalid undo_request payload")
			return
		}
		h.handleUndoRequest(c, payload)
	case shared.MsgResign:
		var payload shared.Resign
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			sendError(c, "invalid resign payload")
			return
		}
		h.handleResign(ctx, c, payload)
	case shared.MsgUndoResponse:
		var payload shared.UndoResponse
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			sendError(c, "invalid undo_response payload")
			return
		}
		h.handleUndoResponse(ctx, c, payload)
	default:
		sendError(c, fmt.Sprintf("unknown message type: %s", env.Type))
	}
}

func (h *Hub) handleCreateGame(ctx context.Context, c *Client) {
	id := generateID()
	h.mu.Lock()
	g := NewGame(id, c, nil)
	h.games[id] = g
	c.game = id
	h.mu.Unlock()
	slog.Info("game created", "game_id", id, "white", c.username)
	h.BroadcastLobby(ctx)
}

func (h *Hub) handleJoinGame(ctx context.Context, c *Client, payload shared.JoinGame) {
	h.mu.Lock()
	g, ok := h.games[payload.GameID]
	if !ok {
		h.mu.Unlock()
		sendError(c, "game not found")
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
	c.game = payload.GameID
	whiteUsername := g.white.username
	g.mu.Unlock()
	h.mu.Unlock()
	slog.Info("player joined game", "game_id", payload.GameID, "black", c.username)
	go h.closeOpenGamesForUsers(ctx, whiteUsername, c.username, payload.GameID)
	// notify both players the game has started
	startMsg, _ := shared.Encode(shared.MsgGameStart, shared.GameStart{
		GameID: payload.GameID,
		White:  whiteUsername,
		Black:  c.username,
	})
	g.Broadcast(startMsg)
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

func (h *Hub) handleSpectateGame(ctx context.Context, c *Client, payload shared.SpectateGame) {
	h.mu.RLock()
	g, ok := h.games[payload.GameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, "game not found")
		return
	}
	g.mu.Lock()
	g.spectators = append(g.spectators, c)
	c.game = payload.GameID
	started := g.black != nil
	var whiteUsername, blackUsername string
	var moveList []string
	if started {
		whiteUsername = g.white.username
		blackUsername = g.black.username
		notation := chess.AlgebraicNotation{}
		positions := g.chess.Positions()
		for i, m := range g.chess.Moves() {
			moveList = append(moveList, notation.Encode(positions[i], m))
		}
	}
	g.mu.Unlock()
	slog.Info("spectator joined game", "game_id", payload.GameID, "spectator", c.username)
	if started {
		startMsg, _ := shared.Encode(shared.MsgGameStart, shared.GameStart{
			GameID: payload.GameID,
			White:  whiteUsername,
			Black:  blackUsername,
		})
		c.Send(startMsg)
		if len(moveList) > 0 {
			type moveMsg struct {
				GameID string   `json:"game_id"`
				SAN    string   `json:"san"`
				Moves  []string `json:"moves"`
			}
			moveData, _ := shared.Encode(shared.MsgMove, moveMsg{
				GameID: payload.GameID,
				SAN:    moveList[len(moveList)-1],
				Moves:  moveList,
			})
			c.Send(moveData)
		}
	}
	h.BroadcastLobby(ctx)
}

func (h *Hub) handleMove(ctx context.Context, c *Client, payload shared.Move) {
	h.mu.RLock()
	g, ok := h.games[payload.GameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, "game not found")
		return
	}
	if err := g.ApplyMove(c, payload.SAN); err != nil {
		sendError(c, err.Error())
		return
	}
	g.BroadcastMove(payload.SAN)
	if g.IsOver() {
		h.mu.Lock()
		_, stillExists := h.games[payload.GameID]
		if stillExists {
			delete(h.games, payload.GameID)
		}
		h.mu.Unlock()
		if stillExists {
			g.handleGameOver(ctx, h)
			h.BroadcastLobby(ctx)
		}
	}
}

func (h *Hub) handleUndoRequest(c *Client, payload shared.UndoRequest) {
	h.mu.RLock()
	g, ok := h.games[payload.GameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, "game not found")
		return
	}
	if err := g.RequestUndo(c); err != nil {
		sendError(c, err.Error())
		return
	}
	// Send the undo request to the opponent.
	g.mu.Lock()
	var opponent *Client
	if c == g.white {
		opponent = g.black
	} else {
		opponent = g.white
	}
	g.mu.Unlock()
	if opponent != nil {
		data, err := shared.Encode(shared.MsgUndoRequest, shared.UndoRequest{GameID: payload.GameID})
		if err == nil {
			opponent.Send(data)
		}
	}
}

func (h *Hub) handleUndoResponse(ctx context.Context, c *Client, payload shared.UndoResponse) {
	h.mu.RLock()
	g, ok := h.games[payload.GameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, "game not found")
		return
	}
	if payload.Accept {
		if err := g.AcceptUndo(c); err != nil {
			sendError(c, err.Error())
			return
		}
		// Broadcast undo accepted with current move list.
		g.BroadcastUndoAccepted()
	} else {
		if err := g.RejectUndo(c); err != nil {
			sendError(c, err.Error())
			return
		}
		// Notify the requester that undo was rejected.
		g.mu.Lock()
		var requester *Client
		if c == g.white {
			requester = g.black
		} else {
			requester = g.white
		}
		g.mu.Unlock()
		if requester != nil {
			data, err := shared.Encode(shared.MsgUndoResponse, shared.UndoResponse{GameID: payload.GameID, Accept: false})
			if err == nil {
				requester.Send(data)
			}
		}
	}
}

func (h *Hub) handleResign(ctx context.Context, c *Client, payload shared.Resign) {
	h.mu.RLock()
	g, ok := h.games[payload.GameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, "game not found")
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
	h.mu.Lock()
	_, stillExists := h.games[payload.GameID]
	if stillExists {
		delete(h.games, payload.GameID)
	}
	h.mu.Unlock()
	if stillExists {
		g.handleGameOver(ctx, h)
		h.BroadcastLobby(ctx)
	}
}

// HandleConn handles a single WebSocket connection from accept to close.
func (h *Hub) HandleConn(ctx context.Context, conn *websocket.Conn) {
	// Step 1: Read first message — must be join_lobby.
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		slog.Error("failed to read first message", "error", err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})
	env, err := shared.Decode(msg)
	if err != nil || env.Type != shared.MsgJoinLobby {
		writeError(conn, "first message must be join_lobby")
		conn.Close()
		return
	}
	var join shared.JoinLobby
	if err := json.Unmarshal(env.Payload, &join); err != nil {
		writeError(conn, "invalid join_lobby payload")
		conn.Close()
		return
	}
	// Step 2: Validate invite code.
	if join.InviteCode != h.inviteCode {
		writeError(conn, "invalid invite code")
		conn.Close()
		return
	}
	if join.Username == "" {
		writeError(conn, "username is required")
		conn.Close()
		return
	}
	// Step 3: Look up username in DB.
	u, err := h.db.GetUser(ctx, join.Username)
	if err != nil {
		writeError(conn, "database error")
		conn.Close()
		return
	}
	if u == nil {
		if err := h.db.CreateUser(ctx, join.Username); err != nil {
			writeError(conn, "failed to create user")
			conn.Close()
			return
		}
	}
	// Step 4: Register the client.
	client := h.Register(join.Username, conn)
	slog.Info("client connected", "username", join.Username)
	// Step 5 & 6: Send lobby state and broadcast.
	h.BroadcastLobby(ctx)
	// Step 7: Start write goroutine.
	go func() {
		defer conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		for {
			select {
			case msg := <-client.send:
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					slog.Error("write error", "username", client.username, "err", err)
					conn.Close()
					return
				}
			case <-client.done:
				return
			}
		}
	}()
	// Step 8: Read loop.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			slog.Info("client disconnected", "username", client.username, "error", err)
			break
		}
		env, err := shared.Decode(msg)
		if err != nil {
			sendError(client, "invalid message format")
			continue
		}
		h.Route(ctx, client, env)
	}
	// Step 9: Cleanup on disconnect.
	h.Unregister(client)
	h.BroadcastLobby(ctx)
}

// sendError sends an error message to a client via their send channel.
func sendError(c *Client, message string) {
	data, err := shared.Encode(shared.MsgError, shared.ErrorMsg{Message: message})
	if err != nil {
		return
	}
	c.Send(data)
}

// writeError writes an error message directly to a websocket connection.
func writeError(conn *websocket.Conn, message string) {
	data, err := shared.Encode(shared.MsgError, shared.ErrorMsg{Message: message})
	if err != nil {
		return
	}
	conn.WriteMessage(websocket.TextMessage, data)
}

// generateID creates a random hex string suitable for game IDs.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%x", b)
}
