#!/bin/sh
set -e

OPTIONS=/data/options.json

# Read a top-level value from the add-on options (the image only has busybox)
option() {
    sed -n "s/.*\"$1\": *\"\{0,1\}\([^\",}]*\)\"\{0,1\}.*/\1/p" "$OPTIONS" | head -n 1
}

set -- config/config.toml

DOMAIN=$(option domain)
if [ -n "$DOMAIN" ]; then
    set -- "$@" --network.domain "$DOMAIN"
fi

FRONTEND_URL=$(option frontend_url)
if [ -n "$FRONTEND_URL" ]; then
    set -- "$@" --network.frontend_url "$FRONTEND_URL"
fi

if [ "$(option ssl)" = "true" ]; then
    CERTFILE="/ssl/$(option certfile)"
    KEYFILE="/ssl/$(option keyfile)"
    for f in "$CERTFILE" "$KEYFILE"; do
        if [ ! -f "$f" ]; then
            echo "[ERROR] SSL is enabled but $f does not exist" >&2
            exit 1
        fi
    done
    set -- "$@" --network.tls.type certificate \
        --network.tls.cert "$CERTFILE" \
        --network.tls.key "$KEYFILE"
fi

echo "[INFO] Starting VoceChat server"
cd /home/vocechat-server
exec ./vocechat-server "$@"
