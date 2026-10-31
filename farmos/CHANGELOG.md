# Changelog

## 2.0.1

- Fix the build on farmOS 4: its image runs as `www-data`, so the add-on's setup
  (which needs root) failed at `chmod +x /run.sh`. The add-on now switches back to
  root, as on farmOS 3; Apache and drush still run as `www-data`.
- Fix the API key on new farmOS 4 sites: farmOS 4 no longer turns on its OAuth
  server or the Farm Manager role by default, or makes the OAuth signing keys.
  The add-on now enables both and makes the keys when they're missing. Sites
  upgraded from farmOS 3 already have them and keep their key.
- Map the add-on's own folder as `app_config`, the Supervisor's current name for
  `addon_config` (same folder; stops a deprecation warning in its log).

## 2.0.0

- Update farmOS 3.5.9 → 4.0.9.
- New major version: check the upstream release notes before merging.

## 1.0.0

- First release: runs farmOS 3.5.9 (the official image) with its own SQLite
  database, its site, uploaded files and database in the add-on's data
- Applies farmOS's database updates and rebuilds its caches on every start
- Sets the private files path farmOS needs for uploads such as CSV imports
- Makes an API key for sync scripts (an `api_sync` client) in the add-on's
  own folder
