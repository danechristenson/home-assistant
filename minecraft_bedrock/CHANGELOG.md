# Changelog

## 2026.10.1.7

- **Supper time**: a 5-minute countdown in the game, then the tracked
  players (everyone, if none are tracked) are kicked and kept out for a
  minute. Buttons to start and cancel, and a countdown sensor.
- Players in Home Assistant: an **Add player** text, and a role select and
  **Remove** button for each allowed player (added and removed as the
  allowlist changes).
- The add-on keeps a **Minecraft** dashboard (Reports for everyone, Actions
  for administrators), built from the tracked players and the allowlist.
  It stops updating it if you edit it. Option: **Keep a Minecraft
  dashboard**.
- The Matrix Helper Bot now reaches the add-on over MQTT: the
  `helper_bot_key` option and port 8790 are gone.

## 2026.10.1.6

- Tracked players can be allowlist names: a player listed as "Robin" is
  tracked by the Xbox ID on the allowlist entry named Robin, whatever their
  gamertag is.
- The server-running and version entities are now
  `binary_sensor.minecraft_bedrock_server_running` and
  `sensor.minecraft_bedrock_bedrock_version` (the old ids clashed with the
  Supervisor integration's add-on entities). Entities that are no longer
  published, e.g. players removed from Tracked players, are removed.

## 2026.10.1.5

- Home Assistant entities over MQTT discovery (needs the Mosquitto broker
  add-on): server running, players online, version, world, seed, game mode,
  allowed players, last join, server started; a join/leave event; and
  controls (allowlist, difficulty, game rules, day/night, clear weather,
  kick everyone, message everyone).
- **Tracked players** option: playing, playtime today, total playtime and
  last seen for each listed gamertag.

## 2026.10.1.4

- The Minecraft panel changes the game while people play, without a
  restart: difficulty, "Only allowed players", and game rules (keep
  inventory, show coordinates, PvP, mob griefing, fire, TNT, day and night,
  weather, mob spawning, players needed to sleep). Difficulty and the
  allowlist switch are saved to the add-on's options too.
- Quick actions: make it day or night, clear the weather, message everyone,
  kick someone who's online.

## 2026.10.1.3

- Only kick a removed player if they're online (kicking someone offline
  logged a harmless "No targets matched selector" error).

## 2026.10.1.2

- New **Minecraft** sidebar panel: server status, who's online, every allowed
  player with their role and when they were last seen, and the settings. Add
  players, change their role and remove them there.
- The visitor role (look around only), alongside member and operator.
- The Matrix Helper Bot can only change players added from chat; players
  added in the panel or before can't be demoted or removed from chat.

## 2026.10.1.1

- Update itzg/minecraft-bedrock-server 2026.10.0 → 2026.10.1.

## 2026.10.0.2

- Import a world moved from another add-on: put it in
  `/share/minecraft_bedrock_import/worlds/<name>` and start the add-on. Existing
  worlds are never overwritten.

## 2026.10.0.1

- First release: itzg/minecraft-bedrock-server 2026.10.0 with Bedrock
  1.26.52.3, settings on the Configuration tab, and player management from
  the Matrix Helper Bot.
