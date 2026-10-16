# Changelog

## 1.0.0

- First release: helpers can reset forgotten passwords with
  `!reset-password @name` in the bot's end-to-end encrypted Server helpers
  room, from verified devices only. Built on mautrix-go v0.32.0.
- Helpers with the `minecraft` permission can allow, promote and remove
  Minecraft players by gamertag in a separate encrypted Minecraft helpers
  room (needs the Minecraft Server add-on with the bot hook).
- Each permission has its own room, so Minecraft helpers never see
  passwords.
