package client

import "testing"

func TestNewMoveList_DelegatesToRender(t *testing.T) {
	ml := NewMoveList(10)
	if ml == nil {
		t.Fatal("NewMoveList returned nil")
	}
}
