#!/bin/sh
set -e

OPTIONS=/data/options.json
CONFIG=/tmp/continuwuity.toml
WEBROOT=/var/www
AUTOLOGIN_TOKENS=/data/autologin_tokens
AUTOLOGIN_ENV=/tmp/autologin.env

# Print an option's raw JSON value. JSON string escapes are valid TOML and
# JSON, so string values can be written into both files as-is.
option() {
    sed -nE "s/.*\"$1\": *(\"([^\"\\\\]|\\\\.)*\"|true|false|[0-9]+).*/\1/p" "$OPTIONS" | head -n 1
}

# Print an option's string value without quotes (for simple values only)
option_text() {
    option "$1" | sed -E 's/^"(.*)"$/\1/'
}

random_token() {
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

SERVER_NAME=$(option_text server_name)
if [ -z "$SERVER_NAME" ]; then
    echo "[ERROR] Set 'server_name' in the add-on configuration before starting." >&2
    echo "[ERROR] It becomes part of every user ID (@name:server_name) and cannot be changed later." >&2
    exit 1
fi

# The server name is baked into every user ID and room, so refuse to start
# if it no longer matches the one the database was created with
if [ -f /data/server_name ]; then
    ORIGINAL_SERVER_NAME=$(cat /data/server_name)
    if [ "$SERVER_NAME" != "$ORIGINAL_SERVER_NAME" ]; then
        echo "[ERROR] 'server_name' was changed from '$ORIGINAL_SERVER_NAME' to '$SERVER_NAME'." >&2
        echo "[ERROR] The server name cannot be changed once the server has started." >&2
        echo "[ERROR] Set it back to '$ORIGINAL_SERVER_NAME', or uninstall and reinstall the add-on to start over (this deletes all Matrix data)." >&2
        exit 1
    fi
fi

if [ "$(option ssl)" = "true" ]; then
    SCHEME=https
else
    SCHEME=http
fi

HOMESERVER_URL=$(option_text homeserver_url)
if [ -z "$HOMESERVER_URL" ]; then
    HOMESERVER_URL="${SCHEME}://${SERVER_NAME}:8008"
fi
HOMESERVER_URL=${HOMESERVER_URL%/}

cat > "$CONFIG" <<TOML
[global]
server_name = $(option server_name)
database_path = "/data/database"
address = ["0.0.0.0"]
port = 8008
allow_federation = $(option allow_federation)
allow_registration = true
new_user_displayname_suffix = ""
TOML

if [ "$(option registration_token)" != '""' ]; then
    echo "registration_token = $(option registration_token)" >> "$CONFIG"
fi

# Home Assistant ingress autologin: an appservice that may log in as any
# local user. The token never leaves the container; turning the option off
# forgets it, which leaves the registered appservice unusable.
# Options added after the first release are optional in the schema, so
# settings saved by older versions still validate; fall back to the defaults
AUTOLOGIN=$(option ingress_autologin)
AUTOLOGIN=${AUTOLOGIN:-true}
if [ "$AUTOLOGIN" = "true" ] && [ "$SCHEME" = "https" ]; then
    # busybox's TLS client cannot talk to Continuwuity's TLS listener
    echo "[WARN] Sidebar autologin is not available while 'Use HTTPS' is on." >&2
    echo "[WARN] Let a reverse proxy handle HTTPS instead (see the Documentation tab) to use it." >&2
    AUTOLOGIN=false
fi

if [ "$AUTOLOGIN" = "true" ]; then
    if [ ! -f "$AUTOLOGIN_TOKENS" ]; then
        (umask 077 && printf 'AS_TOKEN=%s\nHS_TOKEN=%s\n' "$(random_token)" "$(random_token)" > "$AUTOLOGIN_TOKENS")
    fi
    . "$AUTOLOGIN_TOKENS"
    SERVER_NAME_REGEX=$(printf '%s' "$SERVER_NAME" | sed 's/\./\\./g')
    cat >> "$CONFIG" <<TOML
admin_execute = ['''appservices register
\`\`\`
id: home-assistant-autologin
url: null
as_token: $AS_TOKEN
hs_token: $HS_TOKEN
sender_localpart: _ha_autologin
rate_limited: false
namespaces:
  users:
    - exclusive: false
      regex: '@.*:$SERVER_NAME_REGEX'
\`\`\`''']
TOML
    (umask 077 && cat > "$AUTOLOGIN_ENV" <<ENV
AUTOLOGIN=true
AS_TOKEN=$AS_TOKEN
SERVER_NAME='$SERVER_NAME'
HOMESERVER_URL='$HOMESERVER_URL'
INTERNAL_URL=${SCHEME}://127.0.0.1:8008
ENV
)
else
    rm -f "$AUTOLOGIN_TOKENS"
    echo "AUTOLOGIN=false" > "$AUTOLOGIN_ENV"
fi

# Voice and video calls between different networks, through the separate
# Matrix Call Relay add-on. It shares its address and secret via /share; the
# file is parsed rather than sourced because other add-ons can write there.
RELAY_CONF=/share/matrix-call-relay/relay.conf
if [ "$(option call_relay)" = "true" ]; then
    RELAY_ADDRESS=$(sed -nE 's/^address=([A-Za-z0-9.:-]+)$/\1/p' "$RELAY_CONF" 2>/dev/null || true)
    RELAY_SECRET=$(sed -nE 's/^secret=([0-9a-f]+)$/\1/p' "$RELAY_CONF" 2>/dev/null || true)
    if [ -n "$RELAY_ADDRESS" ] && [ -n "$RELAY_SECRET" ]; then
        cat >> "$CONFIG" <<TOML
turn_uris = ["turn:$RELAY_ADDRESS:3478?transport=udp", "turn:$RELAY_ADDRESS:3478?transport=tcp"]
turn_secret = "$RELAY_SECRET"
TOML
        echo "[INFO] Using the Matrix Call Relay at $RELAY_ADDRESS for calls"
    elif [ ! -e "$RELAY_CONF" ]; then
        echo "[WARN] 'Use call relay' is on, but the Matrix Call Relay add-on isn't running." >&2
        echo "[WARN] Install and start it, then restart this add-on." >&2
    else
        echo "[WARN] 'Use call relay' is on, but $RELAY_CONF is unreadable or invalid." >&2
        echo "[WARN] Restart the Matrix Call Relay add-on, then this one." >&2
    fi
fi

if [ "$(option ssl)" = "true" ]; then
    CERTFILE="/ssl/$(option_text certfile)"
    KEYFILE="/ssl/$(option_text keyfile)"
    for f in "$CERTFILE" "$KEYFILE"; do
        if [ ! -f "$f" ]; then
            echo "[ERROR] SSL is enabled but $f does not exist" >&2
            exit 1
        fi
    done
    cat >> "$CONFIG" <<TOML

[global.tls]
certs = "$CERTFILE"
key = "$KEYFILE"
TOML
fi

# Advertise the reachable address, so invite links point at the right place
cat >> "$CONFIG" <<TOML

[global.well_known]
client = "$HOMESERVER_URL"
TOML

# Element Web, pinned to this homeserver
cat > "$WEBROOT/element/config.json" <<JSON
{
    "default_server_config": {
        "m.homeserver": {
            "base_url": "$HOMESERVER_URL",
            "server_name": $(option server_name)
        }
    },
    "disable_custom_urls": false,
    "disable_guests": true,
    "brand": "Element",
    "show_labs_settings": false,
    "embedded_pages": {
        "home_url": "home.html"
    }
}
JSON

# Only accept connections from the Home Assistant ingress proxy
cat > /tmp/httpd.conf <<CONF
A:172.30.32.2
D:*
.wasm:application/wasm
.mjs:text/javascript
.json:application/json
CONF

echo "[INFO] Element Web available through ingress (homeserver: $HOMESERVER_URL)"
httpd -f -p 0.0.0.0:8099 -h "$WEBROOT" -c /tmp/httpd.conf &

printf '%s' "$SERVER_NAME" > /data/server_name

# Pass the server's output through to the log, remembering the one-time token
# it prints until the first (admin) account exists, so the sidebar can offer it
rm -f /tmp/setup_token /tmp/server.out
mkfifo /tmp/server.out
ESC=$(printf '\033')
while IFS= read -r line; do
    printf '%s\n' "$line"
    case "$line" in
        *"using the registration token "*)
            printf '%s' "$line" | sed -E "s/${ESC}\\[[0-9;]*m//g" \
                | sed -nE 's/.*using the registration token ([A-Za-z0-9]+).*/\1/p' > /tmp/setup_token
            ;;
    esac
done < /tmp/server.out &

echo "[INFO] Starting Continuwuity for $SERVER_NAME"
conduwuit --config "$CONFIG" > /tmp/server.out 2>&1 &
SERVER_PID=$!
trap 'kill -TERM "$SERVER_PID" 2>/dev/null' TERM INT

set +e
wait "$SERVER_PID"
STATUS=$?
# A trapped signal interrupts wait; wait again for the server to finish
if kill -0 "$SERVER_PID" 2>/dev/null; then
    wait "$SERVER_PID"
    STATUS=$?
fi
exit "$STATUS"
