package shared_test

import (
	"encoding/json"
	"testing"

	"github.com/ikopke/shellmate/internal/shared"
)

func TestMoveMsgRoundTrip(t *testing.T) {
	msg := shared.MoveMsg{
		GameID: "abc",
		Moves:  []string{"e4", "e5"},
		Clock:  shared.ClockState{WhiteMs: 60000, BlackMs: 59000},
	}
	data, err := shared.Encode(shared.MsgMove, msg)
	if err != nil {
		t.Fatal(err)
	}
	env, err := shared.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	var got shared.MoveMsg
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.GameID != msg.GameID || got.Clock.WhiteMs != 60000 || len(got.Moves) != 2 {
		t.Fatalf("unexpected round-trip result: %+v", got)
	}
}

func TestTimeControl_UntimedSentinelJSON(t *testing.T) {
	// A missing/zero initial_seconds field should unmarshal to the untimed sentinel.
	data := []byte(`{"initial_seconds":0,"increment_seconds":0}`)
	var tc shared.TimeControl
	if err := json.Unmarshal(data, &tc); err != nil {
		t.Fatal(err)
	}
	if tc.InitialSeconds != 0 {
		t.Fatalf("expected untimed sentinel (0), got %d", tc.InitialSeconds)
	}
}
