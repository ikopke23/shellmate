package server

import (
	"strings"
	"testing"
)

const validCSVRow = `abcde,rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1,d7d5 e4d5,1500,80,90,12345,opening fork,https://lichess.org/abc#10,italian`

func TestParsePuzzleCSVRow_Valid(t *testing.T) {
	row, err := parsePuzzleCSVRow(strings.Split(validCSVRow, ","))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != "abcde" {
		t.Errorf("ID = %q, want %q", row.ID, "abcde")
	}
	if row.Moves != "d7d5 e4d5" {
		t.Errorf("Moves = %q, want %q", row.Moves, "d7d5 e4d5")
	}
	if row.Rating != 1500 {
		t.Errorf("Rating = %d, want 1500", row.Rating)
	}
	if len(row.Themes) != 2 {
		t.Errorf("Themes len = %d, want 2", len(row.Themes))
	}
}

func TestParsePuzzleCSVRow_WrongColumnCount(t *testing.T) {
	_, err := parsePuzzleCSVRow([]string{"a", "b", "c"})
	if err == nil {
		t.Error("expected error for wrong column count")
	}
}

func TestParsePuzzleCSVRow_BadRating(t *testing.T) {
	cols := strings.Split(validCSVRow, ",")
	cols[3] = "notanumber"
	_, err := parsePuzzleCSVRow(cols)
	if err == nil {
		t.Error("expected error for non-integer rating")
	}
}

func TestRatingFilter(t *testing.T) {
	cols := strings.Split(validCSVRow, ",")
	cols[3] = "100" // below 800
	row, err := parsePuzzleCSVRow(cols)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if row.Rating != 100 {
		t.Fatalf("expected rating 100, got %d", row.Rating)
	}
	// The rating filter is applied by RunImport, not parsePuzzleCSVRow.
	// Verify the constant bounds used in RunImport.
	if importMinRating > 800 || importMaxRating < 2800 {
		t.Errorf("rating bounds should cover 800–2800, got %d–%d", importMinRating, importMaxRating)
	}
}
