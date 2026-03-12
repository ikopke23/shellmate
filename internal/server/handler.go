package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

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
	default:
		http.NotFound(w, r)
	}
}
