#!/usr/bin/env bash
# Run against a local disposable test environment. Creates a NEW room and auction.
# Existing rooms/auctions are not modified. New auction is left PAUSED for inspection.
set -euo pipefail
: "${BASE:?set BASE}" "${TEST_DIR:?set TEST_DIR}"
BASE=${BASE%/}
for tool in curl jq python3; do command -v "$tool" >/dev/null; done
for who in host agent1 agent2 agent3; do
    [[ -r "$TEST_DIR/$who.cookie" ]] || { echo "Missing $who.cookie" >&2; exit 1; }
done
work=$(mktemp -d)
room=''; auction=''; started=0; paused=0
cleanup() {
    local rc=$?
    if [[ $started == 1 && $paused == 0 && -n $auction ]]; then
        local code
        code=$(curl -sS --max-time 5 -X POST -b "$TEST_DIR/host.cookie" -o /dev/null -w '%{http_code}' "$BASE/auctions/$auction/pause") || code=000
        echo "Cleanup pause attempt for NEW auction: HTTP $code" >&2
    fi
    [[ -z $room ]] || echo "5B test room: $room"
    [[ -z $auction ]] || echo "5B test auction: $auction"
    rm -rf "$work"
    exit "$rc"
}
trap cleanup EXIT
request() {
    local who=$1 method=$2 path=$3 expected=$4 code
    shift 4
    code=$(curl -sS --max-time 35 -X "$method" -b "$TEST_DIR/$who.cookie" -o "$work/body" -w '%{http_code}' "$BASE$path" "$@")
    if [[ $code != "$expected" ]]; then
        echo "FAIL: $who $method $path expected $expected got $code" >&2
        head -c 1500 "$work/body" >&2; echo >&2; exit 1
    fi
    echo "PASS: $who $method $path HTTP $code"
}
json_request() {
    request "$1" "$2" "$3" "$4" -H 'Content-Type: application/json' --data-binary "$5"
}
json_request host POST /rooms/ 201 "$(jq -nc --arg name "5B queue smoke $(date +%s)" '{name:$name,max_participants:4}')"
room=$(jq -er '.room.id' "$work/body")
code=$(jq -er '.room.room_code' "$work/body")
echo "Created isolated WAITING room: $room"
who=(host agent1 agent2 agent3)
franchise=(CSK RR RCB PBKS)
for i in "${!who[@]}"; do
    json_request "${who[$i]}" POST /rooms/join 200 "$(jq -nc --arg c "$code" '{room_code:$c}')"
    json_request "${who[$i]}" POST /teams/ 201 "$(jq -nc --arg r "$room" --arg f "${franchise[$i]}" '{room_id:$r,franchise_code:$f}')"
done
queue="/rooms/$room/players/queue"
players="/rooms/$room/players"
json_request agent1 POST "$queue" 403 '{}'
json_request host POST "$queue" 400 '{"category_order":["MARQUEE","MARQUEE"]}'
json_request host POST "$queue" 400 '{"shuffle_categories":["INVALID_CATEGORY"]}'
json_request host POST "$queue" 400 '{"shuffle_categories":["UNCAPPED_BATTER","UNCAPPED_BATTER"]}'
json_request host POST "$queue" 200 '{}'
cp "$work/body" "$work/default.json"
order='["UNCAPPED_BATTER","MARQUEE","CAPPED_BATTER","CAPPED_BOWLER","CAPPED_ALL_ROUNDER","CAPPED_WICKETKEEPER","UNCAPPED_BOWLER","UNCAPPED_ALL_ROUNDER","UNCAPPED_WICKETKEEPER","ASSOCIATE_ALL_ROUNDER"]'
opts=$(jq -nc --argjson order "$order" '{category_order:$order}')
json_request host POST "$queue" 200 "$opts"
cp "$work/body" "$work/reordered.json"
shuffle=$(jq -nc --argjson order "$order" '{category_order:$order,shuffle_categories:["UNCAPPED_BATTER"]}')
json_request host POST "$queue" 200 "$shuffle"
cp "$work/body" "$work/shuffled.json"
request agent1 GET "$players" 200
cp "$work/body" "$work/member.json"
python3 - "$work" "$order" <<'PY'
import json, sys
from pathlib import Path
from itertools import groupby
root=Path(sys.argv[1]); order=json.loads(sys.argv[2])
def load(n): return json.loads((root/n).read_text())
def check(n, categories):
    data=load(n); rows=data['players']
    assert data['count']==len(rows)==543, (n,'count')
    assert len({p['id'] for p in rows})==len({p['player_id'] for p in rows})==543,(n,'duplicate')
    assert [p['auction_sequence'] for p in rows]==list(range(1,544)),(n,'sequence')
    assert all(p['status']=='AVAILABLE' for p in rows),(n,'status')
    groups={}
    for cat, group in groupby(rows,lambda p:p['auction_category']):
        assert cat not in groups,(n,'split category',cat)
        groups[cat]=[p['id'] for p in group]
    assert list(groups)==[c for c in categories if c in groups],(n,'order')
    print(f'PASS: {n}: 543 unique players, sequences 1-543, contiguous categories')
    return rows,groups
def identity(rows): return {p['id']:(p['player_id'],p['base_price'],p['auction_category']) for p in rows}
default_order=['MARQUEE','CAPPED_BATTER','CAPPED_BOWLER','CAPPED_ALL_ROUNDER','CAPPED_WICKETKEEPER','UNCAPPED_BATTER','UNCAPPED_BOWLER','UNCAPPED_ALL_ROUNDER','UNCAPPED_WICKETKEEPER','ASSOCIATE_ALL_ROUNDER']
a,ag=check('default.json',default_order)
b,bg=check('reordered.json',order)
c,cg=check('shuffled.json',order)
assert identity(a)==identity(b)==identity(c),'player identity or base price changed'
assert ag==bg,'reorder changed within-category order'
assert list(bg)[0]=='UNCAPPED_BATTER','custom first category missing'
assert len(bg['UNCAPPED_BATTER'])>1,'shuffle category has too few players'
for cat in bg:
    assert set(bg[cat])==set(cg[cat]),('category membership changed',cat)
    if cat!='UNCAPPED_BATTER': assert bg[cat]==cg[cat],('unselected category order changed',cat)
assert bg['UNCAPPED_BATTER']!=cg['UNCAPPED_BATTER'],'shuffle matched original order by chance; rerun test'
assert [p['id'] for p in load('member.json')]==[p['id'] for p in c],'member queue differs'
print('PASS: custom category order; only selected category shuffled')
print('PASS: identities, base prices and member-visible queue preserved')
PY
request host POST "/rooms/$room/auction" 201
auction=$(jq -er '.id' "$work/body")
request agent1 POST "/rooms/$room/auction/start" 403
# Set cleanup flag before start in case the HTTP response is lost after commit.
started=1
request host POST "/rooms/$room/auction/start" 200
jq -e '.status == "ACTIVE"' "$work/body" >/dev/null
first=$(jq -er '.players[0].id' "$work/shuffled.json")
found=0
for ((i=0;i<60;i++)); do
    curl -fsS --max-time 5 -b "$TEST_DIR/host.cookie" "$BASE/auctions/$auction" > "$work/state.json"
    current=$(jq -r '.current_player_id // empty' "$work/state.json")
    if [[ -n $current ]]; then
        [[ $current == "$first" ]] || { echo "FAIL: first nomination did not follow custom queue" >&2; exit 1; }
        found=1; break
    fi
    sleep .2
done
[[ $found == 1 ]] || { echo 'FAIL: first player not nominated' >&2; exit 1; }
echo 'PASS: engine nominated the first player in the custom queue'
json_request host POST "$queue" 409 "$opts"
request host GET "$players" 200
cp "$work/body" "$work/after.json"
jq -e --slurpfile before "$work/shuffled.json" 'map({id,auction_sequence}) == ($before[0].players | map({id,auction_sequence}))' "$work/after.json" >/dev/null
echo 'PASS: queue order unchanged after start and rejected modification'
request host POST "/auctions/$auction/pause" 200
jq -e '.status == "PAUSED"' "$work/body" >/dev/null
paused=1
json_request host POST "$queue" 409 "$opts"
echo 'PASS: queue remains locked while paused'
echo 'QUEUE CONTROL SMOKE TEST PASSED (new test auction left PAUSED)'
