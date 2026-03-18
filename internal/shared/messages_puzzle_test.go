package shared

import (
	"encoding/json"
	"testing"
)

func TestPuzzleRecordRoundTrip(t *testing.T) {
	r := PuzzleRecord{
		ID:               "eXZ8r",
		FEN:              "r1bqkb1r/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
		Moves:            "e2e4 e7e5 g1f3",
		Rating:           2001,
		Themes:           []string{"endgame", "pin"},
		GameURL:          "https://lichess.org/abc#34",
		UserPuzzleRating: 1543,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PuzzleRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != r.ID || got.Rating != r.Rating || got.UserPuzzleRating != r.UserPuzzleRating {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, r)
	}
}

func TestPuzzleAttemptResultRoundTrip(t *testing.T) {
	r := PuzzleAttemptResult{PuzzleRating: 1555}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PuzzleAttemptResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.PuzzleRating != r.PuzzleRating {
		t.Errorf("got %d, want %d", got.PuzzleRating, r.PuzzleRating)
	}
}
