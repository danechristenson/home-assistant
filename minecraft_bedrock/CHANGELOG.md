# Changelog

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
