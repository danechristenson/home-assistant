#!/bin/bash
# Home Assistant options -> itzg/minecraft-bedrock-server settings, then the
# image's own entrypoint. The server's output is also kept in run/bedrock.log,
# where minecraft-bot-api learns players' Xbox IDs.
set -euo pipefail

OPTIONS=/data/options.json

opt() {
    jq -r --arg key "$1" '.[$key] | if . == null then "" else tostring end' "$OPTIONS"
}

# Export a server.properties setting (itzg maps the variable) if the option is set
setting() {
    local value
    value=$(opt "$2")
    if [[ -n "$value" ]]; then
        export "$1=$value"
    fi
}

if [[ "$(opt eula)" != "true" ]]; then
    echo "[ERROR] Turn on 'eula' on the Configuration tab to accept the Minecraft End User License Agreement (https://www.minecraft.net/eula)." >&2
    exit 1
fi
export EULA=TRUE

setting SERVER_NAME server_name
setting LEVEL_NAME level_name
setting LEVEL_SEED level_seed
setting GAMEMODE gamemode
setting DIFFICULTY difficulty
setting ALLOW_CHEATS allow_cheats
setting ALLOW_LIST allow_list
setting DEFAULT_PLAYER_PERMISSION_LEVEL default_player_permission_level
setting MAX_PLAYERS max_players
setting VIEW_DISTANCE view_distance
setting TICK_DISTANCE tick_distance
setting PLAYER_IDLE_TIMEOUT player_idle_timeout
setting SERVER_PORT server_port
setting SERVER_PORT_V6 server_port_v6
setting ENABLE_LAN_VISIBILITY lan_visibility
setting TRANSPORT transport
# Allowlists and Xbox IDs only work with Xbox sign-in
export ONLINE_MODE=true

# One-time import of a world moved from another add-on (see the
# Documentation tab): worlds that already exist here are never overwritten,
# and the allowlist and permissions are only taken if there are none yet
IMPORT=/share/minecraft_bedrock_import
if [[ -d "$IMPORT/worlds" ]]; then
    mkdir -p /data/worlds
    for world in "$IMPORT"/worlds/*/; do
        [[ -d "$world" ]] || continue
        name=$(basename "$world")
        if [[ -e "/data/worlds/$name" ]]; then
            echo "[INFO] Not importing world '$name' from $IMPORT: it already exists here"
            continue
        fi
        cp -a "$world" "/data/worlds/$name"
        echo "[INFO] Imported world '$name' from $IMPORT"
    done
    for file in allowlist.json permissions.json; do
        if [[ -s "$IMPORT/$file" && ! -s "/data/$file" ]]; then
            cp "$IMPORT/$file" "/data/$file"
            echo "[INFO] Imported $file from $IMPORT"
        fi
    done
    echo "[INFO] Once the world looks right, delete $IMPORT"
fi

mkdir -p /data/run
: > /data/run/bedrock.log

BOT_KEY=$(opt helper_bot_key)
if [[ -n "$BOT_KEY" ]]; then
    if (( ${#BOT_KEY} < 16 )); then
        echo "[ERROR] 'helper_bot_key' must be at least 16 characters; the Matrix Helper Bot can't manage players until it is." >&2
    else
        BOT_KEY="$BOT_KEY" minecraft-bot-api &
    fi
fi
unset BOT_KEY

echo "[INFO] Starting Bedrock ${VERSION} (world '$(opt level_name)')"
exec /opt/demoter-entry.sh > >(tee -a /data/run/bedrock.log) 2>&1
