#!/bin/sh
# Build an add-on the way Home Assistant does (amd64) and check it starts.
# Usage: smoke_test.sh <addon folder>
set -eu

ADDON=$1
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
IMAGE="smoke-$ADDON"
NAME="smoke-$ADDON"
DATA=$(mktemp -d)
# Like Home Assistant's /data: an add-on's own user (e.g. Foundry's "node")
# must be able to read it, whoever runs the test
chmod 755 "$DATA"

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
# The Supervisor's pattern for build_from: anything else is silently replaced
# by its own base image (e.g. "node:24" needs to be "library/node:24")
image = re.compile(r"^([a-zA-Z\-\.:\d{}]+/)*?([\-\w{}]+)/([\-\w{}]+)(:[\.\-\w{}]+)?$")
for arch, base in re.findall(r"^  (\w+): (\S+)$", text.split("build_from:", 1)[1].split("\n\n")[0].split("args:")[0], re.M):
    if not image.match(base):
        sys.exit(f"FAIL: build_from {arch} '{base}' is not accepted by the Supervisor")
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
{"eula": true, "server_name": "Smoke test", "level_name": "Smoke", "level_seed": "", "gamemode": "survival", "difficulty": "normal", "allow_cheats": false, "allow_list": true, "default_player_permission_level": "member", "max_players": 10, "view_distance": 10, "tick_distance": 4, "player_idle_timeout": 30, "lan_visibility": false, "server_port": 19132, "server_port_v6": 19133, "transport": "nethernet", "tracked_players": [], "dashboard": false}
JSON
        # -i: the add-on runs with stdin open (console commands)
        docker run -di --name "$NAME" -v "$DATA:/data:z" "$IMAGE" > /dev/null
        for _ in $(seq 1 60); do
            docker logs "$NAME" 2>&1 | grep -q "Server started" && break
            sleep 3
        done
        docker logs "$NAME" 2>&1 | grep -q "Server started" || fail "Bedrock did not start"
        docker exec "$NAME" grep -qx "transport=nethernet" /data/server.properties || fail "settings not applied"
        docker exec "$NAME" curl -fsS http://127.0.0.1:8791/api/state | grep -q '"players"' || fail "panel did not respond"
        ;;
    foundry_vtt)
        # Without Foundry installed it must stop and say how to install it
        echo '{"download_url": "", "hostname": "", "proxy_ssl": false, "world": ""}' > "$DATA/options.json"
        mkdir -p "$DATA/config"
        docker run --name "$NAME" -v "$DATA:/data:z" -v "$DATA/config:/config:z" "$IMAGE" > /dev/null 2>&1 || true
        docker logs "$NAME" 2>&1 | grep -q "Paste it into 'Download link'" || fail "add-on did not ask for a download link"
        docker rm -f "$NAME" > /dev/null
        # A stand-in for Foundry's Linux download (main.mjs under resources/app)
        # that records how it was started and answers on port 30000
        mkdir -p "$DATA/fake/resources/app"
        echo '{"release": {"generation": 14, "build": 999}}' > "$DATA/fake/resources/app/package.json"
        cat > "$DATA/fake/resources/app/main.mjs" <<'JS'
import fs from "node:fs";
import http from "node:http";
fs.writeFileSync("/config/smoke.json", JSON.stringify({uid: process.getuid(), args: process.argv.slice(2)}));
http.createServer((request, response) => response.end("fake foundry")).listen(30000);
JS
        (cd "$DATA/fake" && python3 -m zipfile -c "$DATA/fake.zip" resources)
        echo '{"download_url": "file:///data/fake.zip", "hostname": "vtt.example.com", "proxy_ssl": true, "world": "smoke-world"}' > "$DATA/options.json"
        docker run -d --name "$NAME" -p 13000:30000 -v "$DATA:/data:z" -v "$DATA/config:/config:z" "$IMAGE" > /dev/null
        wait_for http://localhost:13000/ || fail "Foundry did not start"
        docker logs "$NAME" 2>&1 | grep -q "Starting Foundry VTT 14.999" || fail "version not logged"
        docker exec "$NAME" jq -e '.uid != 0 and (.args | index("--dataPath=/config")) and (.args | index("--world=smoke-world")) and (.args | index("--noupnp"))' /config/smoke.json > /dev/null \
            || fail "Foundry not started as expected: $(docker exec "$NAME" cat /config/smoke.json)"
        docker exec "$NAME" jq -e '.hostname == "vtt.example.com" and .proxySSL == true and .proxyPort == 443 and .upnp == false and .dataPath == "/config"' /config/Config/options.json > /dev/null \
            || fail "options.json not written"
        # An expired link must not break the existing install
        docker rm -f "$NAME" > /dev/null
        echo '{"download_url": "file:///data/expired.zip", "hostname": "", "proxy_ssl": false, "world": ""}' > "$DATA/options.json"
        docker run -d --name "$NAME" -p 13000:30000 -v "$DATA:/data:z" -v "$DATA/config:/config:z" "$IMAGE" > /dev/null
        wait_for http://localhost:13000/ || fail "existing Foundry did not start after a failed download"
        docker logs "$NAME" 2>&1 | grep -q "Starting the Foundry VTT that was already installed" || fail "failed download not reported"
        ;;
    *)
        fail "no smoke test for $ADDON"
        ;;
esac

if docker logs "$NAME" 2>&1 | grep -q '^\[ERROR\]'; then
    fail "add-on logged errors"
fi
echo "OK: $ADDON"
