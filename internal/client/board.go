package client

import (
	"github.com/ikopke/shellmate/internal/client/render"
	"github.com/notnil/chess"
)

// Board is an alias for render.Board for backward compatibility.
type Board = render.Board

// NewBoard creates a board in the starting position.
func NewBoard(pos *chess.Position, flipped bool) *Board {
	return render.NewBoard(pos, flipped)
}
