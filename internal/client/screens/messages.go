package screens

import (
	"github.com/gorilla/websocket"
	"github.com/ikopke/shellmate/internal/shared"
)

// ScreenID identifies a TUI screen.
type ScreenID int

const (
	ScreenLogin ScreenID = iota
	ScreenLobby
	ScreenGame
	ScreenHistory
	ScreenReplay
	ScreenLeaderboard
	ScreenImport
	ScreenImportedGames
)

// ConnectedMsg is sent when WebSocket connection is established.
type ConnectedMsg struct {
	Conn     *websocket.Conn
	Username string
	FirstMsg *shared.Envelope // first message read from server (lobby_state on success)
}

// ScreenChangeMsg requests a screen transition.
type ScreenChangeMsg struct {
	Screen ScreenID
	Data   interface{}
}

// WSMsg carries an incoming WebSocket message.
type WSMsg struct {
	Env shared.Envelope
}

// ErrMsg carries an error to display.
type ErrMsg struct {
	Err error
}
