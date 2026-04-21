package main

import (
	"os"
	"path/filepath"
	"strings"
)

func readMigrations() (string, error) {
	files := []string{"001_init.sql", "002_imported.sql", "003_puzzles.sql", "004_puzzle_skipped.sql", "005_context_moves.sql", "006_puzzle_rating_index.sql", "007_ssh_auth.sql"}
	var combined strings.Builder
	for _, f := range files {
		data, err := readMigrationFile(f)
		if err != nil {
			return "", err
		}
		combined.WriteString(data)
		combined.WriteString("\n")
	}
	return combined.String(), nil
}

func readMigrationFile(name string) (string, error) {
	execPath, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(execPath), "migrations", name)
		if data, err := os.ReadFile(p); err == nil { //nolint:gosec // migration filenames are baked-in constants, not user input
			return string(data), nil
		}
	}
	data, err := os.ReadFile(filepath.Join("migrations", name)) //nolint:gosec // migration filenames are baked-in constants, not user input
	if err != nil {
		return "", err
	}
	return string(data), nil
}
