# Changelog

## 1.4.2

- Update github.com/mattn/go-colorable v0.1.14 → v0.1.16.
- Update github.com/mattn/go-isatty v0.0.20 → v0.0.24.
- Update github.com/tidwall/match v1.1.1 → v1.2.0.
- Update github.com/tidwall/pretty v1.2.1 → v1.2.2.

## 1.4.1

- Update Home Assistant base image 3.22 → 3.24.

## 1.4.0

- Minecraft requests go to the Minecraft Bedrock add-on over MQTT, with
  credentials from Home Assistant: no more `minecraft_key` or
  `minecraft_url` to set (needs the Mosquitto broker add-on).

## 1.3.0

- Minecraft roles in plain words: "add Steve as operator", "make Steve a
  user", "make Steve a visitor" (visitors can only look around).
- The bot can only change Minecraft players added from chat; players added
  in the Minecraft panel or before are refused, so a helper can't demote or
  remove them.

## 1.2.0

- Ask for a password reset in a shared room (such as a family chat) and the
  bot now does it, posting the new password in the Password helpers room
  with a mention of you, and only "Done" in the shared room.

## 1.1.0

- Talk to the bot in any encrypted room: start a message with its name
  ("@Helper add Steve", "Helper: list", "Helper" for help). `!` commands
  still work.
- It joins rooms when a listed helper invites it, and declines other
  invites.
- New passwords are only posted where everyone may see them: the Password
  helpers room, or a room whose members (including invited ones) can all
  reset passwords.
- Fix: the bot can replace its encryption identity if its data is lost
  (Continuwuity needs the password confirmation in the current format).

## 1.0.1

- Fix: saving the configuration failed with "expected a URL" while the
  optional `matrix_url` and `minecraft_url` were empty (their default).

## 1.0.0

- First release: helpers can reset forgotten passwords with
  `!reset-password @name` in the bot's end-to-end encrypted Server helpers
  room, from verified devices only. Built on mautrix-go v0.32.0.
- Helpers with the `minecraft` permission can allow, promote and remove
  Minecraft players by gamertag in a separate encrypted Minecraft helpers
  room (with the Minecraft Bedrock add-on from this repository).
- Each permission has its own room, so Minecraft helpers never see
  passwords.
