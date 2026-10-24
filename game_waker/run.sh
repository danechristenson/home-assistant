#!/bin/bash
# For each game add-on in the options: while it's stopped, watch its UDP ports
# and start it on the first packet from outside Home Assistant (a player's
# game trying to join). It only watches the traffic, like tcpdump, and never
# takes the ports, so the game can always start, from here or anywhere else.
set -uo pipefail

OPTIONS=/data/options.json
# The smoke test points these at a stand-in
SUPERVISOR=${SUPERVISOR_URL:-http://supervisor}
TOKEN=${SUPERVISOR_TOKEN:-}
# Not players: Home Assistant itself (e.g. a server-status check on 127.0.0.1)
# and the add-ons' own network
NOT_PLAYERS="not src net 127.0.0.0/8 and not src net 172.30.32.0/23"
# How long to wait for a packet before checking the game's state again (so a
# game started some other way is noticed)
LISTEN_SECONDS=${LISTEN_SECONDS:-60}

supervisor() {
    curl -fsS -m "${CURL_SECONDS:-30}" -H "Authorization: Bearer $TOKEN" "$@"
}

addon_state() {
    supervisor "$SUPERVISOR/addons/$1/info" 2>/dev/null | jq -r '.data.state // empty'
}

# "2456-2457" -> "dst portrange 2456-2457"; "19132" -> "dst port 19132"
port_filter() {
    if [[ $1 == *-* ]]; then echo "dst portrange $1"; else echo "dst port $1"; fi
}

watch_game() {
    local addon=$1 ports=$2 filter state packet from
    filter="udp and $(port_filter "$ports") and $NOT_PLAYERS"
    if ! tcpdump -d -i any "$filter" > /dev/null 2>&1; then
        echo "[ERROR] $addon: can't watch UDP $ports (tcpdump: $(tcpdump -d -i any "$filter" 2>&1 | tail -1))" >&2
        return 1
    fi
    echo "[INFO] $addon: starts when someone tries to join on UDP $ports"
    while true; do
        state=$(addon_state "$addon")
        case "$state" in
            stopped | error) ;;
            "")
                echo "[WARN] $addon: no add-on with that slug (it's in the add-on page's address); checking again in 5 minutes"
                sleep 300
                continue
                ;;
            *)
                sleep 15
                continue
                ;;
        esac
        # (tcpdump exits 0 when timeout stops it too, so check a packet came)
        packet=$(timeout "$LISTEN_SECONDS" tcpdump -c 1 -l -n -p -q -i any "$filter" 2> /dev/null)
        [[ -n "$packet" ]] || continue
        [[ "$(addon_state "$addon")" =~ ^(stopped|error)$ ]] || continue
        from=$(sed -nE 's/.* IP6? ([0-9a-fA-F.:]+)\.[0-9]+ > .*/\1/p' <<< "$packet")
        echo "[INFO] $addon: ${from:-someone} is trying to join; starting it"
        supervisor -X POST -H "Content-Type: application/json" \
            -d "$(jq -cn --arg addon "$addon" --arg from "$from" '{addon: $addon, from: $from}')" \
            "$SUPERVISOR/core/api/events/game_waker_wake" > /dev/null 2>&1 \
            || echo "[WARN] $addon: couldn't tell Home Assistant (event game_waker_wake)"
        # Starting can take minutes (games often update first)
        if CURL_SECONDS=900 supervisor -X POST "$SUPERVISOR/addons/$addon/start" > /dev/null; then
            echo "[INFO] $addon: started"
        else
            echo "[WARN] $addon: couldn't start it; trying again on the next join"
        fi
    done
}

count=$(jq '.games | length' "$OPTIONS")
if [[ "$count" -eq 0 ]]; then
    echo "[WARN] No games to watch: add one under 'Games' on the Configuration tab"
    exec sleep infinity
fi
for i in $(seq 0 $((count - 1))); do
    watch_game "$(jq -r ".games[$i].addon" "$OPTIONS")" "$(jq -r ".games[$i].udp_ports" "$OPTIONS")" &
done
wait
