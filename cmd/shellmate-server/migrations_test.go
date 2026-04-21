package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMigration(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, "migrations", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestReadMigrationFile_FromCurrentDir(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "dummy.sql", "-- dummy content\n")
	t.Chdir(dir)
	data, err := readMigrationFile("dummy.sql")
	if err != nil {
		t.Fatalf("readMigrationFile: %v", err)
	}
	if data != "-- dummy content\n" {
		t.Errorf("got %q, want %q", data, "-- dummy content\n")
	}
}

func TestReadMigrationFile_NonExistent_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if _, err := readMigrationFile("missing.sql"); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestReadMigrations_ConcatenatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	files := []string{"001_init.sql", "002_imported.sql", "003_puzzles.sql", "004_puzzle_skipped.sql", "005_context_moves.sql", "006_puzzle_rating_index.sql", "007_ssh_auth.sql"}
	for _, f := range files {
		writeMigration(t, dir, f, "-- "+f+"\n")
	}
	t.Chdir(dir)
	combined, err := readMigrations()
	if err != nil {
		t.Fatalf("readMigrations: %v", err)
	}
	for _, f := range files {
		if !strings.Contains(combined, "-- "+f) {
			t.Errorf("combined output missing %q", f)
		}
	}
}

func TestReadMigrations_MissingFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_init.sql", "-- 001\n")
	t.Chdir(dir)
	if _, err := readMigrations(); err == nil {
		t.Error("expected error for missing migration file, got nil")
	}
}
