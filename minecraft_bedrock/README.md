# Home Assistant Add-on: Minecraft Bedrock

A Minecraft Bedrock server for phones, tablets, consoles and Windows.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

Runs [itzg/minecraft-bedrock-server][itzg], the widely used Bedrock server
image, with its settings on the Home Assistant Configuration tab.

- Only allowed players can join (an allowlist), on by default
- A **Minecraft** sidebar panel: who's online, every allowed player and their
  role, game rules (keep inventory, PvP, mob griefing…), difficulty, and
  quick actions (day, clear weather, message everyone), all live
- Home Assistant entities over MQTT: players online, playtime per player,
  join/leave events, version, seed, and controls for automations
- Works with the [Matrix Helper Bot][bot]: "@Helper add Steve as operator"
  in an encrypted chat room
- Follows Mojang's Bedrock releases through this repository's daily updates,
  so players' apps and the server stay on the same version

See the **Documentation** tab for setup.

[itzg]: https://github.com/itzg/docker-minecraft-bedrock-server
[bot]: https://github.com/danechristenson/home-assistant/tree/main/matrix_helper_bot
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
