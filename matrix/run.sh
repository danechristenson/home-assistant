#!/bin/sh
set -e

OPTIONS=/data/options.json
CONFIG=/tmp/continuwuity.toml
WEBROOT=/var/www/element

# Print an option's raw JSON value. JSON string escapes are valid TOML and
# JSON, so string values can be written into both files as-is.
option() {
    sed -nE "s/.*\"$1\": *(\"([^\"\\\\]|\\\\.)*\"|true|false|[0-9]+).*/\1/p" "$OPTIONS" | head -n 1
}

# Print an option's string value without quotes (for simple values only)
option_text() {
    option "$1" | sed -E 's/^"(.*)"$/\1/'
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

HOMESERVER_URL=$(option_text homeserver_url)
if [ -z "$HOMESERVER_URL" ]; then
    if [ "$(option ssl)" = "true" ]; then
        HOMESERVER_URL="https://${SERVER_NAME}:8008"
    else
        HOMESERVER_URL="http://${SERVER_NAME}:8008"
    fi
fi

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

# Element Web, pinned to this homeserver
cat > "$WEBROOT/config.json" <<JSON
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
    "show_labs_settings": false
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

echo "[INFO] Starting Continuwuity for $SERVER_NAME"
exec conduwuit --config "$CONFIG"
