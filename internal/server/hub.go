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
	hub      *Hub
	game     string // game ID the client is currently in, or ""
}

// Send queues a message for writing. Drops the message if the buffer is full.
func (c *Client) Send(msg []byte) {
	select {
	case c.send <- msg:
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

// Register adds a new authenticated client to the hub.
func (h *Hub) Register(username string, conn *websocket.Conn) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := &Client{
		username: username,
		conn:     conn,
		send:     make(chan []byte, 256),
		hub:      h,
	}
	h.clients[username] = c
	return c
}

// Unregister removes a client and cleans up any games they were in.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c.username)
	close(c.send)
	// Remove from any game as spectator.
	for _, g := range h.games {
		g.RemoveSpectator(c)
	}
}

// BroadcastLobby sends the current lobby state to all connected clients.
func (h *Hub) BroadcastLobby(ctx context.Context) {
	h.mu.RLock()
	var players []shared.PlayerInfo
	for _, c := range h.clients {
		u, err := h.db.GetUser(ctx, c.username)
		if err != nil || u == nil {
			players = append(players, shared.PlayerInfo{Username: c.username, Elo: 1500, Online: true})
			continue
		}
		players = append(players, shared.PlayerInfo{Username: u.Username, Elo: u.Elo, Online: true})
	}
	var gameInfos []shared.GameInfo
	for _, g := range h.games {
		g.mu.Lock()
		black := ""
		if g.black != nil {
			black = g.black.username
		}
		gi := shared.GameInfo{
			ID:         g.id,
			White:      g.white.username,
			Black:      black,
			Spectators: len(g.spectators),
			Moves:      len(g.chess.Moves()),
		}
		g.mu.Unlock()
		gameInfos = append(gameInfos, gi)
	}
	h.mu.RUnlock()
	data, err := shared.Encode(shared.MsgLobbyState, shared.LobbyState{Players: players, Games: gameInfos})
	if err != nil {
		slog.Error("failed to encode lobby state", "error", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		c.Send(data)
	}
}

// Route dispatches an incoming Envelope to the correct handler.
func (h *Hub) Route(ctx context.Context, c *Client, env shared.Envelope) {
	switch env.Type {
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
		h.handleSpectateGame(c, payload)
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
	g.mu.Unlock()
	h.mu.Unlock()
	slog.Info("player joined game", "game_id", payload.GameID, "black", c.username)
	h.BroadcastLobby(ctx)
}

func (h *Hub) handleSpectateGame(c *Client, payload shared.SpectateGame) {
	h.mu.RLock()
	g, ok := h.games[payload.GameID]
	h.mu.RUnlock()
	if !ok {
		sendError(c, "game not found")
		return
	}
	g.AddSpectator(c)
	c.game = payload.GameID
	slog.Info("spectator joined game", "game_id", payload.GameID, "spectator", c.username)
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
		g.handleGameOver(ctx, h)
		h.mu.Lock()
		delete(h.games, payload.GameID)
		h.mu.Unlock()
		h.BroadcastLobby(ctx)
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
		// Broadcast updated move state with empty SAN to indicate undo.
		g.BroadcastMove("")
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
		for msg := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Error("write error", "username", client.username, "error", err)
				return
			}
		}
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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

// encodeMsg is a package-level helper used by game.go.
func encodeMsg(msgType string, payload any) ([]byte, error) {
	return shared.Encode(shared.MsgType(msgType), payload)
}
