package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/shared"
)

type saveImportedRequest struct {
	White       string `json:"white"`
	Black       string `json:"black"`
	PGN         string `json:"pgn"`
	ForceCreate bool   `json:"force_create"`
}

type checkUsernameResponse struct {
	Exists bool `json:"exists"`
}

type puzzleAttemptRequest struct {
	Username string `json:"username"`
	PuzzleID string `json:"puzzle_id"`
	Solved   bool   `json:"solved"`
	Skipped  bool   `json:"skipped"`
}

// Handler holds the hub and upgrader for HTTP/WS handling.
type Handler struct {
	hub      *Hub
	upgrader websocket.Upgrader
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{
		hub: hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ServeHTTP routes requests:
// GET /ws          → WebSocket upgrade, calls hub.HandleConn
// GET /leaderboard → JSON array of User records ordered by Elo
// GET /history     → JSON array of HistoryRecord for ?user=<username>
// Otherwise → 404
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ws":
		conn, err := h.upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("websocket upgrade failed", "error", err)
			return
		}
		h.hub.HandleConn(r.Context(), conn)
	case "/leaderboard":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		users, err := h.hub.GetLeaderboard(r.Context())
		if err != nil {
			slog.Error("failed to get leaderboard", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if users == nil {
			users = []User{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	case "/history":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := r.URL.Query().Get("user")
		if username == "" {
			http.Error(w, "user parameter required", http.StatusBadRequest)
			return
		}
		records, err := h.hub.GetGameHistory(r.Context(), username)
		if err != nil {
			slog.Error("failed to get game history", "error", err, "username", username)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if records == nil {
			records = []HistoryRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	case "/check-username":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "name parameter required", http.StatusBadRequest)
			return
		}
		exists, err := h.hub.CheckUsername(r.Context(), name)
		if err != nil {
			slog.Error("failed to check username", "error", err, "name", name)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(checkUsernameResponse{Exists: exists})
	case "/save-imported":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req saveImportedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.White == "" || req.Black == "" || req.PGN == "" {
			http.Error(w, "white, black, and pgn are required", http.StatusBadRequest)
			return
		}
		if err := h.hub.SaveImportedGame(r.Context(), req.White, req.Black, req.PGN, req.ForceCreate); err != nil {
			slog.Error("failed to save imported game", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "/imported-games":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		records, err := h.hub.GetImportedGames(r.Context())
		if err != nil {
			slog.Error("failed to get imported games", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if records == nil {
			records = []HistoryRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	case "/puzzle/attempt":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req puzzleAttemptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Username == "" || req.PuzzleID == "" {
			http.Error(w, "username and puzzle_id are required", http.StatusBadRequest)
			return
		}
		newRating, err := h.hub.RecordPuzzleAttempt(r.Context(), req.Username, req.PuzzleID, req.Solved, req.Skipped)
		if err != nil {
			slog.Error("failed to record puzzle attempt", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(shared.PuzzleAttemptResult{PuzzleRating: newRating})
	case "/puzzle":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := r.URL.Query().Get("user")
		if username == "" {
			http.Error(w, "user parameter required", http.StatusBadRequest)
			return
		}
		puzzle, userRating, err := h.hub.GetPuzzleForUser(r.Context(), username)
		if err != nil {
			slog.Error("failed to get puzzle", "error", err, "username", username)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if puzzle == nil {
			http.Error(w, "no puzzle available", http.StatusServiceUnavailable)
			return
		}
		record := shared.PuzzleRecord{
			ID:               puzzle.ID,
			FEN:              puzzle.FEN,
			Moves:            puzzle.Moves,
			ContextMoves:     puzzle.ContextMoves,
			Rating:           puzzle.Rating,
			Themes:           puzzle.Themes,
			GameURL:          puzzle.GameURL,
			UserPuzzleRating: userRating,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(record)
	default:
		http.NotFound(w, r)
	}
}
