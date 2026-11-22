#!/bin/sh
# One-time import from another NPMplus, then NPMplus's own start-up. Only
# for moving in; a later version drops this script.
#
# The other NPMplus's data folder (the one at its /data), copied to `import`
# in this add-on's config folder (/data/custom_nginx/import here), is copied
# in on the first start, then deleted: it holds private keys. Never over
# hosts already here.
set -eu

IMPORT=/data/custom_nginx/import
if [ -d "$IMPORT" ]; then
    if [ -e /data/npmplus/database.sqlite ]; then
        echo "[npmplus] Not importing: this add-on already has its own hosts. Delete the import folder." >&2
    elif [ ! -s "$IMPORT/npmplus/database.sqlite" ]; then
        echo "[npmplus] Not importing: no npmplus/database.sqlite in the import folder." >&2
    else
        echo "[npmplus] Importing another NPMplus's data"
        # Home Assistant's options file is this add-on's own; custom nginx
        # files go in the config folder itself
        rm -rf "$IMPORT/options.json" "$IMPORT/custom_nginx"
        cp -a "$IMPORT/." /data/
        rm -rf "$IMPORT"
        echo "[npmplus] Imported; import folder deleted"
    fi
fi

exec /usr/local/bin/entrypoint.sh
