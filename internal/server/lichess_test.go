package server

import (
	"strings"
	"testing"
)

// openingPGN is the first 5 moves of a standard game, giving us 11 positions (plies 0-10).
const openingPGN = `1. e4 e5 2. Nf3 Nc6 3. Bb5 a6 4. Ba4 Nf6 5. O-O Be7`

func TestFenAtPly(t *testing.T) {
	tests := []struct {
		name    string
		ply     int
		wantFen string // piece-placement portion only (before first space)
		wantErr bool
	}{
		{
			name:    "ply 0 is starting position",
			ply:     0,
			wantFen: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR",
		},
		{
			name:    "ply 1 after e4",
			ply:     1,
			wantFen: "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR",
		},
		{
			name:    "ply 2 after e4 e5",
			ply:     2,
			wantFen: "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR",
		},
		{
			name:    "out of range returns error",
			ply:     999,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fen, err := fenAtPly(openingPGN, tt.ply)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for ply %d, got nil", tt.ply)
				}
				return
			}
			if err != nil {
				t.Fatalf("fenAtPly: %v", err)
			}
			piecePlacement := strings.SplitN(fen, " ", 2)[0]
			if piecePlacement != tt.wantFen {
				t.Errorf("ply %d: got %q, want %q", tt.ply, piecePlacement, tt.wantFen)
			}
		})
	}
}

func TestToPuzzleRowMovesField(t *testing.T) {
	resp := &lichessPuzzleResponse{
		Puzzle: lichessPuzzleData{
			ID:         "abc123",
			Rating:     1800,
			Solution:   []string{"e2e4", "e7e5", "g1f3"},
			Themes:     []string{"opening"},
			InitialPly: 2,
		},
		Game: lichessGameData{
			ID:  "gameid1",
			PGN: openingPGN,
		},
	}
	row, err := toPuzzleRow(resp)
	if err != nil {
		t.Fatalf("toPuzzleRow: %v", err)
	}
	if row.ID != "abc123" {
		t.Errorf("ID: got %q, want %q", row.ID, "abc123")
	}
	if row.Moves != "e2e4 e7e5 g1f3" {
		t.Errorf("Moves: got %q, want %q", row.Moves, "e2e4 e7e5 g1f3")
	}
	if !strings.HasPrefix(row.GameURL, "https://lichess.org/gameid1") {
		t.Errorf("GameURL: got %q", row.GameURL)
	}
}
