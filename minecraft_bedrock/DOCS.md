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

## The Minecraft panel

The add-on adds **Minecraft** to the Home Assistant sidebar (for
administrators). It shows:

- whether the server is running, its Bedrock version and world, and who's
  online;
- every player on the allowlist, with their role, when they were last seen,
  whether they've joined yet, and where they were added (from chat, in the
  panel, or before);
- the settings from the Configuration tab.

You can add players, change their role and remove them there; changes apply
at once. Settings are changed on the Configuration tab.

## Who can join

**Only allowed players** is on by default: only players on the allowlist can
join. Add them in the panel, or from chat with the
[Matrix Helper Bot](https://github.com/danechristenson/home-assistant/tree/main/matrix_helper_bot):

1. Make up a random key of at least 16 characters (a password manager can
   generate one).
2. Set it as **Matrix Helper Bot key** here and as `minecraft_key` in the
   Matrix Helper Bot, restart both, and give people the `minecraft`
   permission there.
3. In a room with the bot: "@Helper add Gamertag", "@Helper add Gamertag as
   operator", "@Helper make Gamertag a visitor", "@Helper remove Gamertag",
   "@Helper list".

The bot only changes players added from chat. Players added in the panel or
before (such as your own account) can only be changed in the panel, so a
helper can't demote or remove them.

Changes apply at once, without a restart. A gamertag is all you need: the
server records the player's Xbox ID the first time they join. Bedrock stores
roles by Xbox ID, so a role chosen before someone's first join takes effect
when they first do (until then they get the **New players are** level).

**Roles:** members can play and build, visitors can only look around, and
operators can also use commands and change game settings. Operators can only use commands while
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

The add-on imports worlds from `/share/minecraft_bedrock_import` when it
starts:

1. Stop the old Minecraft add-on, and take a backup of just that add-on
   (Settings → System → Backups → Create backup → Partial).
2. From the backup, copy the world folder (`worlds/<name>`), plus
   `allowlist.json` and `permissions.json` if you want the same players, into
   `/share/minecraft_bedrock_import/` (keeping the `worlds/<name>` layout),
   for example with the Terminal & SSH or Samba add-on.
3. Set **World** to that name here and start this add-on. The log says
   `Imported world '<name>'`.
4. Once you've checked the world, delete `/share/minecraft_bedrock_import`.

A world that already exists here is never overwritten, and the allowlist and
permissions are only imported if this add-on has none yet. Keep the old
add-on, stopped, until you've played on the new one.

## Network

The add-on uses the host network: Bedrock's NetherNet connections need it.
NetherNet sets up connections through Xbox Live (the Friends tab), which can
often get through your router without a port forward. If players outside
your home can't connect, forward UDP 19132 to Home Assistant, after reading
[Before you forward ports](https://github.com/danechristenson/home-assistant#before-you-forward-ports).

The Matrix Helper Bot reaches the add-on on port 8790, which only answers
Home Assistant's internal network, with the key. The panel (port 8791) only
answers Home Assistant's sidebar.

## Backups

Worlds, the allowlist and settings are in Home Assistant backups. The
Bedrock program and Mojang's built-in packs aren't (they're downloaded again
on start), which keeps backups small.

## Support

[Open an issue](https://github.com/danechristenson/home-assistant/issues).
Server problems may belong upstream:
[itzg/docker-minecraft-bedrock-server](https://github.com/itzg/docker-minecraft-bedrock-server/issues).
