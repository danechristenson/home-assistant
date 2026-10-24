# Game Waker

Game servers use memory and CPU even when nobody is playing. This add-on
lets you stop them while they're idle and starts them again when they're
wanted:

- **From home or your VPN**, just join: the game's first knock on its port
  starts it.
- **From anywhere else**, open the start page (e.g. `valheim.example.com`)
  and press **Start**. That address is remembered, so next time joining from
  there is enough.

The first try to join fails, because the game isn't running yet. Wait until
it has started (often a minute or two: many game add-ons check for updates
when they start) and join again. The start page shows when it's ready.

## Why not wake on any knock?

Server browsers and trackers query game ports all over the internet, every
few minutes, with exactly the packet a joining game sends first. Woken by
those, a game would hardly ever stay stopped. So only addresses you trust
wake a game: your home networks, the **Other addresses** you list, and
addresses that pressed Start on the start page (forgotten after 60 days
without use).

## Setup

1. Find the game add-on's slug: open the add-on in Home Assistant; it's the
   last part of the page's address, e.g. `9f95fc58_valheim_steamcmd_server`.
2. On this add-on's **Configuration** tab, add it under **Games**:

   ```yaml
   - name: Valheim
     addon: 9f95fc58_valheim_steamcmd_server
     udp_ports: 2456-2457
     query_port: 2457
     join_address: valheim.example.com:2456
   ```

   `query_port` is optional: with it, the start page shows when the game is
   ready and how many are playing (games that answer Steam server queries).
3. Check **Home networks** covers your home network and your VPN (the
   default covers the usual private networks).
4. Start this add-on. The log shows `starts when someone at home, on the VPN
   or at a remembered address joins`.
5. For the start page from outside, add a proxy host to your reverse proxy
   (e.g. NGINX Proxy Manager): your domain, scheme `http`, your Home
   Assistant's IP, port `8095` (**Start page port**), with HTTPS. Don't
   forward port 8095 on your router.
6. Stop the game when it's idle, for example with an automation that stops
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

## Notifications

Each start fires a `game_waker_wake` event in Home Assistant, with the
add-on's slug (`addon`), the game's name (`game`) and the address that woke
it (`from`), for a notification such as "Valheim is starting".

## Permissions

To watch the ports and start the games, the add-on uses the host network,
raw packet capture (`NET_RAW`) and the Supervisor's manager role (to start
other add-ons). It doesn't change any game or open any ports on your router.
The start page can start the games and nothing else.
