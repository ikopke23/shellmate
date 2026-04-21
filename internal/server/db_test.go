package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testDB connects to the PostgreSQL instance specified by SHELLMATE_TEST_POSTGRES_*
// env vars. Skips the test if env vars are absent. Closed via t.Cleanup.
func testDB(t *testing.T) *DB {
	t.Helper()
	host := os.Getenv("SHELLMATE_TEST_POSTGRES_HOST")
	user := os.Getenv("SHELLMATE_TEST_POSTGRES_USER")
	pass := os.Getenv("SHELLMATE_TEST_POSTGRES_PASS")
	if host == "" {
		t.Skip("SHELLMATE_TEST_POSTGRES_HOST not set — skipping DB integration tests")
	}
	connStr := fmt.Sprintf("postgresql://%s:%s@%s/shellmate_test?sslmode=disable", user, pass, host)
	migSQL := readAllMigrations(t)
	ctx := context.Background()
	db, err := NewDB(ctx, connStr, migSQL)
	if err != nil {
		t.Fatalf("testDB: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

// readAllMigrations reads and concatenates all .sql files in migrations/ in order.
func readAllMigrations(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatalf("readAllMigrations: %v", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("../../migrations", e.Name()))
		if err != nil {
			t.Fatalf("readAllMigrations %s: %v", e.Name(), err)
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// truncateAll removes all rows from every table to isolate tests.
func truncateAll(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	_, err := db.pool.Exec(ctx,
		"TRUNCATE users, user_ssh_keys, games, puzzles, user_puzzle_attempts CASCADE")
	if err != nil {
		t.Fatalf("truncateAll: %v", err)
	}
}

func TestDB_NewDB(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	// simple ping to verify connection
	if err := db.pool.Ping(ctx); err != nil {
		t.Fatalf("ping after NewDB: %v", err)
	}
}

func TestDB_CreateUser_GetUser(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	u, err := db.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u == nil {
		t.Fatal("GetUser returned nil for existing user")
	}
	if u.Username != "alice" {
		t.Fatalf("username = %q, want %q", u.Username, "alice")
	}
	if u.Elo != 1500 {
		t.Fatalf("elo = %d, want 1500", u.Elo)
	}
	if u.GamesPlayed != 0 {
		t.Fatalf("games_played = %d, want 0", u.GamesPlayed)
	}
}

func TestDB_GetUser_NotFound(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	u, err := db.GetUser(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u != nil {
		t.Fatal("expected nil for non-existent user")
	}
}

func TestDB_CreateUserWithKey_GetByFingerprint(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	created, err := db.CreateUserWithKey(ctx, "bob", "fp:bob:1")
	if err != nil {
		t.Fatalf("CreateUserWithKey: %v", err)
	}
	if created == nil || created.Username != "bob" {
		t.Fatalf("CreateUserWithKey returned unexpected user: %v", created)
	}
	u, err := db.GetUserByKeyFingerprint(ctx, "fp:bob:1")
	if err != nil {
		t.Fatalf("GetUserByKeyFingerprint: %v", err)
	}
	if u == nil || u.Username != "bob" {
		t.Fatalf("expected user bob, got %v", u)
	}
}

func TestDB_GetUserByKeyFingerprint_NotFound(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	u, err := db.GetUserByKeyFingerprint(ctx, "nonexistent-fp")
	if err != nil {
		t.Fatalf("GetUserByKeyFingerprint: %v", err)
	}
	if u != nil {
		t.Fatal("expected nil for non-existent fingerprint")
	}
}

func TestDB_LinkKeyToUser(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "carol"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := db.LinkKeyToUser(ctx, "carol", "fp:carol:2"); err != nil {
		t.Fatalf("LinkKeyToUser: %v", err)
	}
	u, err := db.GetUserByKeyFingerprint(ctx, "fp:carol:2")
	if err != nil {
		t.Fatalf("GetUserByKeyFingerprint: %v", err)
	}
	if u == nil || u.Username != "carol" {
		t.Fatalf("expected carol, got %v", u)
	}
}

func TestDB_CheckUsername(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "dave"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	exists, err := db.CheckUsername(ctx, "dave")
	if err != nil {
		t.Fatalf("CheckUsername: %v", err)
	}
	if !exists {
		t.Fatal("CheckUsername returned false for existing user")
	}
	exists, err = db.CheckUsername(ctx, "unknown")
	if err != nil {
		t.Fatalf("CheckUsername: %v", err)
	}
	if exists {
		t.Fatal("CheckUsername returned true for non-existent user")
	}
}

func TestDB_SaveGameAndUpdateElo(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "white"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateUser(ctx, "black"); err != nil {
		t.Fatal(err)
	}
	rec := GameRecord{
		White: "white", Black: "black", Result: "1-0",
		WhiteEloBefore: 1500, BlackEloBefore: 1500,
		WhiteEloAfter: 1516, BlackEloAfter: 1484,
		PGN: "1. e4 e5 *",
	}
	if err := db.SaveGameAndUpdateElo(ctx, rec, 1516, 1484); err != nil {
		t.Fatalf("SaveGameAndUpdateElo: %v", err)
	}
	w, _ := db.GetUser(ctx, "white")
	if w.Elo != 1516 {
		t.Fatalf("white elo = %d, want 1516", w.Elo)
	}
	if w.GamesPlayed != 1 {
		t.Fatalf("white games_played = %d, want 1", w.GamesPlayed)
	}
	history, err := db.GetGameHistory(ctx, "white")
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history))
	}
}

func TestDB_GetLeaderboard(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	for _, u := range []string{"p1", "p2", "p3"} {
		if err := db.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	// manually bump p1's elo higher
	_, err := db.pool.Exec(ctx, "UPDATE users SET elo = 1800 WHERE username = 'p1'")
	if err != nil {
		t.Fatalf("UPDATE elo: %v", err)
	}
	lb, err := db.GetLeaderboard(ctx)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(lb) < 3 {
		t.Fatalf("expected at least 3 entries, got %d", len(lb))
	}
	if lb[0].Username != "p1" {
		t.Fatalf("expected p1 first in leaderboard, got %s", lb[0].Username)
	}
}

func TestDB_SaveImportedGame_GetImportedGames(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	// forceCreate=true creates users automatically
	if err := db.SaveImportedGame(ctx, "imp1", "imp2", "1. e4 *", true); err != nil {
		t.Fatalf("SaveImportedGame: %v", err)
	}
	games, err := db.GetImportedGames(ctx)
	if err != nil {
		t.Fatalf("GetImportedGames: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 imported game, got %d", len(games))
	}
	if !games[0].Imported {
		t.Fatal("expected Imported == true")
	}
}

func TestDB_SaveImportedGame_ForceCreateFalse_ExistingUsers(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "w"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateUser(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveImportedGame(ctx, "w", "b", "1. d4 *", false); err != nil {
		t.Fatalf("SaveImportedGame(forceCreate=false): %v", err)
	}
	games, _ := db.GetImportedGames(ctx)
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
}

func TestDB_SavePuzzle_GetPuzzleByID(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	p := PuzzleRow{
		ID: "pzl1", FEN: "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		Moves: "d7d5", Rating: 1500, PuzzleDate: time.Now().UTC().Truncate(24 * time.Hour),
		Themes: []string{"opening"}, OpeningTags: []string{},
	}
	if err := db.SavePuzzle(ctx, p); err != nil {
		t.Fatalf("SavePuzzle: %v", err)
	}
	got, err := db.GetPuzzleByID(ctx, "pzl1")
	if err != nil {
		t.Fatalf("GetPuzzleByID: %v", err)
	}
	if got == nil || got.ID != "pzl1" {
		t.Fatalf("expected puzzle pzl1, got %v", got)
	}
}

func TestDB_GetPuzzleByID_NotFound(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	got, err := db.GetPuzzleByID(ctx, "missing")
	if err != nil {
		t.Fatalf("GetPuzzleByID: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent puzzle")
	}
}

func TestDB_BulkSavePuzzles_GetNextPuzzle(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "solver"); err != nil {
		t.Fatal(err)
	}
	base := PuzzleRow{
		FEN:   "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		Moves: "d7d5", PuzzleDate: time.Now().UTC().Truncate(24 * time.Hour),
		Themes: []string{"opening"}, OpeningTags: []string{},
	}
	puzzles := []PuzzleRow{
		{ID: "bulk1", Rating: 1500, FEN: base.FEN, Moves: base.Moves, PuzzleDate: base.PuzzleDate, Themes: base.Themes, OpeningTags: base.OpeningTags},
		{ID: "bulk2", Rating: 1600, FEN: base.FEN, Moves: base.Moves, PuzzleDate: base.PuzzleDate, Themes: base.Themes, OpeningTags: base.OpeningTags},
		{ID: "bulk3", Rating: 1400, FEN: base.FEN, Moves: base.Moves, PuzzleDate: base.PuzzleDate, Themes: base.Themes, OpeningTags: base.OpeningTags},
	}
	if err := db.BulkSavePuzzles(ctx, puzzles); err != nil {
		t.Fatalf("BulkSavePuzzles: %v", err)
	}
	p, err := db.GetNextPuzzle(ctx, "solver", 1500)
	if err != nil {
		t.Fatalf("GetNextPuzzle: %v", err)
	}
	if p == nil {
		t.Fatal("expected a puzzle, got nil")
	}
}

func TestDB_GetNextPuzzle_ExhaustedPool(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "solver2"); err != nil {
		t.Fatal(err)
	}
	// no puzzles in DB
	p, err := db.GetNextPuzzle(ctx, "solver2", 1500)
	if err != nil {
		t.Fatalf("GetNextPuzzle: %v", err)
	}
	if p != nil {
		t.Fatalf("expected nil when no puzzles available, got %v", p)
	}
}

func TestDB_RecordAttemptAndUpdateRating(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	if err := db.CreateUser(ctx, "rater"); err != nil {
		t.Fatal(err)
	}
	base := PuzzleRow{
		ID: "rate1", Rating: 1500,
		FEN:   "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		Moves: "d7d5", PuzzleDate: time.Now().UTC().Truncate(24 * time.Hour),
		Themes: []string{}, OpeningTags: []string{},
	}
	if err := db.SavePuzzle(ctx, base); err != nil {
		t.Fatal(err)
	}
	initialRating, err := db.GetPuzzleRating(ctx, "rater")
	if err != nil {
		t.Fatalf("GetPuzzleRating: %v", err)
	}
	newRating := PuzzleEloOutcome(initialRating, 1500, true)
	if err := db.RecordAttemptAndUpdateRating(ctx, "rater", "rate1", true, false, newRating); err != nil {
		t.Fatalf("RecordAttemptAndUpdateRating: %v", err)
	}
	updatedRating, err := db.GetPuzzleRating(ctx, "rater")
	if err != nil {
		t.Fatalf("GetPuzzleRating after attempt: %v", err)
	}
	if updatedRating == initialRating {
		t.Fatalf("expected rating to change after solving, still %d", updatedRating)
	}
	count, err := db.CountUnseenPuzzles(ctx, "rater")
	if err != nil {
		t.Fatalf("CountUnseenPuzzles: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unseen puzzles after solving all, got %d", count)
	}
}

func TestDB_GetPuzzleRating_DefaultWhenNotFound(t *testing.T) {
	db := testDB(t)
	truncateAll(t, db)
	ctx := context.Background()
	// user doesn't exist → GetPuzzleRating returns 1500 default
	rating, err := db.GetPuzzleRating(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetPuzzleRating: %v", err)
	}
	if rating != 1500 {
		t.Fatalf("expected default rating 1500, got %d", rating)
	}
}
