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

const lichessNextURL = "https://lichess.org/api/puzzle/next"

var lichessClient = &http.Client{Timeout: 10 * time.Second}

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

// parseGameAt parses pgnText and returns the FEN at the given ply and the SAN moves
// leading up to it (indices 0..ply-1). positions[0] is the start; positions[n] is after n half-moves.
func parseGameAt(pgnText string, ply int) (fen string, contextSANs []string, err error) {
	pgnOpt, parseErr := chess.PGN(strings.NewReader(pgnText))
	if parseErr != nil {
		return "", nil, fmt.Errorf("parse pgn: %w", parseErr)
	}
	g := chess.NewGame(pgnOpt)
	positions := g.Positions()
	moves := g.Moves()
	if ply < 0 || ply >= len(positions) {
		return "", nil, fmt.Errorf("ply %d out of range (0..%d)", ply, len(positions)-1)
	}
	fen = positions[ply].String()
	notation := chess.AlgebraicNotation{}
	for i := 0; i < ply && i < len(moves); i++ {
		contextSANs = append(contextSANs, notation.Encode(positions[i], moves[i]))
	}
	return fen, contextSANs, nil
}

// fenAtPly replays pgnText to the given half-move ply index and returns the position FEN.
func fenAtPly(pgnText string, ply int) (string, error) {
	fen, _, err := parseGameAt(pgnText, ply)
	return fen, err
}

// fetchDailyPuzzle retrieves a random puzzle from the Lichess public API.
func fetchDailyPuzzle(ctx context.Context) (*lichessPuzzleResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lichessNextURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := lichessClient.Do(req)
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
// FEN is derived server-side from the game PGN at initialPly+1:
// Lichess's initialPly is the number of half-moves before the opponent's last forcing move.
// The PGN includes that move, so positions[initialPly+1] is the position from which
// solution[0] (the solver's first move) is valid.
// ContextMoves holds the SAN moves for all initialPly+1 moves (for display in the client sidebar).
func toPuzzleRow(resp *lichessPuzzleResponse) (*PuzzleRow, error) {
	fen, contextSANs, err := parseGameAt(resp.Game.PGN, resp.Puzzle.InitialPly+1)
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
		ID:           resp.Puzzle.ID,
		FEN:          fen,
		Moves:        strings.Join(resp.Puzzle.Solution, " "),
		ContextMoves: strings.Join(contextSANs, " "),
		Rating:       resp.Puzzle.Rating,
		RatingDev:    resp.Puzzle.RatingDeviation,
		Popularity:   resp.Puzzle.Popularity,
		NbPlays:      resp.Puzzle.NbPlays,
		Themes:       themes,
		GameURL:      gameURL,
		OpeningTags:  openingTags,
		PuzzleDate:   time.Now().UTC().Truncate(24 * time.Hour),
	}, nil
}
