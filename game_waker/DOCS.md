# Game Waker

Game servers use memory and CPU even when nobody is playing. This add-on
lets you stop them while they're idle: when someone tries to join a stopped
game, Game Waker starts it.

The first try to join fails, because the game isn't running yet. Wait until
it has started (often a few minutes: many game add-ons check for updates when
they start) and join again.

## Setup

1. Find the game add-on's slug: open the add-on in Home Assistant; it's the
   last part of the page's address, e.g. `9f95fc58_valheim_steamcmd_server`.
2. On this add-on's **Configuration** tab, add it under **Games** with the
   UDP ports players connect to:

   ```yaml
   - addon: 9f95fc58_valheim_steamcmd_server
     udp_ports: 2456-2457
   ```

3. Start this add-on. The log shows `starts when someone tries to join`.
4. Stop the game when it's idle, for example with an automation that stops
   it after 30 minutes with nobody on (see below).

Game Waker only watches the game's ports, the way `tcpdump` does; it never
takes them, so the game can still be started from its own add-on page as
usual. While the game is running, Game Waker does nothing.

## Stopping a game when nobody's playing

Game Waker only starts games. To stop one, use an automation with a sensor
for the number of players. For example, every 5 minutes, stop Valheim when
it has had no players for 30 minutes:

```yaml
triggers:
  - trigger: time_pattern
    minutes: /5
conditions:
  - condition: state
    entity_id: switch.valheim_dedicated_server_steamcmd
    state: "on"
    for:
      minutes: 30
  - condition: state
    entity_id: sensor.valheim_players
    state: "0"
    for:
      minutes: 30
actions:
  - action: switch.turn_off
    target:
      entity_id: switch.valheim_dedicated_server_steamcmd
```

The add-on's on/off switch comes from the Supervisor integration (enable it
on the add-on's device if it's disabled).

## What wakes a game

Any UDP packet to the game's ports from outside Home Assistant: a player
joining, but also a server browser or a stray scan from the internet. A game
woken by mistake is stopped again by your automation. Packets from Home
Assistant itself (127.0.0.1, and the add-ons' own network) never wake it, so
a server-status sensor can keep checking a stopped game.

Each wake fires a `game_waker_wake` event in Home Assistant, with the
add-on's slug (`addon`) and the address that knocked (`from`), for a
notification such as "Valheim is starting".

## Permissions

To watch the ports and start the games, the add-on uses the host network,
raw packet capture (`NET_RAW`) and the Supervisor's manager role (to start
other add-ons). It doesn't change any game or open any ports.
