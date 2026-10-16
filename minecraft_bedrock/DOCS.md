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

## The Minecraft dashboard

With **Keep a Minecraft dashboard** on (the default), the add-on creates a
**Minecraft** dashboard in Home Assistant and keeps it up to date:

- **Reports** (everyone): server status, who's online, supper time, a
  section for each tracked player (playing, playtime today and in total,
  last seen), playtime per day, players online, and joins and leaves.
- **Actions** (administrators only): supper time, quick actions, a message
  to everyone, players (add one, change roles, remove), and the game
  settings.

It changes when you track another player or the allowlist changes. If you
edit the dashboard yourself, the add-on notices and stops updating it;
delete it to get the add-on's version back.

## Supper time

Press **Supper time** (on the dashboard, or the `button` entity in an
automation) and everyone playing is told "Supper time in 5 minutes!", then
4, 3, 2 and 1. Then the tracked players (everyone, if nobody is tracked) are
kicked and kept out for a minute: if they rejoin, they're kicked again.
**Cancel supper time** stops the countdown and tells everyone.

## The web panel

The add-on also has a web panel (Settings → Add-ons → Minecraft Bedrock →
**Open web UI**, or turn on **Show in sidebar** there). It shows:

- whether the server is running, its Bedrock version and world, and who's
  online;
- every player on the allowlist, with their role, when they were last seen,
  whether they've joined yet, and where they were added (from chat, in the
  panel, or before);
- the settings from the Configuration tab.

You can add players, change their role and remove them there; changes apply
at once.

It also changes these while people are playing, without a restart:

- **Game:** difficulty, "Only allowed players" and game rules: keep
  inventory, show coordinates, player vs player, mob griefing, fire spreads,
  TNT explodes, day and night, weather changes, mobs spawn, and the share of
  players needed to sleep. Game rules are saved in the world; difficulty
  and "Only allowed players" are also saved to the add-on's options, so a
  restart keeps them.
- **Quick actions:** make it day or night, clear the weather, send a message
  to everyone playing, and kick someone who's online.

Everything else (server name, world, game mode, cheats, player limits,
distances, ports) is changed on the Configuration tab, and the add-on
restarts to apply it. Bedrock can't change the game mode while running.

## Home Assistant entities

With the **Mosquitto broker** add-on (and Home Assistant's MQTT
integration), the add-on adds a **Minecraft Bedrock** device to Home
Assistant, with:

| Entity | |
| --- | --- |
| Server | Running or not (a health check) |
| Players online | How many, with their names as the `players` attribute |
| Bedrock version, World, World seed, Game mode | About the server and world |
| Allowed players, Last player to join, Last join, Server started | Activity |
| Player activity | An event when anyone joins or leaves (`joined` / `left`, with the `player`), for notifications |
| Only allowed players, Difficulty, game rule switches, Players needed to sleep | Controls: change them live |
| Make it day, Make it night, Clear the weather, Kick everyone | Buttons |
| Message everyone | Text: what you set is sent to everyone playing |
| Supper time, Cancel supper time | Buttons (see [Supper time](#supper-time)); a **Supper time** sensor shows the countdown |
| Add player | Text: a gamertag to let join (as a member) |
| *Name* role, Remove *Name* | For each allowed player: change their role, or remove them |

For each gamertag in **Tracked players**: *playing*, *playtime today*
(resets at midnight), *total playtime* and *last seen*. Playtime is counted
by the minute while they're online, and works with Home Assistant's
statistics and utility meters (e.g. weekly totals).

Example automations: press **Supper time** at 17:55; at 20:50 set
**Message everyone** to "Bedtime in 10 minutes", at 21:00 press **Kick
everyone**.

Note: Home Assistant can't limit which users may use an entity. Non-admin
users could change the controls if they find them, so keep them off their
dashboards.

## Who can join

**Only allowed players** is on by default: only players on the allowlist can
join. Add them on the dashboard (or the web panel), or from chat with the
[Matrix Helper Bot](https://github.com/danechristenson/home-assistant/tree/main/matrix_helper_bot):
give people the `minecraft` permission there, and in a room with the bot
say "@Helper add Gamertag", "@Helper add Gamertag as operator", "@Helper
make Gamertag a visitor", "@Helper remove Gamertag" or "@Helper list". The
bot reaches this add-on over MQTT, so there's nothing else to set up.

The bot only changes players added from chat. Players added in Home
Assistant or before (such as your own account) can only be changed there,
so a helper can't demote or remove them.

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
| Tracked players | Players to track in Home Assistant (playtime and more), by gamertag or allowlist name. |
| Keep a Minecraft dashboard | Create and update the **Minecraft** dashboard. |

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

Home Assistant entities and the Matrix Helper Bot use your MQTT broker. The
web panel (port 8791) only answers Home Assistant's ingress.

## Backups

Worlds, the allowlist and settings are in Home Assistant backups. The
Bedrock program and Mojang's built-in packs aren't (they're downloaded again
on start), which keeps backups small.

## Support

[Open an issue](https://github.com/danechristenson/home-assistant/issues).
Server problems may belong upstream:
[itzg/docker-minecraft-bedrock-server](https://github.com/itzg/docker-minecraft-bedrock-server/issues).
