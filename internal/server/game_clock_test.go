package server

import (
	"errors"
	"testing"
	"time"

	"github.com/notnil/chess"
)

func makeTimedGame(initialSec, incrementSec int) *Game {
	white := &Client{username: "white"}
	black := &Client{username: "black"}
	g := NewGame("test", white, black, initialSec, incrementSec)
	g.turnStartedAt = time.Now().Add(-2 * time.Second) // simulate 2s elapsed
	return g
}

func TestApplyMove_DeductsTime(t *testing.T) {
	g := makeTimedGame(60, 0)
	before := g.whiteRemaining
	if err := g.ApplyMove(g.white, "e4"); err != nil {
		t.Fatal(err)
	}
	if g.whiteRemaining >= before {
		t.Fatalf("expected time deducted, got %v (before %v)", g.whiteRemaining, before)
	}
}

func TestApplyMove_AddsIncrement(t *testing.T) {
	g := makeTimedGame(60, 5)
	g.turnStartedAt = time.Now().Add(-1 * time.Second) // 1s elapsed
	before := g.whiteRemaining
	if err := g.ApplyMove(g.white, "e4"); err != nil {
		t.Fatal(err)
	}
	// net change: -1s + 5s = +4s
	if g.whiteRemaining <= before {
		t.Fatalf("expected time gained from increment, got %v (before %v)", g.whiteRemaining, before)
	}
}

func TestApplyMove_TimeExpired(t *testing.T) {
	g := makeTimedGame(1, 0)
	g.turnStartedAt = time.Now().Add(-3 * time.Second) // 3s > 1s limit
	err := g.ApplyMove(g.white, "e4")
	if !errors.Is(err, ErrTimeExpired) {
		t.Fatalf("expected ErrTimeExpired, got %v", err)
	}
	if g.chess.Outcome() == chess.NoOutcome {
		t.Fatal("expected game to be marked over after time expiry")
	}
}

func TestApplyMove_Untimed(t *testing.T) {
	white := &Client{username: "white"}
	black := &Client{username: "black"}
	g := NewGame("test", white, black, 0, 0)
	// untimed: no clock deduction, move always succeeds
	if err := g.ApplyMove(g.white, "e4"); err != nil {
		t.Fatalf("untimed game should not fail: %v", err)
	}
	if g.whiteRemaining != 0 {
		t.Fatal("untimed game should have zero remaining")
	}
}

func TestApplyMove_DeductsBlackTime(t *testing.T) {
	white := &Client{username: "white"}
	black := &Client{username: "black"}
	g := NewGame("test", white, black, 60, 0)
	g.turnStartedAt = time.Now().Add(-1 * time.Second)
	if err := g.ApplyMove(g.white, "e4"); err != nil {
		t.Fatal(err)
	}
	whiteBefore := g.whiteRemaining
	g.turnStartedAt = time.Now().Add(-2 * time.Second)
	if err := g.ApplyMove(g.black, "e5"); err != nil {
		t.Fatal(err)
	}
	if g.blackRemaining >= 60*time.Second {
		t.Fatalf("expected black time deducted, got %v", g.blackRemaining)
	}
	if g.whiteRemaining != whiteBefore {
		t.Fatalf("white time should not change on black's move, got %v (before %v)", g.whiteRemaining, whiteBefore)
	}
}
