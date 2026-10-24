# Changelog

## 1.2.0

- Icon and logo.
- The start page in Home Assistant too: **Open web UI**, and **Show in
  sidebar** for a Game Waker entry. The page shows the icon.
- The `game_waker_wake` event says how the game was woken (`how`: `joined`
  or `start page`).
- Fixed: the page didn't load with anything after a `?` in its address; a
  malformed answer from a game's status query broke the page's status; Start
  on a running game didn't say the address was remembered.

## 1.1.0

- Only trusted addresses wake a game: the home networks (home and VPN),
  addresses listed in the options, and addresses remembered from the new
  start page. Server browsers and trackers query game ports all over the
  internet every few minutes and kept waking it.
- Start page (port 8095, for a reverse proxy): each game's state, players
  and join address, and a Start button that also remembers the visitor's
  address.
- Rewritten in Python (standard library only).

## 1.0.1

- The log shows what woke a game: the port, the packet's size and its first
  bytes, to tell players from server browsers and scans.

## 1.0.0

- First release: starts a stopped game add-on when someone tries to join on
  its UDP ports, without taking the ports. Ignores Home Assistant's own
  traffic and fires a `game_waker_wake` event.
