package screens

import (
	"strings"
	"testing"
	"time"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"alice", "alice"},
		{"a b/c", "a-b-c"},
		{"", ""},
	}
	for _, tc := range tests {
		got := sanitizeName(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPgnClipboardOSC_FilenameContainsPlayersAndDate(t *testing.T) {
	playedAt := time.Date(2026, 4, 16, 12, 30, 45, 0, time.UTC)
	osc, filename := pgnClipboardOSC("alice", "bob", playedAt, "1. e4 e5")
	if !strings.Contains(filename, "alice") {
		t.Errorf("filename = %q, want it to contain %q", filename, "alice")
	}
	if !strings.Contains(filename, "bob") {
		t.Errorf("filename = %q, want it to contain %q", filename, "bob")
	}
	if !strings.Contains(filename, "2026-04-16") {
		t.Errorf("filename = %q, want it to contain date %q", filename, "2026-04-16")
	}
	if osc == "" {
		t.Error("osc is empty")
	}
	if !strings.HasPrefix(osc, "\x1b]52;") {
		t.Errorf("osc = %q, want prefix %q", osc, "\\x1b]52;")
	}
}
