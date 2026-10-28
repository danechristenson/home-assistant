#!/bin/bash
# Keeps farmOS's state in the add-on's /data (the Drupal sites directory with
# settings.php and the public files, the private files, the OAuth keys, and the
# SQLite database the installer is pointed at), keeps it up to date once it's
# installed, and makes the API key for sync scripts in /config.
set -e

DRUPAL=/opt/drupal
SETTINGS=/data/sites/default/settings.php
mkdir -p /data/sites /data/keys /data/db /data/private/files

drush() {
  runuser -u www-data -- "${DRUPAL}/vendor/bin/drush" --root="${DRUPAL}/web" "$@"
}

# The first start: seed the sites directory from the image
if [ -z "$(ls -A /data/sites)" ]; then
  echo "First start: copying farmOS's sites directory into /data"
  cp -rp "${DRUPAL}/web/sites/." /data/sites/
fi

for pair in "web/sites:/data/sites" "keys:/data/keys"; do
  target="${DRUPAL}/${pair%%:*}"
  if [ ! -L "${target}" ]; then
    rm -rf "${target}"
    ln -s "${pair#*:}" "${target}"
  fi
done

chown -R www-data:www-data /data
# The add-on's own folder (addon_configs on the host): the API key
chown www-data:www-data /config && chmod 700 /config

if [ -f "${SETTINGS}" ] && grep -q "^\$databases\['default'\]" "${SETTINGS}"; then
  # The installer writes settings.php without the private files path farmOS
  # needs for uploads such as CSV imports: outside the web root
  if ! grep -q "^\$settings\['file_private_path'\]" "${SETTINGS}"; then
    echo "Setting farmOS's private files path to /data/private/files"
    chmod u+w "${SETTINGS}"
    printf "\n\$settings['file_private_path'] = '/data/private/files';\n" >> "${SETTINGS}"
    chmod 444 "${SETTINGS}"
  fi
  # Any database updates a new farmOS version brings, then the caches (also
  # how Drupal picks up a settings.php change)
  echo "Applying farmOS database updates and rebuilding caches"
  drush updatedb -y || echo "Database updates failed: see above"
  drush cache:rebuild || echo "Cache rebuild failed: see above"
  drush php:script /provision_api.php || echo "Setting up the API key failed: see above"
else
  echo "farmOS isn't installed yet: open the web UI and choose SQLite, with the database file /data/db/farmos.sqlite"
fi

exec apache2-foreground
