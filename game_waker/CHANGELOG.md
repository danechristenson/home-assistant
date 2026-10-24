# Changelog

## 1.0.0

- First release: starts a stopped game add-on when someone tries to join on
  its UDP ports, without taking the ports. Ignores Home Assistant's own
  traffic and fires a `game_waker_wake` event.
