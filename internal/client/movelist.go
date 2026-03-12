package client

import "github.com/ikopke/shellmate/internal/client/render"

// MoveList is an alias for render.MoveList for backward compatibility.
type MoveList = render.MoveList

// NewMoveList creates an empty move list with the given visible height.
func NewMoveList(height int) *MoveList {
	return render.NewMoveList(height)
}
