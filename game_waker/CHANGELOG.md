# Changelog

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
