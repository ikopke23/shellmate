package screens

// ScreenID identifies a TUI screen.
type ScreenID int

const (
	ScreenLobby ScreenID = iota
	ScreenGame
	ScreenHistory
	ScreenReplay
	ScreenLeaderboard
	ScreenImport
	ScreenImportedGames
	ScreenPuzzle
	ScreenCreateGame
)

// ScreenChangeMsg requests a screen transition.
type ScreenChangeMsg struct {
	Screen ScreenID
	Data   interface{}
}

// ErrMsg carries an error to display.
type ErrMsg struct {
	Err error
}

// JoinGameMsg is sent by the lobby screen to join a game.
type JoinGameMsg struct{ GameID string }

// SpectateGameMsg is sent by the lobby screen to spectate a game.
type SpectateGameMsg struct{ GameID string }

// MakeMoveMsg is sent by the game screen when the player makes a move.
type MakeMoveMsg struct{ SAN string }

// ResignMsg is sent by the game screen when the player resigns.
type ResignMsg struct{}

// RequestUndoMsg is sent by the game screen to request an undo.
type RequestUndoMsg struct{}

// RespondUndoMsg is sent by the game screen to accept/reject an undo.
type RespondUndoMsg struct{ Accept bool }

// SubmitPuzzleAttemptMsg is sent by the puzzle screen to record a puzzle attempt.
type SubmitPuzzleAttemptMsg struct {
	PuzzleID string
	Solved   bool
	Skipped  bool
}

// CheckUsernamesActionMsg is sent by the replay screen to check username existence.
type CheckUsernamesActionMsg struct{ White, Black string }

// UsernameCheckDoneMsg carries the result of username existence checks.
type UsernameCheckDoneMsg struct{ Unknown []string }

// SaveImportedActionMsg is sent by the replay screen to save an imported game.
type SaveImportedActionMsg struct {
	White, Black, PGN string
	ForceCreate       bool
}

// SaveImportedDoneMsg signals a successful imported game save.
type SaveImportedDoneMsg struct{}
