package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/notnil/chess"
)

const lichessDailyURL = "https://lichess.org/api/puzzle/daily" //nolint:unused

type lichessPuzzleData struct {
	ID              string   `json:"id"`
	Rating          int      `json:"rating"`
	RatingDeviation int      `json:"ratingDeviation"`
	Popularity      int      `json:"popularity"`
	NbPlays         int      `json:"plays"`
	Solution        []string `json:"solution"`
	Themes          []string `json:"themes"`
	OpeningTags     []string `json:"openingTags"`
	InitialPly      int      `json:"initialPly"`
}

type lichessGameData struct {
	ID  string `json:"id"`
	PGN string `json:"pgn"`
}

type lichessPuzzleResponse struct {
	Puzzle lichessPuzzleData `json:"puzzle"`
	Game   lichessGameData   `json:"game"`
}

// fenAtPly replays pgnText to the given half-move ply index and returns the position FEN.
// positions[0] is the start position; positions[n] is after n half-moves.
func fenAtPly(pgnText string, ply int) (string, error) {
	pgnOpt, err := chess.PGN(strings.NewReader(pgnText))
	if err != nil {
		return "", fmt.Errorf("parse pgn: %w", err)
	}
	g := chess.NewGame(pgnOpt)
	positions := g.Positions()
	if ply < 0 || ply >= len(positions) {
		return "", fmt.Errorf("ply %d out of range (0..%d)", ply, len(positions)-1)
	}
	return positions[ply].String(), nil
}

// fetchDailyPuzzle retrieves today's puzzle from the Lichess public API.
func fetchDailyPuzzle(ctx context.Context) (*lichessPuzzleResponse, error) { //nolint:unused
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lichessDailyURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lichess api returned %d", resp.StatusCode)
	}
	var result lichessPuzzleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// toPuzzleRow converts a Lichess API response to a PuzzleRow ready for storage.
// FEN is derived server-side from the game PGN at initialPly.
func toPuzzleRow(resp *lichessPuzzleResponse) (*PuzzleRow, error) {
	fen, err := fenAtPly(resp.Game.PGN, resp.Puzzle.InitialPly)
	if err != nil {
		return nil, fmt.Errorf("derive fen: %w", err)
	}
	gameURL := ""
	if resp.Game.ID != "" {
		gameURL = fmt.Sprintf("https://lichess.org/%s#%d", resp.Game.ID, resp.Puzzle.InitialPly)
	}
	themes := resp.Puzzle.Themes
	if themes == nil {
		themes = []string{}
	}
	openingTags := resp.Puzzle.OpeningTags
	if openingTags == nil {
		openingTags = []string{}
	}
	return &PuzzleRow{
		ID:          resp.Puzzle.ID,
		FEN:         fen,
		Moves:       strings.Join(resp.Puzzle.Solution, " "),
		Rating:      resp.Puzzle.Rating,
		RatingDev:   resp.Puzzle.RatingDeviation,
		Popularity:  resp.Puzzle.Popularity,
		NbPlays:     resp.Puzzle.NbPlays,
		Themes:      themes,
		GameURL:     gameURL,
		OpeningTags: openingTags,
		PuzzleDate:  time.Now().UTC().Truncate(24 * time.Hour),
	}, nil
}
