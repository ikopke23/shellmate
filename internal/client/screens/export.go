package screens

import (
	"encoding/base64"
	"regexp"
	"time"
)

var nonAlphanumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeName(s string) string {
	return nonAlphanumRe.ReplaceAllString(s, "-")
}

// pgnClipboardOSC returns an OSC 52 terminal escape sequence that copies pgn
// to the SSH client's clipboard, and a human-readable filename for the status line.
func pgnClipboardOSC(white, black string, playedAt time.Time, pgn string) (osc, filename string) {
	filename = sanitizeName(white) + "-vs-" + sanitizeName(black) + "-" + playedAt.Format("2006-01-02_15-04-05") + ".pgn"
	enc := base64.StdEncoding.EncodeToString([]byte(pgn))
	osc = "\x1b]52;c;" + enc + "\x07"
	return osc, filename
}
