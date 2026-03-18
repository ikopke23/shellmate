package server

import (
	"math"
	"testing"
)

func TestPuzzleEloOutcome(t *testing.T) {
	tests := []struct {
		name         string
		userRating   int
		puzzleRating int
		solved       bool
		wantMin      int
		wantMax      int
	}{
		{
			name:       "equal ratings solved",
			userRating: 1500, puzzleRating: 1500, solved: true,
			wantMin: 1515, wantMax: 1517,
		},
		{
			name:       "equal ratings failed",
			userRating: 1500, puzzleRating: 1500, solved: false,
			wantMin: 1483, wantMax: 1485,
		},
		{
			name:       "user stronger solves easy puzzle small gain",
			userRating: 2000, puzzleRating: 1200, solved: true,
			wantMin: 2001, wantMax: 2004,
		},
		{
			name:       "user weaker solves hard puzzle big gain",
			userRating: 1200, puzzleRating: 2000, solved: true,
			wantMin: 1230, wantMax: 1233,
		},
		{
			name:       "user stronger fails easy puzzle big drop",
			userRating: 2000, puzzleRating: 1200, solved: false,
			wantMin: 1968, wantMax: 1971,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PuzzleEloOutcome(tt.userRating, tt.puzzleRating, tt.solved)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("PuzzleEloOutcome(%d, %d, %v) = %d, want [%d, %d]",
					tt.userRating, tt.puzzleRating, tt.solved, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestPuzzleEloNeverGoesNegative(t *testing.T) {
	got := PuzzleEloOutcome(800, 3000, false)
	if got < 0 {
		t.Errorf("rating went negative: %d", got)
	}
	_ = math.Round // confirm math import works
}
