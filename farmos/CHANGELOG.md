# Changelog

## 1.0.0

- First release: farmOS 3.5.9 (the official image) with its own SQLite
  database, its site, uploaded files and database in the add-on's data
- Applies farmOS's database updates and rebuilds its caches on every start
- Sets the private files path farmOS needs for uploads such as CSV imports
- Makes an API key for sync scripts (an `api_sync` client) in the add-on's
  own folder
