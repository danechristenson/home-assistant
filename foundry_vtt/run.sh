#!/bin/bash
# Installs Foundry VTT from the user's timed download link (once), applies
# the Home Assistant options to Foundry's options.json, then runs Foundry as
# the unprivileged "node" user.
set -euo pipefail

OPTIONS=/data/options.json
INSTALL=/data/install
# Foundry's own data folder: Config, Data (worlds, systems, modules), Logs
FOUNDRY_DATA=/config
# Which link the current install came from (a hash, not the link)
INSTALLED_FROM=/data/installed-from

opt() {
    jq -r --arg key "$1" '.[$key] | if . == null then "" else tostring end' "$OPTIONS"
}

# The entry point of an installed Foundry: main.mjs since V13, main.js before
entry_point() {
    local dir=$1 name
    for name in main.mjs main.js; do
        if [[ -f "$dir/$name" ]]; then
            echo "$dir/$name"
            return 0
        fi
    done
    return 1
}

# Removes the link from the add-on's options, so the Configuration tab shows
# it has been used (it only lasts 5 minutes anyway)
clear_download_url() {
    local options
    options=$(jq -c '{options: (. + {download_url: ""})}' "$OPTIONS")
    if curl -fsS -o /dev/null --max-time 15 -X POST \
        -H "Authorization: Bearer ${SUPERVISOR_TOKEN:-}" \
        -H "Content-Type: application/json" \
        -d "$options" http://supervisor/addons/self/options 2>/dev/null; then
        echo "[INFO] Cleared the download link from the Configuration tab"
    else
        echo "[WARN] Couldn't clear the download link from the Configuration tab; it won't be used again, so you can clear it yourself"
    fi
}

# Downloads and unpacks Foundry into $INSTALL. Returns 1 (and leaves any
# existing install alone) if the link has expired or isn't a Foundry zip.
install_foundry() {
    local url=$1 staging=/data/install.new app
    rm -rf "$staging" /data/download.zip

    echo "[INFO] Downloading Foundry VTT (a few hundred MB; this can take a few minutes)"
    if ! curl -fL --no-progress-meter --retry 2 --proto-redir =https -o /data/download.zip "$url"; then
        echo "[WARN] The download failed. Timed links only last 5 minutes: on foundryvtt.com, open your Purchased Licenses, press the link icon next to Node.js again, paste the new link into 'Download link' and restart the add-on."
        rm -f /data/download.zip
        return 1
    fi
    if ! unzip -tq /data/download.zip > /dev/null 2>&1; then
        echo "[WARN] That download isn't a zip file. Choose the Node.js (or Linux) download on foundryvtt.com, not Windows or macOS."
        rm -f /data/download.zip
        return 1
    fi

    mkdir -p "$staging"
    unzip -q /data/download.zip -d "$staging"
    rm -f /data/download.zip

    # The Node.js download has main.mjs at the top; the Linux download (and
    # V12 and before) has it under resources/app
    if entry_point "$staging" > /dev/null; then
        app=$staging
    elif entry_point "$staging/resources/app" > /dev/null; then
        app=$staging/resources/app
    else
        echo "[WARN] That zip doesn't contain Foundry VTT (no main.mjs or main.js). Choose the Node.js download on foundryvtt.com."
        rm -rf "$staging"
        return 1
    fi

    rm -rf "$INSTALL"
    mv "$app" "$INSTALL"
    rm -rf "$staging"
    # Foundry updates itself from its Setup screen, so it must own its files
    chown -R node:node "$INSTALL"
    echo "[INFO] Installed Foundry VTT"
    return 0
}

URL=$(opt download_url)
if [[ -n "$URL" ]]; then
    url_hash=$(printf '%s' "$URL" | sha256sum | cut -d' ' -f1)
    if [[ -f "$INSTALLED_FROM" && "$(cat "$INSTALLED_FROM")" == "$url_hash" ]] && entry_point "$INSTALL" > /dev/null; then
        echo "[INFO] Foundry VTT was already installed from this download link"
    elif install_foundry "$URL"; then
        echo "$url_hash" > "$INSTALLED_FROM"
    elif entry_point "$INSTALL" > /dev/null; then
        echo "[WARN] Starting the Foundry VTT that was already installed"
    fi
    clear_download_url
fi

if ! ENTRY=$(entry_point "$INSTALL"); then
    echo "[ERROR] Foundry VTT isn't installed yet. On foundryvtt.com, open your Purchased Licenses, choose the Node.js download and press the link icon next to it (a timed link, valid for 5 minutes). Paste it into 'Download link' on the Configuration tab, save, and start the add-on within 5 minutes." >&2
    exit 1
fi

# Settings from the Configuration tab, applied on each start; the rest of
# options.json stays as Foundry's Setup screen left it
mkdir -p "$FOUNDRY_DATA/Config"
CONFIG=$FOUNDRY_DATA/Config/options.json
[[ -s "$CONFIG" ]] || echo '{}' > "$CONFIG"
PROXY_SSL=false
[[ "$(opt proxy_ssl)" == "true" ]] && PROXY_SSL=true
jq --arg hostname "$(opt hostname)" --argjson proxy_ssl "$PROXY_SSL" '
    .dataPath = "/config"
    | .port = 30000
    | .upnp = false
    | .hostname = (if $hostname == "" then null else $hostname end)
    | .proxySSL = $proxy_ssl
    | .proxyPort = (if $proxy_ssl then 443 else null end)
' "$CONFIG" > "$CONFIG.tmp"
mv "$CONFIG.tmp" "$CONFIG"

# Files copied in over Samba or the File editor arrive owned by root
find "$FOUNDRY_DATA" "$INSTALL" ! -user node -exec chown node:node {} +

ARGS=("--dataPath=$FOUNDRY_DATA" --port=30000 --noupnp --headless)
WORLD=$(opt world)
if [[ -n "$WORLD" ]]; then
    ARGS+=("--world=$WORLD")
fi

VERSION=$(jq -r '.release | "\(.generation).\(.build)"' "$INSTALL/package.json" 2>/dev/null || echo "unknown")
echo "[INFO] Starting Foundry VTT ${VERSION} on port 30000 (Node.js $(node --version))"
cd "$INSTALL"
exec setpriv --reuid=node --regid=node --init-groups --inh-caps=-all \
    env HOME=/home/node node "$ENTRY" "${ARGS[@]}"
