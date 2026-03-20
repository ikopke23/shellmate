package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ikopke/shellmate/internal/server"
)

func main() {
	importPath := flag.String("import-puzzles", "", "path to Lichess puzzle CSV to import (server will not start)")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
	}
	migrationSQL, err := readMigrations()
	if err != nil {
		slog.Error("failed to read migration file", "error", err)
		os.Exit(1)
	}

	if *importPath != "" {
		ctx := context.Background()
		importDB, importErr := server.NewDB(ctx, dbURL, migrationSQL)
		if importErr != nil {
			slog.Error("failed to connect to database", "error", importErr)
			os.Exit(1)
		}
		defer importDB.Close()
		slog.Info("starting puzzle import", "path", *importPath)
		processed, inserted, skipped, importErr := server.RunImport(ctx, importDB, *importPath)
		if importErr != nil {
			slog.Error("import failed", "error", importErr)
			os.Exit(1)
		}
		slog.Info("import complete", "processed", processed, "inserted", inserted, "skipped", skipped)
		return
	}

	inviteCode := os.Getenv("INVITE_CODE")
	if inviteCode == "" {
		slog.Error("INVITE_CODE environment variable is required")
		os.Exit(1)
	}
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, err := server.NewDB(ctx, dbURL, migrationSQL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	hub := server.NewHub(db, inviteCode)
	handler := server.NewHandler(hub)
	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: handler,
	}
	slog.Info("starting shellmate server", "addr", listenAddr)
	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErr:
		slog.Error("http server error", "error", err)
		cancel()
		return
	case <-quit:
		slog.Info("shutting down")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func readMigrations() (string, error) {
	files := []string{"001_init.sql", "002_imported.sql", "003_puzzles.sql", "004_puzzle_skipped.sql", "005_context_moves.sql", "006_puzzle_rating_index.sql"}
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
		if data, err := os.ReadFile(p); err == nil {
			return string(data), nil
		}
	}
	data, err := os.ReadFile(filepath.Join("./migrations", name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
