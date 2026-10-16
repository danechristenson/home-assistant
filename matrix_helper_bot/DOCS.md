# Matrix Helper Bot

Lets people you choose reset forgotten Matrix passwords, and let players
onto your Minecraft server, from chat, without making them admins.

Talk to it like a person in any encrypted room it's in: "@Helper add
Steve" in your family's Minecraft chat. It only does what the people you
list are allowed to do; ask for a password reset in a shared room and the
new password goes to the private Password helpers room instead.

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

   For Minecraft, also install the Mosquitto broker add-on (see
   [Minecraft](#minecraft)).

4. **Start it.** The log shows `Ready: 2 helper(s)`. Helpers get invites to
   the rooms for their permissions; once they join, they can type `!help`
   there.

5. **Helpers must use a verified session.** The bot only reads requests
   from, and only lets its replies be read on, devices their owner has
   verified (in Element X: set up recovery or verify with another device;
   in Element: All settings → Encryption/Sessions). On an unverified device
   the bot's messages show as "Unable to decrypt", and it ignores requests.

## Talking to the bot

Start a message with the bot's name, the way you'd mention a person:
"@Helper add Steve", "Helper: list", or just "Helper" for help. (Its name is
the bot account's display name; `!` commands like `!help` work too.) Other
messages are ignored, so it can sit in a normal chat.

**Rooms:** the bot keeps two encrypted rooms of its own, **Password helpers**
and **Minecraft helpers**, and invites each helper to the ones for their
permissions. You can also invite it to any other room, such as a family
chat or a private chat with you: it joins when someone on the helper list
invites it, and declines everyone else.

**Who it listens to:** only people on the helper list, for the permissions
they have, from verified sessions and in encrypted rooms. Anyone else who
asks gets a polite no.

## Resetting passwords

- `help` lists what you're allowed to do.
- `reset-password @name` gives that person a new password. You can type
  the username or pick the person from the mention list. The bot replies
  with the new password; pass it on privately (in person, or a message they
  can read without signing in). The message is deleted after
  `password_message_minutes`. The person's other signed-in devices stay
  signed in, and they can change the password afterwards (Element → All
  settings → Account).

New passwords are only posted where everyone may see them. Ask in the
Password helpers room, or in another encrypted room whose members (including
invited ones) can all reset passwords, such as a private chat with the bot,
and the password is posted right there. Ask in a shared room, such as a
family chat, and the bot still resets it, but posts the password in the
Password helpers room with a mention of you (so you get a notification)
and only says "Done" in the shared room, without naming the account.

The bot refuses to reset:

- server admins (anyone in the Admin Room) and helpers: ask the server
  admin;
- deactivated accounts (resetting would bring them back);
- accounts on other servers, and the bot itself.

Each helper can reset at most 5 passwords an hour.

## Minecraft

Needs the **Minecraft Bedrock** add-on from this repository and the
**Mosquitto broker** add-on: the bot sends its requests to Minecraft over
MQTT, with credentials it gets from Home Assistant, so there's nothing to set
up beyond giving people the `minecraft` permission.

In any room with the bot, use the player's Xbox gamertag:

- `add Gamertag` lets them join the server, straight away, as a member;
  `add Gamertag as operator` (or `as visitor`) picks the role.
- `make Gamertag a member` / `an operator` / `a visitor` changes their role
  ("user" and "player" mean member, "op" means operator).
- `remove Gamertag` takes them off the server (and kicks them if they're
  on).
- `list` shows who's been added from chat, and who hasn't joined yet.

For example "@Helper add Steve as operator". (`!minecraft add Steve` works
too.) Members can play and build, visitors can only look around, operators
can use commands.

The bot only changes players added from chat. Players added in the
Minecraft panel in Home Assistant, or before (such as your own account),
can only be changed there, so a helper can't demote or remove them.

A gamertag is all you need: the Minecraft add-on learns the player's Xbox ID
from the server log the first time they join. Bedrock stores operators by
Xbox ID, so `op` for someone who hasn't joined yet takes effect
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
- **Minecraft requests** go over your MQTT broker, which only Home
  Assistant and add-ons can use. The Minecraft add-on only lets them add,
  re-role and remove players added from chat.
- Nothing is reachable from outside: the bot only talks to the Matrix
  add-on and the MQTT broker over Home Assistant's internal network, and
  opens no ports.

## Support

[Open an issue](https://github.com/danechristenson/home-assistant/issues).
