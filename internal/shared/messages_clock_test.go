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

func TestTimeControlSentinel(t *testing.T) {
	tc := shared.TimeControl{InitialSeconds: 0, IncrementSeconds: 0}
	if tc.InitialSeconds != 0 {
		t.Fatal("untimed sentinel should have InitialSeconds == 0")
	}
}
