# Changelog

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
