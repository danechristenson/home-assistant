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
    # (an add-on that takes over /data, like farmOS's www-data, leaves it unwritable here)
    { docker logs "$NAME" > "$DATA/container.log" 2>&1; } 2>/dev/null || true
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
        # Open web UI (ingress) goes to the address players use
        open_web_ui() { docker exec "$NAME" curl -fsS http://127.0.0.1:8099/; }
        open_web_ui | grep -q 'const url = "https://vtt.example.com/" ||' \
            || fail "Open web UI does not go to the address players use: $(open_web_ui)"
        # An expired link must not break the existing install
        docker rm -f "$NAME" > /dev/null
        echo '{"download_url": "file:///data/expired.zip", "hostname": "", "proxy_ssl": false, "world": ""}' > "$DATA/options.json"
        docker run -d --name "$NAME" -p 13000:30000 -v "$DATA:/data:z" -v "$DATA/config:/config:z" "$IMAGE" > /dev/null
        wait_for http://localhost:13000/ || fail "existing Foundry did not start after a failed download"
        docker logs "$NAME" 2>&1 | grep -q "Starting the Foundry VTT that was already installed" || fail "failed download not reported"
        # No address set: Open web UI uses Home Assistant's own, port 30000
        open_web_ui | grep -q 'const url = "" ||' || fail "Open web UI without an address: $(open_web_ui)"
        ;;
    farmos)
        mkdir -p "$DATA/config"
        docker run -d --name "$NAME" -p 18096:80 -v "$DATA:/data:z" -v "$DATA/config:/config:z" "$IMAGE" > /dev/null
        wait_for http://127.0.0.1:18096/core/install.php || fail "installer did not respond"
        curl -fsS http://127.0.0.1:18096/core/install.php | grep -q "SQLite" || fail "installer offers no SQLite"
        # Installed (as the installer would with SQLite), then restarted: the private files path, the
        # updates and the API key. (Drush 13 / Drupal 11, farmOS 4: an SQLite URL needs a host, so
        # sqlite://localhost//absolute/path)
        docker exec "$NAME" runuser -u www-data -- /opt/drupal/vendor/bin/drush --root=/opt/drupal/web \
            site:install farm --db-url=sqlite://localhost//data/db/farmos.sqlite --site-name=Smoke \
            --account-name=admin --account-pass="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')" -y \
            > /dev/null 2>&1 || fail "farmOS did not install"
        # (farmOS 4's minimal install has no asset types; the web installer offers plants and the rest)
        docker exec "$NAME" runuser -u www-data -- /opt/drupal/vendor/bin/drush --root=/opt/drupal/web \
            pm:enable -y farm_plant > /dev/null 2>&1 || fail "could not enable plants"
        docker restart "$NAME" > /dev/null
        wait_for http://127.0.0.1:18096/user/login || fail "farmOS did not come back after installing"
        docker logs "$NAME" 2>&1 | grep -q "private files path" || fail "private files path not set"
        docker exec "$NAME" grep -q "file_private_path" /data/sites/default/settings.php || fail "private files path missing"
        # (only farmOS's own user can read the key: through the container)
        KEY=$(docker exec "$NAME" cat /config/api_client.json) || fail "no API key in /config"
        SECRET=$(echo "$KEY" | python3 -c "import json, sys; print(json.load(sys.stdin)['client_secret'])")
        TOKEN=$(curl -fsS http://127.0.0.1:18096/oauth/token -d grant_type=client_credentials -d client_id=api_sync \
            -d "client_secret=$SECRET" -d scope=farm_manager \
            | python3 -c "import json, sys; print(json.load(sys.stdin)['access_token'])") || fail "the API key got no token"
        curl -fsS -H "Authorization: Bearer $TOKEN" -H "Accept: application/vnd.api+json" \
            http://127.0.0.1:18096/api/asset/plant > /dev/null || fail "the API key can't read plants"
        # A second start keeps the same key
        docker restart "$NAME" > /dev/null
        wait_for http://127.0.0.1:18096/user/login || fail "farmOS did not come back"
        docker logs "$NAME" 2>&1 | grep -q "API key: ready" || fail "a restart replaced the API key"
        ;;
    game_waker)
        # A stand-in Supervisor: the "smoke" game is stopped until something
        # starts it (POST .../stop stops it again); every POST is written down
        cat > "$DATA/supervisor.py" <<'PY'
import http.server, json, sys
state, log = "stopped", sys.argv[1]
class Handler(http.server.BaseHTTPRequestHandler):
    def reply(self, body):
        data = json.dumps(body).encode()
        self.send_response(200); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data))); self.end_headers(); self.wfile.write(data)
    def do_GET(self):
        data = {"/addons/smoke/info": {"state": state}, "/addons/self/info": {"ingress_port": 18096}}.get(self.path, {})
        self.reply({"result": "ok", "data": data})
    def do_POST(self):
        global state
        body = self.rfile.read(int(self.headers.get("Content-Length") or 0)).decode()
        open(log, "a").write(f"POST {self.path} {body}\n")
        state = {"/addons/smoke/start": "started", "/addons/smoke/stop": "stopped"}.get(self.path, state)
        self.reply({"result": "ok", "data": {}})
    def log_message(self, *args):
        pass
http.server.HTTPServer(("127.0.0.1", 18099), Handler).serve_forever()
PY
        python3 "$DATA/supervisor.py" "$DATA/requests.log" &
        SUPERVISOR_PID=$!
        trap 'kill $SUPERVISOR_PID 2>/dev/null || true; cleanup' EXIT
        # No home networks: the machine's own address is an outsider until it presses Start
        cat > "$DATA/options.json" <<'JSON'
{"games": [{"name": "Smoke", "addon": "smoke", "udp_ports": "23456-23457", "join_address": "smoke.example.com:23456"}],
 "home_networks": [], "wake_from": [], "web_port": 18095}
JSON
        docker run -d --name "$NAME" --network host --cap-add NET_RAW -v "$DATA:/data:z" \
            -e SUPERVISOR_URL=http://127.0.0.1:18099 -e SUPERVISOR_TOKEN=smoke -e LISTEN_SECONDS=3 "$IMAGE" > /dev/null
        wait_for http://127.0.0.1:18095/ || fail "start page did not respond"
        docker logs "$NAME" 2>&1 | grep -q "Smoke: starts when someone at home" || fail "waker did not start watching"
        starts() { cat "$DATA/requests.log" 2>/dev/null | grep -c "POST /addons/smoke/start" || true; }
        send() { python3 -c "import socket, sys; s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM); s.bind((sys.argv[1], 0)); s.sendto(b'join', (sys.argv[1], int(sys.argv[2])))" "$@"; }
        HOST_IP=$(python3 -c "import socket; s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM); s.connect(('192.0.2.1', 9)); print(s.getsockname()[0])")
        # Home Assistant's own checks (127.0.0.1) and outsiders (scanners) never wake it
        for _ in 1 2 3; do send 127.0.0.1 23457; send "$HOST_IP" 23457; sleep 2; done
        [ "$(starts)" = 0 ] || fail "an outsider or 127.0.0.1 started the game: $(cat "$DATA/requests.log")"
        # The page, then Start from that address (through a reverse proxy: X-Real-IP)
        curl -fsS http://127.0.0.1:18095/ | grep -q "Start Smoke" || fail "start page has no Start button"
        curl -fsS http://127.0.0.1:18095/ | grep -q "smoke.example.com:23456" || fail "start page has no join address"
        curl -fsS http://127.0.0.1:18095/status | grep -q '"state": "stopped"' || fail "status not shown: $(curl -fsS http://127.0.0.1:18095/status)"
        curl -fsS "http://127.0.0.1:18095/?from=a-link" | grep -q "Start Smoke" || fail "start page with a query string"
        [ "$(curl -fsS -o /dev/null -w '%{content_type}' http://127.0.0.1:18095/icon.png)" = image/png ] || fail "no icon"
        # The same page for Home Assistant's Open web UI, on the ingress port the Supervisor gives
        curl -fsS http://127.0.0.1:18096/ | grep -q "Start Smoke" || fail "no start page on the ingress port"
        curl -fsS -X POST -H "X-Real-IP: $HOST_IP" http://127.0.0.1:18095/start/0 | grep -q "remembered" \
            || fail "Start didn't remember the address"
        sleep 2
        [ "$(starts)" = 1 ] || fail "Start didn't start the game: $(cat "$DATA/requests.log")"
        grep -q "\"$HOST_IP\"" "$DATA/remembered.json" || fail "address not saved: $(cat "$DATA/remembered.json")"
        # Stopped again, joining from the remembered address wakes it by itself
        curl -fsS -X POST http://127.0.0.1:18099/addons/smoke/stop > /dev/null
        # (while a game runs, the waker checks its state every 15 seconds)
        for _ in $(seq 1 30); do
            send "$HOST_IP" 23456
            [ "$(starts)" = 2 ] && break
            sleep 1
        done
        [ "$(starts)" = 2 ] || fail "joining from a remembered address did not start the game"
        grep -q "POST /core/api/events/game_waker_wake .*\"from\": \"$HOST_IP\", \"how\": \"start page\"" "$DATA/requests.log" \
            || fail "no game_waker_wake event for the start page: $(cat "$DATA/requests.log")"
        grep -q "POST /core/api/events/game_waker_wake .*\"from\": \"$HOST_IP\", \"how\": \"joined\"" "$DATA/requests.log" \
            || fail "no game_waker_wake event with the player's address: $(cat "$DATA/requests.log")"
        docker logs "$NAME" 2>&1 | grep -q "Smoke: $HOST_IP is trying to join (UDP 23456, 4 bytes, starting 6a6f696e); starting it" \
            || fail "join not logged: $(docker logs "$NAME" 2>&1 | tail -5)"
        ;;
    *)
        fail "no smoke test for $ADDON"
        ;;
esac

if docker logs "$NAME" 2>&1 | grep -q '^\[ERROR\]'; then
    fail "add-on logged errors"
fi
echo "OK: $ADDON"
