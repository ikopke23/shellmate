package screens

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeName(s string) string {
	return nonAlphanumRe.ReplaceAllString(s, "-")
}

// exportPGN writes a PGN string to ~/Downloads (or working dir) and returns the full path.
func exportPGN(white, black string, playedAt time.Time, pgn string) (string, error) {
	home, err := os.UserHomeDir()
	dir := "."
	if err == nil {
		dl := filepath.Join(home, "Downloads")
		if info, err := os.Stat(dl); err == nil && info.IsDir() {
			dir = dl
		}
	}
	filename := sanitizeName(white) + "-vs-" + sanitizeName(black) + "-" + playedAt.Format("2006-01-02_15-04-05") + ".pgn"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(pgn), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
