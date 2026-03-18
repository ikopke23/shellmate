package server

import "math"

const puzzleEloK = 32.0

// PuzzleEloOutcome computes the new user puzzle rating after an attempt.
// K=32, no floor, simple Elo delta.
func PuzzleEloOutcome(userRating, puzzleRating int, solved bool) int {
	e := 1.0 / (1.0 + math.Pow(10, float64(puzzleRating-userRating)/400))
	outcome := 0.0
	if solved {
		outcome = 1.0
	}
	raw := puzzleEloK * (outcome - e)
	var delta float64
	if raw >= 0 {
		delta = math.Ceil(raw)
	} else {
		delta = math.Floor(raw)
	}
	return userRating + int(delta)
}
