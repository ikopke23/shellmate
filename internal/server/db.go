package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool.Pool and provides typed query methods.
type DB struct {
	pool *pgxpool.Pool
}

// User represents a row from the users table.
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Elo         int    `json:"elo"`
	GamesPlayed int    `json:"games_played"`
}

// GameRecord holds everything needed to write a completed game.
type GameRecord struct {
	White          string
	Black          string
	Result         string
	WhiteEloBefore int
	BlackEloBefore int
	WhiteEloAfter  int
	BlackEloAfter  int
	PGN            string
}

// HistoryRecord is one row from game history.
type HistoryRecord struct {
	ID             string    `json:"id"`
	White          string    `json:"white"`
	Black          string    `json:"black"`
	Result         string    `json:"result"`
	WhiteEloBefore int       `json:"white_elo_before"`
	BlackEloBefore int       `json:"black_elo_before"`
	WhiteEloAfter  int       `json:"white_elo_after"`
	BlackEloAfter  int       `json:"black_elo_after"`
	PGN            string    `json:"pgn,omitempty"`
	PlayedAt       time.Time `json:"played_at"`
	Imported       bool      `json:"imported"`
}

// PuzzleRow is one row from the puzzles table.
type PuzzleRow struct {
	ID           string
	FEN          string
	Moves        string
	ContextMoves string
	Rating       int
	RatingDev    int
	Popularity   int
	NbPlays      int
	Themes       []string
	GameURL      string
	OpeningTags  []string
	PuzzleDate   time.Time
}

// NewDB connects to Postgres using connStr, applies the migration at migrationSQL, and returns a DB.
// migrationSQL is the content of migrations/001_init.sql passed as a string (server reads it at startup).
func NewDB(ctx context.Context, connStr string, migrationSQL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, err
	}
	if _, err = pool.Exec(ctx, migrationSQL); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{pool: pool}, nil
}

// CreateUser inserts a new user with default Elo 1500. Returns error if username already exists.
func (d *DB) CreateUser(ctx context.Context, username string) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO users (username) VALUES ($1)`,
		username,
	)
	return err
}

// GetUser returns the user row for the given username. Returns nil, nil if not found.
func (d *DB) GetUser(ctx context.Context, username string) (*User, error) {
	u := &User{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, username, elo, games_played FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.Elo, &u.GamesPlayed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// SaveGameAndUpdateElo atomically inserts a completed game and updates both players' elo and games_played.
func (d *DB) SaveGameAndUpdateElo(ctx context.Context, g GameRecord, whiteElo, blackElo int) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx,
		`INSERT INTO games (white, black, result, white_elo_before, black_elo_before, white_elo_after, black_elo_after, pgn)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		g.White, g.Black, g.Result, g.WhiteEloBefore, g.BlackEloBefore, g.WhiteEloAfter, g.BlackEloAfter, g.PGN,
	); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`UPDATE users SET elo = $1, games_played = games_played + 1 WHERE username = $2`,
		whiteElo, g.White,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("user not found: %s", g.White)
	}
	tag, err = tx.Exec(ctx,
		`UPDATE users SET elo = $1, games_played = games_played + 1 WHERE username = $2`,
		blackElo, g.Black,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("user not found: %s", g.Black)
	}
	return tx.Commit(ctx)
}

// GetGameHistory returns the 100 most recent games where username is white or black, ordered by played_at DESC.
func (d *DB) GetGameHistory(ctx context.Context, username string) ([]HistoryRecord, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, white, black, result, white_elo_before, black_elo_before, white_elo_after, black_elo_after, pgn, played_at, imported
		 FROM games
		 WHERE white = $1 OR black = $1
		 ORDER BY played_at DESC
		 LIMIT 100`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []HistoryRecord
	for rows.Next() {
		var r HistoryRecord
		if err := rows.Scan(&r.ID, &r.White, &r.Black, &r.Result, &r.WhiteEloBefore, &r.BlackEloBefore, &r.WhiteEloAfter, &r.BlackEloAfter, &r.PGN, &r.PlayedAt, &r.Imported); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// GetLeaderboard returns the top 200 users ordered by Elo DESC.
func (d *DB) GetLeaderboard(ctx context.Context) ([]User, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, username, elo, games_played FROM users ORDER BY elo DESC LIMIT 200`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Elo, &u.GamesPlayed); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CheckUsername checks whether a user exists by username. Returns false, nil if not found.
func (d *DB) CheckUsername(ctx context.Context, username string) (bool, error) {
	u, err := d.GetUser(ctx, username)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

// SaveImportedGame inserts a game marked as imported. Creates missing users if forceCreate is true.
func (d *DB) SaveImportedGame(ctx context.Context, white, black, pgn string, forceCreate bool) error {
	if forceCreate {
		for _, name := range []string{white, black} {
			exists, err := d.CheckUsername(ctx, name)
			if err != nil {
				return err
			}
			if !exists {
				if err := d.CreateUser(ctx, name); err != nil {
					return err
				}
			}
		}
	}
	_, err := d.pool.Exec(ctx,
		`INSERT INTO games (white, black, result, white_elo_before, black_elo_before, white_elo_after, black_elo_after, pgn, imported)
		 VALUES ($1, $2, 'imported', 0, 0, 0, 0, $3, true)`,
		white, black, pgn,
	)
	return err
}

// GetImportedGames returns the 100 most recent imported games, ordered by played_at DESC.
func (d *DB) GetImportedGames(ctx context.Context) ([]HistoryRecord, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, white, black, result, white_elo_before, black_elo_before, white_elo_after, black_elo_after, pgn, played_at, imported
		 FROM games
		 WHERE imported = true
		 ORDER BY played_at DESC
		 LIMIT 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []HistoryRecord
	for rows.Next() {
		var r HistoryRecord
		if err := rows.Scan(&r.ID, &r.White, &r.Black, &r.Result, &r.WhiteEloBefore, &r.BlackEloBefore, &r.WhiteEloAfter, &r.BlackEloAfter, &r.PGN, &r.PlayedAt, &r.Imported); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// SavePuzzle inserts a puzzle into the cache. Silently skips if the ID already exists.
func (d *DB) SavePuzzle(ctx context.Context, p PuzzleRow) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO puzzles (id, fen, moves, context_moves, rating, rating_dev, popularity, nb_plays, themes, game_url, opening_tags, puzzle_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (id) DO NOTHING`,
		p.ID, p.FEN, p.Moves, p.ContextMoves, p.Rating, p.RatingDev, p.Popularity, p.NbPlays,
		p.Themes, p.GameURL, p.OpeningTags, p.PuzzleDate,
	)
	return err
}

// GetNextPuzzle returns an unseen puzzle for the user, or nil if none are cached.
func (d *DB) GetNextPuzzle(ctx context.Context, username string) (*PuzzleRow, error) {
	p := &PuzzleRow{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, fen, moves, context_moves, rating, rating_dev, popularity, nb_plays, themes, game_url, opening_tags, puzzle_date
		 FROM puzzles
		 WHERE id NOT IN (SELECT puzzle_id FROM user_puzzle_attempts WHERE username = $1)
		 ORDER BY fetched_at ASC
		 LIMIT 1`,
		username,
	).Scan(&p.ID, &p.FEN, &p.Moves, &p.ContextMoves, &p.Rating, &p.RatingDev, &p.Popularity, &p.NbPlays,
		&p.Themes, &p.GameURL, &p.OpeningTags, &p.PuzzleDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CountUnseenPuzzles returns how many cached puzzles the user has not yet attempted.
func (d *DB) CountUnseenPuzzles(ctx context.Context, username string) (int, error) {
	var n int
	err := d.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM puzzles
		 WHERE id NOT IN (SELECT puzzle_id FROM user_puzzle_attempts WHERE username = $1)`,
		username,
	).Scan(&n)
	return n, err
}

// RecordAttemptAndUpdateRating atomically records a puzzle attempt and updates the user's puzzle rating.
// When skipped is true the rating is not updated (skips are tracked for stats but don't affect Elo).
func (d *DB) RecordAttemptAndUpdateRating(ctx context.Context, username, puzzleID string, solved, skipped bool, newRating int) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx,
		`INSERT INTO user_puzzle_attempts (username, puzzle_id, solved, skipped) VALUES ($1, $2, $3, $4)`,
		username, puzzleID, solved, skipped,
	); err != nil {
		return err
	}
	if !skipped {
		if _, err = tx.Exec(ctx,
			`UPDATE users SET puzzle_rating = $1 WHERE username = $2`,
			newRating, username,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// GetPuzzleRating returns the user's current puzzle rating. Returns 1500 if user not found.
func (d *DB) GetPuzzleRating(ctx context.Context, username string) (int, error) {
	var rating int
	err := d.pool.QueryRow(ctx,
		`SELECT puzzle_rating FROM users WHERE username = $1`,
		username,
	).Scan(&rating)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1500, nil
	}
	return rating, err
}

// GetPuzzleByID returns the puzzle with the given ID, or nil if not found.
func (d *DB) GetPuzzleByID(ctx context.Context, id string) (*PuzzleRow, error) {
	p := &PuzzleRow{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, fen, moves, context_moves, rating, rating_dev, popularity, nb_plays, themes, game_url, opening_tags, puzzle_date
		 FROM puzzles WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.FEN, &p.Moves, &p.ContextMoves, &p.Rating, &p.RatingDev, &p.Popularity, &p.NbPlays,
		&p.Themes, &p.GameURL, &p.OpeningTags, &p.PuzzleDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// BulkSavePuzzles inserts a batch of puzzles in a single multi-row INSERT.
// ON CONFLICT (id) DO NOTHING makes it safe to re-run imports.
func (d *DB) BulkSavePuzzles(ctx context.Context, puzzles []PuzzleRow) (err error) {
	if len(puzzles) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, p := range puzzles {
		batch.Queue(
			`INSERT INTO puzzles (id, fen, moves, context_moves, rating, rating_dev, popularity, nb_plays, themes, game_url, opening_tags, puzzle_date)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			 ON CONFLICT (id) DO NOTHING`,
			p.ID, p.FEN, p.Moves, p.ContextMoves, p.Rating, p.RatingDev, p.Popularity, p.NbPlays,
			p.Themes, p.GameURL, p.OpeningTags, p.PuzzleDate,
		)
	}
	br := d.pool.SendBatch(ctx, batch)
	defer func() {
		if closeErr := br.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for range puzzles {
		if _, execErr := br.Exec(); execErr != nil {
			return execErr
		}
	}
	return nil
}
