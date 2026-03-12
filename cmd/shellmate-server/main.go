package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ikopke/shellmate/internal/server"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
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
	migrationSQL, err := readMigration()
	if err != nil {
		slog.Error("failed to read migration file", "error", err)
		os.Exit(1)
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

func readMigration() (string, error) {
	execPath, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(execPath), "migrations", "001_init.sql")
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data), nil
		}
	}
	data, err := os.ReadFile("./migrations/001_init.sql")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
