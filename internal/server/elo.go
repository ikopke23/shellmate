package server

import "math"

// expected returns the expected score for player A given ratings rA and rB.
func expected(rA, rB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(rB-rA)/400))
}

// Calculate computes new Elo ratings for both players after a game.
//
// Parameters:
//   - rA, rB: current ratings
//   - gamesA, gamesB: games played by each player (used for provisional K-factor)
//   - result: 1.0 = player A wins, 0.0 = player A loses, 0.5 = draw
//
// Returns (newA, newB).
//
// Rules:
//   - K = 40 if either player has played fewer than 15 games (provisional), else K = 20
//   - Delta capped at ±50 per game
//   - Elo floor of 800
func Calculate(rA, rB, gamesA, gamesB int, result float64) (int, int) {
	k := 20.0
	if gamesA < 15 || gamesB < 15 {
		k = 40.0
	}
	e := expected(rA, rB)
	deltaA := math.Round(k * (result - e))
	deltaA = math.Max(-50, math.Min(50, deltaA))
	newA := max(800, rA+int(deltaA))
	newB := max(800, rB-int(deltaA))
	return newA, newB
}
