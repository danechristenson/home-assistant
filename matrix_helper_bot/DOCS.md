# Matrix Helper Bot

Lets people you choose reset forgotten Matrix passwords, and let players
onto your Minecraft server, from chat, without making them admins.

Each permission has its own end-to-end encrypted room: **Password helpers**
and **Minecraft helpers**. People are only invited to the rooms for the
permissions you give them, so someone who can only manage Minecraft players
never sees passwords.

## Do you need this?

Your Matrix server can't send email, so its "Forgot your password?" link
doesn't work, and only a server admin can give someone a new password. If
you'd like a few trusted people to handle that, install this bot. People who
use Element from the Home Assistant sidebar can already set their own
password there (**Password for phone apps**); this is for everyone else.

## Setup

1. **Create the bot account.** In Element, open the **Admin Room** and send:

   ```
   !admin users create-user helper
   ```

   The server replies with the new account's password. Copy it, then delete
   that reply (hover over it, ⋯ → Remove).

2. **Make the bot a server admin**, so it can reset passwords:

   ```
   !admin users make-user-admin @helper:matrix.example.com
   ```

   (Use your server name.) This adds the bot to the Admin Room.

3. **Install this add-on**, then on its Configuration tab set the
   **password** from step 1 and add your helpers:

   ```yaml
   helpers:
     - user: "@alex:matrix.example.com"
       reset_passwords: true
       minecraft: true
     - user: "@sam:matrix.example.com"
       minecraft: true
   ```

   For Minecraft, also set `minecraft_key` (see [Minecraft](#minecraft)).

4. **Start it.** The log shows `Ready: 2 helper(s)`. Helpers get invites to
   the rooms for their permissions; once they join, they can type `!help`
   there.

5. **Helpers must use a verified session.** The bot only reads requests
   from, and only lets its replies be read on, devices their owner has
   verified (in Element X: set up recovery or verify with another device;
   in Element: All settings → Encryption/Sessions). On an unverified device
   the bot's messages show as "Unable to decrypt", and it ignores requests.

## Resetting passwords

In the Password helpers room:

- `!help` lists what you're allowed to do.
- `!reset-password @name` gives that person a new password. You can type
  the username or pick the person from the mention list. The bot replies
  with the new password; pass it on privately (in person, or a message they
  can read without signing in). The message is deleted after
  `password_message_minutes`. The person's other signed-in devices stay
  signed in, and they can change the password afterwards (Element → All
  settings → Account).

The bot refuses to reset:

- server admins (anyone in the Admin Room) and helpers: ask the server
  admin;
- deactivated accounts (resetting would bring them back);
- accounts on other servers, and the bot itself.

Each helper can reset at most 5 passwords an hour.

## Minecraft

Needs the **Minecraft Bedrock** add-on from this repository. Make up a
random key of at least 16 characters (a password manager can generate one),
set it as `helper_bot_key` in Minecraft Bedrock and as `minecraft_key` here,
and restart both.

In the Minecraft helpers room, use the player's Xbox gamertag:

- `!minecraft add Gamertag` lets them join the server, straight away.
- `!minecraft op Gamertag` makes them an operator.
- `!minecraft member Gamertag` makes an operator a normal player again.
- `!minecraft remove Gamertag` takes them off the server (and kicks them if
  they're on).
- `!minecraft list` shows who's been added from chat, and who hasn't joined
  yet.

A gamertag is all you need: the Minecraft add-on learns the player's Xbox ID
from the server log the first time they join. Bedrock stores operators by
Xbox ID, so `!minecraft op` for someone who hasn't joined yet takes effect
the first time they do. Players already on the allowlist (for example
from before) aren't affected.

## Configuration

### Option: `username` / `password`

The bot's account from setup steps 1 and 2.

### Option: `helpers`

Who may use the bot, and what each may do. `user` is their Matrix ID (just
the name works too). Permissions, each with its own room:

- `reset_passwords`: may use `!reset-password` (Password helpers room).
- `minecraft`: may use the `!minecraft` commands (Minecraft helpers room).

The bot invites people to the rooms for their permissions and removes
anyone else. After changing the list, restart the add-on.

### Option: `minecraft_key` / `minecraft_url`

The same key as `helper_bot_key` in the Minecraft Bedrock add-on. Leave
`minecraft_url` empty: the bot finds Minecraft Bedrock on Home Assistant's
internal network.

### Option: `password_message_minutes`

How long a new password stays in the Password helpers room. Default 10.

### Option: `matrix_url`

Leave empty: the bot finds the Matrix add-on from this repository on Home
Assistant's internal network. Only set it if the log says it can't connect.

## Security

- **The helper rooms are end-to-end encrypted.** Requests and new
  passwords are only readable on the helpers' verified devices and the
  bot's; your server only stores them encrypted. The bot cross-signs its
  own device, so helpers' apps show it as verified, and it withholds its
  keys from any device that isn't cross-signed by its owner. So someone
  who signs in to a helper's account on a new device can't read passwords
  without also verifying that device. Password messages are deleted after
  `password_message_minutes`.
- **One step isn't encrypted:** the bot asks the server for a new password
  in the Admin Room, which can't be encrypted (the server's own admin bot
  doesn't support encryption). The server's reply contains the password, so
  the bot deletes it as soon as it has read it, usually within a second.
  The Admin Room's only members are server admins, and the bot's commands
  stay there, so admins can see every reset.
- **The bot is a full server admin.** Anyone who gets its password could
  run any admin command, so keep it to the add-on settings and don't use
  the account yourself. The bot only ever sends `users reset-password`, for
  a single checked username. Its encryption keys are kept in the add-on's
  data folder (included in Home Assistant backups).
- **The Minecraft key** only lets the bot add, promote and remove players
  added from chat; the Minecraft add-on only accepts it from Home
  Assistant's internal network.
- Nothing is reachable from outside: the bot only talks to the Matrix and
  Minecraft add-ons over Home Assistant's internal network, and opens no
  ports.

## Support

[Open an issue](https://github.com/danechristenson/home-assistant/issues).
