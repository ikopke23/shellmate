package client

import (
	"fmt"
	"io"

	"github.com/ikopke/shellmate/internal/shared"
)

// bel is the terminal BEL. Most terminal emulators turn BEL into a
// window-manager urgency hint (taskbar flash) when their own window is
// unfocused, so it works as an "alert when not looking" even on terminals
// that don't support focus reporting or desktop notifications.
const bel = "\a"

// notificationSeq builds a best-effort terminal alert: a BEL for the urgency
// hint, followed by an OSC 9 desktop notification (ESC ] 9 ; <body> BEL),
// which iTerm2, WezTerm, and other OSC-9-aware terminals surface as a real
// desktop notification. Terminals that don't understand OSC 9 ignore it
// harmlessly; the sequence moves no cursor, so it can't corrupt the render.
func notificationSeq(body string) string {
	return bel + fmt.Sprintf("\x1b]9;%s\x07", body)
}

// shouldNotifyJoin reports whether an opponent joining should raise a terminal
// alert. We alert only when the local terminal is not focused and the user is
// actually a player in the game (not a spectator). Gating on focus rather than
// color is robust: the player who just clicked join is focused and is not
// alerted, while the waiting creator who walked away is.
func shouldNotifyJoin(focused bool, msg shared.GameStart, username string) bool {
	if focused {
		return false
	}
	return msg.White == username || msg.Black == username
}

// notify writes a terminal alert to w, ignoring errors (best-effort).
func notify(w io.Writer, body string) {
	if w == nil {
		return
	}
	_, _ = io.WriteString(w, notificationSeq(body))
}
