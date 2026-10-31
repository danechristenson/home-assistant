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

# An option from the add-on's settings (true/false for switches, one per line
# for lists); the default when there's no options file, as in the smoke test
option() {
  php -d display_startup_errors=0 -d display_errors=0 -r '$o = is_file("/data/options.json") ? json_decode(file_get_contents("/data/options.json"), true) : [];
    $v = $o[$argv[1]] ?? $argv[2];
    echo is_bool($v) ? ($v ? "true" : "false") : (is_array($v) ? implode("\n", $v) : $v);' "$1" "${2:-}" 2> /dev/null
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

# HTTPS on farmOS's own port: Apache's port-80 site (port 8096 outside) speaks
# TLS with the certificates from Home Assistant's /ssl. Apache reads them at
# start, so a renewed certificate needs a restart of the add-on.
VHOST=/etc/apache2/sites-available/000-default.conf
sed -i '/# add-on https: start/,/# add-on https: end/d' "${VHOST}"
if [ "$(option ssl false)" = "true" ]; then
  CERTFILE="/ssl/$(option certfile fullchain.pem)"
  KEYFILE="/ssl/$(option keyfile privkey.pem)"
  for f in "${CERTFILE}" "${KEYFILE}"; do
    if [ ! -f "${f}" ]; then
      echo "[ERROR] HTTPS is on but ${f} does not exist" >&2
      exit 1
    fi
  done
  a2enmod -q ssl > /dev/null
  sed -i "/<VirtualHost \*:80>/a\\
\t# add-on https: start\\
\tSSLEngine on\\
\tSSLCertificateFile ${CERTFILE}\\
\tSSLCertificateKeyFile ${KEYFILE}\\
\t# add-on https: end" "${VHOST}"
  echo "Serving farmOS over HTTPS"
fi

# Behind a reverse proxy: farmOS believes the proxy's X-Forwarded-* headers
# (the address and https:// people used) from the trusted addresses only.
# Rewritten at every start from the options, and included by settings.php.
PROXY_SETTINGS=/data/sites/default/settings.addon.php
{
  echo "<?php"
  echo "// Written by the add-on at every start from its options: edits here are lost."
  if [ "$(option reverse_proxy false)" = "true" ]; then
    echo "\$settings['reverse_proxy'] = TRUE;"
    echo "\$settings['reverse_proxy_addresses'] = ["
    option trusted_proxies 172.30.32.0/23 | while read -r address || [ -n "${address}" ]; do
      [ -n "${address}" ] && echo "  '${address}',"
    done
    echo "];"
    echo "\$settings['reverse_proxy_trusted_headers'] = \\Symfony\\Component\\HttpFoundation\\Request::HEADER_X_FORWARDED_FOR"
    echo "  | \\Symfony\\Component\\HttpFoundation\\Request::HEADER_X_FORWARDED_HOST"
    echo "  | \\Symfony\\Component\\HttpFoundation\\Request::HEADER_X_FORWARDED_PORT"
    echo "  | \\Symfony\\Component\\HttpFoundation\\Request::HEADER_X_FORWARDED_PROTO;"
  fi
} > "${PROXY_SETTINGS}"
chown www-data:www-data "${PROXY_SETTINGS}"
[ "$(option reverse_proxy false)" = "true" ] && echo "Trusting a reverse proxy at: $(option trusted_proxies 172.30.32.0/23 | paste -sd ' ' -)"

if [ -f "${SETTINGS}" ] && grep -q "^\$databases\['default'\]" "${SETTINGS}"; then
  if ! grep -q "settings.addon.php" "${SETTINGS}"; then
    chmod u+w "${SETTINGS}"
    printf "\nif (file_exists(__DIR__ . '/settings.addon.php')) {\n  include __DIR__ . '/settings.addon.php';\n}\n" >> "${SETTINGS}"
    chmod 444 "${SETTINGS}"
  fi
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
  # The keys OAuth tokens are signed with: made at install on farmOS 3, not on farmOS 4
  if [ ! -f /data/keys/private.key ]; then
    echo "Making the OAuth signing keys in /data/keys"
    drush simple-oauth:generate-keys /data/keys || echo "Making the OAuth keys failed: see above"
  fi
else
  echo "farmOS isn't installed yet: open the web UI and choose SQLite, with the database file /data/db/farmos.sqlite"
fi

exec apache2-foreground
