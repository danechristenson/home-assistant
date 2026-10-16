# Home Assistant Add-on: Matrix Helper Bot

Lets people you choose reset forgotten Matrix passwords and manage Minecraft players from chat.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

A companion to the [Matrix][matrix] add-on, which must also be installed.

Without email, only a server admin can reset a forgotten password. This bot
lets trusted helpers do it too, without making them admins: they type
`!reset-password @name` in a private, end-to-end encrypted **Server
helpers** room, and the bot posts a new password for them to pass on.

- Talk to it in any encrypted room it's invited to: "@Helper add Steve"
- You list each helper and what they may do; passwords only go where
  everyone may see them
- Minecraft: `add Gamertag`, `op`, `remove` (with the Minecraft Bedrock
  add-on from this repository)
- It never resets server admins', helpers' or deactivated accounts
- Only works from verified devices; new passwords are deleted after a few minutes

See the **Documentation** tab for setup.

[matrix]: https://github.com/danechristenson/home-assistant/tree/main/matrix
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
