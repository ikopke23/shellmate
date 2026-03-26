package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/logging"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ikopke/shellmate/internal/client"
	"github.com/ikopke/shellmate/internal/server"
)

func newTeaHandler(hub *server.Hub) bm.Handler {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		pty, _, active := s.Pty()
		if !active {
			return nil, nil
		}
		fingerprint := gossh.FingerprintSHA256(s.PublicKey())
		ctx := s.Context()
		w, h := pty.Window.Width, pty.Window.Height
		opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
		user, err := hub.GetUserByKeyFingerprint(ctx, fingerprint)
		if err != nil {
			slog.Error("fingerprint lookup failed", "err", err)
			return nil, nil
		}
		if user == nil {
			return client.NewRegistrationModel(hub, fingerprint, w, h), opts
		}
		c := hub.Register(user.Username)
		hub.BroadcastLobby(ctx)
		return client.NewModel(hub, c, user, w, h), opts
	}
}

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
		slog.Info("starting puzzle import", "path", *importPath)
		processed, inserted, skipped, importErr := server.RunImport(ctx, importDB, *importPath)
		if importErr != nil {
			importDB.Close()
			slog.Error("import failed", "error", importErr)
			os.Exit(1)
		}
		slog.Info("import complete", "processed", processed, "inserted", inserted, "skipped", skipped)
		importDB.Close()
		return
	}

	inviteCode := os.Getenv("INVITE_CODE")
	if inviteCode == "" {
		slog.Error("INVITE_CODE environment variable is required")
		os.Exit(1)
	}
	sshPort := os.Getenv("SSH_PORT")
	if sshPort == "" {
		sshPort = ":2222"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, err := server.NewDB(ctx, dbURL, migrationSQL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	hub := server.NewHub(db, inviteCode)
	s, err := wish.NewServer(
		wish.WithAddress(sshPort),
		wish.WithHostKeyPath(".ssh/shellmate_host_key"),
		wish.WithMiddleware(
			bm.Middleware(newTeaHandler(hub)),
			logging.Middleware(),
		),
	)
	if err != nil {
		slog.Error("failed to create SSH server", "err", err)
		os.Exit(1)
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	slog.Info("SSH server starting", "addr", sshPort)
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			slog.Error("server error", "err", err)
		}
	}()
	<-done
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	_ = s.Shutdown(shutdownCtx)
}

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
