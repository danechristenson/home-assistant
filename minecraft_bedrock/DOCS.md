# Minecraft Bedrock

A Minecraft Bedrock server: phones, tablets, Xbox, PlayStation, Switch and
Windows can join (not Minecraft Java Edition).

It runs [itzg/minecraft-bedrock-server](https://github.com/itzg/docker-minecraft-bedrock-server),
which downloads Mojang's Bedrock server and applies the settings below. This
repository's daily updates follow new Bedrock releases; with **Auto update**
on, the server restarts onto the new version, so it matches players' apps.

## Setup

1. Install the add-on, open the **Configuration** tab and turn on **Accept
   the Minecraft EULA**. Set the server name and world.
2. Start it. The first start downloads the Bedrock server (a minute or two);
   the log then shows `Server started`.
3. Players join from **Play → Friends** (at home it shows up automatically)
   or **Servers → Add server** with your Home Assistant address and port
   19132.

## Who can join

**Only allowed players** is on by default: only players on the allowlist can
join. Add them from chat with the [Matrix Helper Bot](https://github.com/danechristenson/home-assistant/tree/main/matrix_helper_bot):

1. Make up a random key of at least 16 characters (a password manager can
   generate one).
2. Set it as **Matrix Helper Bot key** here and as `minecraft_key` in the
   Matrix Helper Bot, restart both, and give people the `minecraft`
   permission there.
3. In the bot's Minecraft helpers room: `!minecraft add Gamertag`,
   `!minecraft op Gamertag`, `!minecraft remove Gamertag`, `!minecraft list`.

Changes apply at once, without a restart. A gamertag is all you need: the
server records the player's Xbox ID the first time they join. Bedrock stores
operators by Xbox ID, so making someone an operator before they've joined
takes effect when they first do. Operators can only use commands while
**Allow cheats** is on.

Turning **Only allowed players** off lets anyone who finds the server join,
as **New players are** (visitor, member or operator).

## Configuration

| Option | |
| --- | --- |
| Accept the Minecraft EULA | Required. |
| Server name | Shown in the Friends tab and server list. |
| World | The world folder (`/data/worlds/<name>`). A new name creates a new world; the old one is kept. |
| Seed for new worlds | Only used when a new world is created. |
| Game mode, Difficulty, Allow cheats | As in the game. |
| Only allowed players | The allowlist (above). |
| New players are | Permission level for players without one. |
| Most players at once, View distance, Simulation distance, Kick idle players after | Server performance and limits. |
| Show on the home network | Lets devices at home find the server in the Friends tab. |
| Port (IPv4 / IPv6) | 19132 / 19133 by default. |
| Connection type | NetherNet, which current Bedrock versions require. |
| Matrix Helper Bot key | Shared with the Matrix Helper Bot; empty turns bot access off. |

## Moving from another Minecraft add-on

Worlds are folders in `/data/worlds`. To bring a world over (for example
from "Minecraft Server for HA"), stop both add-ons and copy the old add-on's
`worlds/<name>` folder, plus `allowlist.json` and `permissions.json`, into
this add-on's data folder, then set **World** to that name. On Home Assistant
OS the data folders are `/mnt/data/supervisor/addons/data/<add-on>` on the
host (reachable with the developer SSH access on port 22222). Keep the old
add-on, stopped, until you've checked the world.

## Network

The add-on uses the host network: Bedrock's NetherNet connections need it.
NetherNet sets up connections through Xbox Live (the Friends tab), which can
often get through your router without a port forward. If players outside
your home can't connect, forward UDP 19132 to Home Assistant, after reading
[Before you forward ports](https://github.com/danechristenson/home-assistant#before-you-forward-ports).

The Matrix Helper Bot reaches the add-on on port 8790, which only answers
Home Assistant's internal network, with the key.

## Backups

Worlds, the allowlist and settings are in Home Assistant backups. The
Bedrock program and Mojang's built-in packs aren't (they're downloaded again
on start), which keeps backups small.

## Support

[Open an issue](https://github.com/danechristenson/home-assistant/issues).
Server problems may belong upstream:
[itzg/docker-minecraft-bedrock-server](https://github.com/itzg/docker-minecraft-bedrock-server/issues).
