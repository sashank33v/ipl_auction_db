package handlers

import "testing"

func TestSocketOrigin(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   bool
	}{{nil, true}, {[]string{"http://localhost:3000"}, true}, {[]string{"https://evil.example"}, false}, {[]string{"null"}, false}, {[]string{"http://localhost:3000", "http://localhost:3000"}, false}} {
		if got := allowedSocketOrigin(tc.values, "http://localhost:3000"); got != tc.want {
			t.Fatalf("origin %v: %v", tc.values, got)
		}
	}
}
func TestStrictSocketJSON(t *testing.T) {
	for _, tc := range []struct {
		raw string
		ok  bool
	}{{`{"type":"REQUEST_SNAPSHOT"}`, true}, {`{"type":"BID_PLACE","payload":{}}`, true}, {`{"type":"BID_PLACE","team_id":"spoof"}`, false}, {`{} {}`, false}, {`not json`, false}} {
		var msg clientMessage
		if got := strictJSON([]byte(tc.raw), &msg) == nil; got != tc.ok {
			t.Fatalf("decode %s: %v", tc.raw, got)
		}
	}
	var bid PlaceBidRequest
	if err := strictJSON([]byte(`{"room_player_id":"x","request_id":"y","bid_amount":"2.00","team_id":"spoof"}`), &bid); err == nil {
		t.Fatal("spoofed team ID accepted")
	}
}
