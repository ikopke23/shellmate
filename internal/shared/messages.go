package shared

import "encoding/json"

// MsgType identifies the kind of WebSocket message being sent or received.
type MsgType string

const (
	MsgJoinLobby    MsgType = "join_lobby"
	MsgLobbyState   MsgType = "lobby_state"
	MsgCreateGame   MsgType = "create_game"
	MsgJoinGame     MsgType = "join_game"
	MsgSpectateGame MsgType = "spectate_game"
	MsgMove         MsgType = "move"
	MsgUndoRequest  MsgType = "undo_request"
	MsgUndoResponse MsgType = "undo_response"
	MsgGameOver     MsgType = "game_over"
	MsgError        MsgType = "error"
)

// Envelope is the top-level wrapper for all WebSocket messages.
type Envelope struct {
	Type    MsgType         `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// JoinLobby is sent by a client when connecting to the lobby.
type JoinLobby struct {
	InviteCode string `json:"invite_code"`
	Username   string `json:"username"`
}

// PlayerInfo describes a player in the lobby.
type PlayerInfo struct {
	Username string `json:"username"`
	Elo      int    `json:"elo"`
	Online   bool   `json:"online"`
}

// GameInfo describes a game visible in the lobby.
type GameInfo struct {
	ID         string `json:"id"`
	White      string `json:"white"`
	Black      string `json:"black"`
	Spectators int    `json:"spectators"`
	Moves      int    `json:"moves"`
}

// LobbyState is broadcast to clients to describe current lobby membership and active games.
type LobbyState struct {
	Players []PlayerInfo `json:"players"`
	Games   []GameInfo   `json:"games"`
}

// CreateGame is an empty signal from a client requesting a new game be created.
type CreateGame struct{}

// JoinGame is sent by a client to join an existing game as a player.
type JoinGame struct {
	GameID string `json:"game_id"`
}

// SpectateGame is sent by a client to observe a game without playing.
type SpectateGame struct {
	GameID string `json:"game_id"`
}

// Move is sent by a client to make a move in a game.
type Move struct {
	GameID string `json:"game_id"`
	SAN    string `json:"san"`
}

// UndoRequest is sent by a client to request that the last move be undone.
type UndoRequest struct {
	GameID string `json:"game_id"`
}

// UndoResponse is sent by a client to accept or reject an undo request.
type UndoResponse struct {
	GameID string `json:"game_id"`
	Accept bool   `json:"accept"`
}

// GameOver is broadcast when a game ends, including Elo changes.
type GameOver struct {
	GameID         string `json:"game_id"`
	Result         string `json:"result"`
	WhiteEloBefore int    `json:"white_elo_before"`
	BlackEloBefore int    `json:"black_elo_before"`
	WhiteEloAfter  int    `json:"white_elo_after"`
	BlackEloAfter  int    `json:"black_elo_after"`
	WhiteUsername  string `json:"white_username"`
	BlackUsername  string `json:"black_username"`
}

// Encode wraps a payload into an Envelope and marshals it to JSON.
func Encode(msgType MsgType, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: msgType, Payload: raw})
}

// Decode unmarshals an Envelope from JSON bytes.
func Decode(data []byte) (Envelope, error) {
	var env Envelope
	err := json.Unmarshal(data, &env)
	return env, err
}
