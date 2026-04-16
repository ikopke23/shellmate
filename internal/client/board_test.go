package client

import (
	"testing"

	"github.com/notnil/chess"
)

func TestNewBoard_DelegatesToRender(t *testing.T) {
	pos := chess.NewGame().Position()
	b := NewBoard(pos, false)
	if b == nil {
		t.Fatal("NewBoard returned nil")
	}
}
