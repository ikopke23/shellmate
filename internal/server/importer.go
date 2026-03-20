package server

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	importMinRating = 800
	importMaxRating = 2800
	importBatchSize = 500
)

// parsePuzzleCSVRow converts a slice of CSV fields (one row from the Lichess puzzle CSV)
// into a PuzzleRow. Returns an error for wrong column count or unparseable integers.
// The Lichess CSV columns in order:
//
//	PuzzleId, FEN, Moves, Rating, RatingDeviation, Popularity, NbPlays, Themes, GameUrl, OpeningTags
func parsePuzzleCSVRow(cols []string) (*PuzzleRow, error) {
	if len(cols) < 10 {
		return nil, fmt.Errorf("expected 10 columns, got %d", len(cols))
	}
	rating, err := strconv.Atoi(cols[3])
	if err != nil {
		return nil, fmt.Errorf("invalid rating %q: %w", cols[3], err)
	}
	ratingDev, err := strconv.Atoi(cols[4])
	if err != nil {
		return nil, fmt.Errorf("invalid rating_dev %q: %w", cols[4], err)
	}
	popularity, err := strconv.Atoi(cols[5])
	if err != nil {
		return nil, fmt.Errorf("invalid popularity %q: %w", cols[5], err)
	}
	nbPlays, err := strconv.Atoi(cols[6])
	if err != nil {
		return nil, fmt.Errorf("invalid nb_plays %q: %w", cols[6], err)
	}
	themes := strings.Fields(cols[7])
	openingTags := strings.Fields(cols[9])
	return &PuzzleRow{
		ID:           cols[0],
		FEN:          cols[1],
		Moves:        cols[2],
		ContextMoves: "",
		Rating:       rating,
		RatingDev:    ratingDev,
		Popularity:   popularity,
		NbPlays:      nbPlays,
		Themes:       themes,
		GameURL:      cols[8],
		OpeningTags:  openingTags,
		PuzzleDate:   time.Now().UTC().Truncate(24 * time.Hour),
	}, nil
}

// RunImport streams the Lichess puzzle CSV at csvPath, filters by rating band,
// and bulk-inserts into the DB in batches of importBatchSize.
// Malformed rows are logged and skipped. Any DB error causes immediate exit.
// Returns counts of (processed, inserted, skipped).
func RunImport(ctx context.Context, db *DB, csvPath string) (processed, inserted, skipped int, err error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close() //nolint:errcheck
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // allow variable columns; we validate manually
	// Skip header row
	if _, err := r.Read(); err != nil {
		return 0, 0, 0, fmt.Errorf("read header: %w", err)
	}
	var batch []PuzzleRow
	for {
		cols, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			slog.Warn("skipping malformed CSV row", "error", readErr)
			skipped++
			processed++
			continue
		}
		processed++
		row, parseErr := parsePuzzleCSVRow(cols)
		if parseErr != nil {
			slog.Warn("skipping unparseable row", "error", parseErr)
			skipped++
			continue
		}
		if row.Rating < importMinRating || row.Rating > importMaxRating {
			skipped++
			continue
		}
		batch = append(batch, *row)
		if len(batch) >= importBatchSize {
			if bulkErr := db.BulkSavePuzzles(ctx, batch); bulkErr != nil {
				return processed, inserted, skipped, fmt.Errorf("bulk insert: %w", bulkErr)
			}
			inserted += len(batch)
			batch = batch[:0]
		}
		if processed%10000 == 0 {
			slog.Info("import progress", "processed", processed, "inserted", inserted, "skipped", skipped)
		}
	}
	if len(batch) > 0 {
		if bulkErr := db.BulkSavePuzzles(ctx, batch); bulkErr != nil {
			return processed, inserted, skipped, fmt.Errorf("bulk insert final batch: %w", bulkErr)
		}
		inserted += len(batch)
	}
	return processed, inserted, skipped, nil
}
