#!/bin/sh
# Build an add-on the way Home Assistant does (amd64) and check it starts.
# Usage: smoke_test.sh <addon folder>
set -eu

ADDON=$1
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
IMAGE="smoke-$ADDON"
NAME="smoke-$ADDON"
DATA=$(mktemp -d)

cleanup() {
    docker logs "$NAME" > "$DATA/container.log" 2>&1 || true
    docker rm -f "$NAME" > /dev/null 2>&1 || true
}
trap cleanup EXIT

fail() {
    echo "FAIL: $*" >&2
    docker logs "$NAME" 2>&1 | tail -40 >&2 || true
    exit 1
}

# Build arguments: BUILD_FROM for amd64 plus everything under args:
BUILD_ARGS=$(python3 - "$ROOT/$ADDON/build.yaml" <<'EOF'
import re, sys
text = open(sys.argv[1]).read()
args = [f"BUILD_FROM={re.search(r'^  amd64: (\S+)$', text, re.M).group(1)}"]
section = text.split("args:", 1)[1] if "args:" in text else ""
args += [f"{k}={v.strip(chr(34))}" for k, v in re.findall(r"^  (\w+): (\S+)$", section, re.M)]
print(" ".join(f"--build-arg {a}" for a in args))
EOF
)
# shellcheck disable=SC2086
docker build $BUILD_ARGS -t "$IMAGE" "$ROOT/$ADDON"

wait_for() {
    url=$1
    for _ in $(seq 1 30); do
        curl -fsS -o /dev/null "$url" && return 0
        sleep 2
    done
    return 1
}

case "$ADDON" in
    matrix)
        cat > "$DATA/options.json" <<'EOF'
{"server_name": "smoke.test", "homeserver_url": "http://localhost:18008", "registration_token": "", "allow_federation": false, "ingress_autologin": true, "call_relay": false, "lockdown_room_directory": true, "allow_room_creation": true, "auto_join_rooms": [], "federation_allowed_servers": [], "landing_contact": "Smoke test", "ssl": false, "certfile": "fullchain.pem", "keyfile": "privkey.pem"}
EOF
        docker run -d --name "$NAME" -p 18008:8008 -p 18010:8010 -v "$DATA:/data:z" "$IMAGE" > /dev/null
        wait_for http://localhost:18008/_matrix/client/versions || fail "Matrix API did not respond"
        wait_for http://localhost:18010/ || fail "landing page did not respond"
        curl -fsS http://localhost:18010/ | grep -q "Smoke test" || fail "landing page missing contact line"
        docker exec "$NAME" test -s /var/www/element/index.html || fail "Element Web missing"
        docker exec "$NAME" test -s /var/www/invite/qrcode.js || fail "QR code library missing"
        docker exec "$NAME" sh -c 'grep -q "createSvgTag" /var/www/invite/qrcode.js' || fail "QR code library API changed"
        ;;
    matrix_call_relay)
        echo '{"address": "203.0.113.10"}' > "$DATA/options.json"
        mkdir -p "$DATA/share"
        docker run -d --name "$NAME" -v "$DATA:/data:z" -v "$DATA/share:/share:z" "$IMAGE" > /dev/null
        sleep 5
        [ "$(docker inspect -f '{{.State.Running}}' "$NAME")" = "true" ] || fail "relay exited"
        docker logs "$NAME" 2>&1 | grep -q "Call relay running" || fail "relay did not start"
        docker exec "$NAME" sh -c 'ps -o user,args | grep "[t]urnserver" | grep -q "^nobody"' || fail "relay not running as nobody"
        ;;
    matrix_group_calls)
        echo '{"domain": "livekit.example.com", "matrix_server_name": "matrix.example.com"}' > "$DATA/options.json"
        mkdir -p "$DATA/share"
        docker run -d --name "$NAME" -p 17880:7880 -p 17883:7883 -v "$DATA:/data:z" -v "$DATA/share:/share:z" "$IMAGE" > /dev/null
        wait_for http://localhost:17880/ || fail "LiveKit did not respond"
        wait_for http://localhost:17883/healthz || fail "lk-jwt-service did not respond"
        grep -q '^livekit_service_url=https://livekit.example.com$' "$DATA/share/matrix-group-calls/calls.conf" \
            || fail "call service not shared with the Matrix add-on"
        docker exec "$NAME" grep -q 'max_participants: 8' /tmp/livekit.yaml || fail "call limit missing"
        ;;
    matrix_helper_bot)
        # Without a password it must refuse to start, with a clear message
        echo '{"username": "helper", "password": "", "helpers": [], "password_message_minutes": 10, "matrix_url": ""}' > "$DATA/options.json"
        docker run --name "$NAME" -v "$DATA:/data:z" "$IMAGE" > /dev/null 2>&1 || true
        docker logs "$NAME" 2>&1 | grep -q "Set the bot account's 'password'" || fail "bot did not ask for a password"
        docker rm -f "$NAME" > /dev/null
        # With one but no Matrix server, it must keep running and retry
        echo '{"username": "helper", "password": "x", "helpers": [], "password_message_minutes": 10, "matrix_url": "http://127.0.0.1:9"}' > "$DATA/options.json"
        docker run -d --name "$NAME" -v "$DATA:/data:z" "$IMAGE" > /dev/null
        sleep 5
        [ "$(docker inspect -f '{{.State.Running}}' "$NAME")" = "true" ] || fail "bot exited"
        docker logs "$NAME" 2>&1 | grep -q "retrying in 15 seconds" || fail "bot did not retry"
        ;;
    minecraft_bedrock)
        cat > "$DATA/options.json" <<'JSON'
{"eula": true, "server_name": "Smoke test", "level_name": "Smoke", "level_seed": "", "gamemode": "survival", "difficulty": "normal", "allow_cheats": false, "allow_list": true, "default_player_permission_level": "member", "max_players": 10, "view_distance": 10, "tick_distance": 4, "player_idle_timeout": 30, "lan_visibility": false, "server_port": 19132, "server_port_v6": 19133, "transport": "nethernet", "helper_bot_key": "smoke-test-key-0123456789"}
JSON
        # -i: the add-on runs with stdin open (console commands)
        docker run -di --name "$NAME" -v "$DATA:/data:z" "$IMAGE" > /dev/null
        for _ in $(seq 1 60); do
            docker logs "$NAME" 2>&1 | grep -q "Server started" && break
            sleep 3
        done
        docker logs "$NAME" 2>&1 | grep -q "Server started" || fail "Bedrock did not start"
        docker exec "$NAME" grep -qx "transport=nethernet" /data/server.properties || fail "settings not applied"
        docker exec "$NAME" curl -fsS -H "Authorization: Bearer smoke-test-key-0123456789" \
            http://127.0.0.1:8790/api/bot/players | grep -q '"players"' || fail "bot access did not respond"
        ;;
    *)
        fail "no smoke test for $ADDON"
        ;;
esac

if docker logs "$NAME" 2>&1 | grep -q '^\[ERROR\]'; then
    fail "add-on logged errors"
fi
echo "OK: $ADDON"
