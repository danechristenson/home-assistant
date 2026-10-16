# Matrix Helper Bot

Lets people you choose reset forgotten Matrix passwords from a chat room,
without making them server admins.

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
   ```

4. **Start it.** The log shows `Ready: 1 helper(s) in the Server helpers
   room`. Each helper gets an invite to the encrypted **Server helpers**
   room; once they join, they can type `!help` there.

5. **Helpers must use a verified session.** The bot only reads requests
   from, and only lets its replies be read on, devices their owner has
   verified (in Element X: set up recovery or verify with another device;
   in Element: All settings → Encryption/Sessions). On an unverified device
   the bot's messages show as "Unable to decrypt", and it ignores requests.

## Using it

In the Server helpers room:

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

## Configuration

### Option: `username` / `password`

The bot's account from setup steps 1 and 2.

### Option: `helpers`

Who may use the bot, and what each may do. `user` is their Matrix ID (just
the name works too). Permissions:

- `reset_passwords`: may use `!reset-password`.

The bot invites everyone listed to the Server helpers room and removes
anyone else. After changing the list, restart the add-on.

### Option: `password_message_minutes`

How long a new password stays in the Server helpers room. Default 10.

### Option: `matrix_url`

Leave empty: the bot finds the Matrix add-on from this repository on Home
Assistant's internal network. Only set it if the log says it can't connect.

## Security

- **The Server helpers room is end-to-end encrypted.** Requests and new
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
- Nothing is reachable from outside: the bot only talks to the Matrix
  add-on over Home Assistant's internal network, and opens no ports.

## Support

[Open an issue](https://github.com/danechristenson/home-assistant/issues).
