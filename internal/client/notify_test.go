package client

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ikopke/shellmate/internal/shared"
)

func TestShouldNotifyJoin(t *testing.T) {
	const me = "alice"
	tests := []struct {
		name    string
		focused bool
		msg     shared.GameStart
		want    bool
	}{
		{"blurred and white player", false, shared.GameStart{White: me, Black: "bob"}, true},
		{"blurred and black player", false, shared.GameStart{White: "bob", Black: me}, true},
		{"focused suppresses alert", true, shared.GameStart{White: me, Black: "bob"}, false},
		{"blurred but only a spectator", false, shared.GameStart{White: "bob", Black: "carol"}, false},
		{"focused and not a player", true, shared.GameStart{White: "bob", Black: "carol"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldNotifyJoin(tt.focused, tt.msg, me); got != tt.want {
				t.Errorf("shouldNotifyJoin(%v, %+v, %q) = %v, want %v", tt.focused, tt.msg, me, got, tt.want)
			}
		})
	}
}

func TestNotificationSeq(t *testing.T) {
	seq := notificationSeq("hello")
	if !strings.HasPrefix(seq, bel) {
		t.Errorf("sequence should start with BEL for the urgency hint, got %q", seq)
	}
	if want := "\x1b]9;hello\x07"; !strings.Contains(seq, want) {
		t.Errorf("sequence missing OSC 9 notification %q, got %q", want, seq)
	}
}

func TestNotify(t *testing.T) {
	var buf bytes.Buffer
	notify(&buf, "ping")
	if got := buf.String(); got != notificationSeq("ping") {
		t.Errorf("notify wrote %q, want %q", got, notificationSeq("ping"))
	}
	// nil writer must be a no-op, not a panic.
	notify(nil, "ping")
}
