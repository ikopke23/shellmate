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
	defer tx.Rollback(ctx)
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
		`SELECT id, white, black, result, white_elo_before, black_elo_before, white_elo_after, black_elo_after, pgn, played_at
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
		if err := rows.Scan(&r.ID, &r.White, &r.Black, &r.Result, &r.WhiteEloBefore, &r.BlackEloBefore, &r.WhiteEloAfter, &r.BlackEloAfter, &r.PGN, &r.PlayedAt); err != nil {
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
