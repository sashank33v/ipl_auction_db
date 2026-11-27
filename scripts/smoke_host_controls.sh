#!/usr/bin/env bash
# Run ONLY against a disposable/test auction with no other active bidders.
# Requires BASE, AUCTION_ID, TEST_DIR; cookie jars from the lobby smoke test.
set -euo pipefail
: "${BASE:?set BASE}" "${AUCTION_ID:?set AUCTION_ID}" "${TEST_DIR:?set TEST_DIR}"
mode=${1:-no-bid}
[[ "$mode" == no-bid || "$mode" == with-bid ]] || { echo 'Usage: smoke_host_controls.sh [no-bid|with-bid]' >&2; exit 2; }
for tool in curl jq python3; do command -v "$tool" >/dev/null; done
for jar in host agent1; do [[ -r "$TEST_DIR/$jar.cookie" ]]; done
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
url="$BASE/auctions/$AUCTION_ID"
state() { curl -fsS --max-time 5 -b "$TEST_DIR/host.cookie" "$url"; }
request() {
    local who=$1 endpoint=$2 expected=$3 code
    shift 3
    code=$(curl -sS --max-time 5 -X POST -b "$TEST_DIR/$who.cookie" -o "$work/body" -w '%{http_code}' "$url/$endpoint" "$@")
    if [[ "$code" != "$expected" ]]; then
        printf 'FAIL: %s %s expected %s got %s\n' "$who" "$endpoint" "$expected" "$code" >&2
        cat "$work/body" >&2
        echo 'Check current auction state; if paused, resume manually when safe.' >&2
        exit 1
    fi
    printf 'PASS: %s %s HTTP %s\n' "$who" "$endpoint" "$code"
}
initial=$(state)
[[ $(jq -r '.status' <<<"$initial") == ACTIVE ]] || { echo 'Auction must be ACTIVE; recover first if necessary.' >&2; exit 1; }
request agent1 pause 403
old=$(jq -r '.current_player_id // empty' <<<"$initial")
player=''
for ((i=0; i<140; i++)); do
    data=$(state)
    [[ $(jq -r '.status' <<<"$data") == ACTIVE ]] || { echo 'Auction is no longer ACTIVE' >&2; exit 1; }
    candidate=$(jq -r '.current_player_id // empty' <<<"$data")
    if [[ -n "$candidate" && "$candidate" != "$old" ]] && jq -e '.current_bid == null' <<<"$data" >/dev/null; then
        player=$candidate
        break
    fi
    sleep .25
done
[[ -n "$player" ]] || { echo 'No new player; main queue may be exhausted.' >&2; exit 1; }
price=$(curl -fsS --max-time 5 -b "$TEST_DIR/host.cookie" "$BASE/room-players/$player" | jq -er '.base_price | tostring')
make_bid() { jq -nc --arg p "$player" --arg a "$1" --arg r "$(cat /proc/sys/kernel/random/uuid)" '{room_player_id:$p,bid_amount:$a,request_id:$r}'; }
if [[ "$mode" == with-bid ]]; then
    request host bids 200 -H 'Content-Type: application/json' -d "$(make_bid "$price")"
    jq -e --arg p "$player" '.room_player_id == $p' "$work/body" >/dev/null
    blocked_price=$(python3 -c 'from decimal import Decimal; import sys; print(Decimal(sys.argv[1])+Decimal("0.10"))' "$price")
else
    blocked_price=$price
fi
request host pause 200
cp "$work/body" "$work/paused.json"
jq -e --arg p "$player" '.status == "PAUSED" and .current_player_id == $p and .paused_at != null and .timer_deadline_at != null' "$work/paused.json" >/dev/null
if [[ "$mode" == with-bid ]]; then jq -e '.current_bid != null and .current_bidder_team_id != null' "$work/paused.json" >/dev/null; else jq -e '.current_bid == null' "$work/paused.json" >/dev/null; fi
request host pause 409
request agent1 resume 403
request agent1 bids 409 -H 'Content-Type: application/json' -d "$(make_bid "$blocked_price")"
echo 'Waiting 17 seconds (longer than either bidding window)...'
sleep 17
state > "$work/frozen.json"
jq -e --slurpfile p "$work/paused.json" '.status == "PAUSED" and .current_player_id == $p[0].current_player_id and .current_bid == $p[0].current_bid and .current_bidder_team_id == $p[0].current_bidder_team_id and .timer_deadline_at == $p[0].timer_deadline_at and .paused_at == $p[0].paused_at' "$work/frozen.json" >/dev/null
echo 'PASS: auction did not advance during pause'
request host resume 200
cp "$work/body" "$work/resumed.json"
python3 - "$work/paused.json" "$work/resumed.json" <<'PY'
import json,sys
from datetime import datetime
with open(sys.argv[1]) as f: p=json.load(f)
with open(sys.argv[2]) as f: r=json.load(f)
def dt(x): return datetime.fromisoformat(x.replace('Z','+00:00'))
assert r['status']=='ACTIVE' and r['paused_at'] is None
for key in ('current_player_id','current_bid','current_bidder_team_id'):
    assert r[key]==p[key],key
before=dt(p['timer_deadline_at'])-dt(p['paused_at'])
after=dt(r['timer_deadline_at'])-dt(r['updated_at'])
assert before==after and before.total_seconds()>0,(before,after)
print(f'PASS: exact remaining time restored: {before.total_seconds():.6f}s')
PY
request host resume 409
echo "HOST CONTROL SMOKE TEST PASSED ($mode)"
