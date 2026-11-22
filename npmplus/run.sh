#!/bin/sh
# Home Assistant settings for NPMplus, then NPMplus's own start-up. Everything
# NPMplus keeps (hosts, certificates, users) is in /data, as with its own
# compose file.
set -eu

# NPMplus settings (environment variables, see its compose.yaml). TZ comes
# from Home Assistant: the Supervisor sets it for every add-on.
export NPM_PORT=81
export DISABLE_GRAVATAR=true

# Moving in from another NPMplus: its data folder's contents, put in
# /config/import, are copied in on the first start, then removed (they hold
# private keys). Only into an empty add-on: never over hosts already here.
IMPORT=/config/import
if [ -d "$IMPORT" ]; then
    if [ -e /data/npmplus/database.sqlite ]; then
        echo "[npmplus] Not importing /config/import: this add-on already has its own hosts. Remove the folder." >&2
    elif [ ! -s "$IMPORT/npmplus/database.sqlite" ]; then
        echo "[npmplus] Not importing /config/import: no npmplus/database.sqlite in it." >&2
    else
        echo "[npmplus] Importing another NPMplus's data from /config/import"
        # (Home Assistant's options file is this add-on's own; a custom_nginx
        # folder is moved to /config below, a link to one elsewhere is dropped)
        rm -f "$IMPORT/options.json"
        if [ -L "$IMPORT/custom_nginx" ]; then rm -f "$IMPORT/custom_nginx"; fi
        cp -a "$IMPORT/." /data/
        rm -rf "$IMPORT"
        echo "[npmplus] Imported; /config/import removed"
    fi
fi

# Custom nginx files (NPMplus's /data/custom_nginx, e.g. http_top.conf) live in
# this add-on's config folder, where they can be edited (Samba, SSH, File
# editor); /data/custom_nginx points there.
mkdir -p /config/custom_nginx
if [ -d /data/custom_nginx ] && [ ! -L /data/custom_nginx ]; then
    cp -an /data/custom_nginx/. /config/custom_nginx/
    rm -rf /data/custom_nginx
fi
ln -sfn /config/custom_nginx /data/custom_nginx

exec /usr/local/bin/entrypoint.sh
