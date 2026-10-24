# Home Assistant Add-on: Game Waker

Starts a stopped game server add-on when someone tries to join it.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

Game servers use memory and CPU even when nobody is playing. Stop them when
they're idle, and Game Waker starts them again when a player's game knocks
on their port.

- Works with any game add-on that players reach over UDP (Valheim and more)
- Only watches the ports; the game still starts and stops as usual
- Ignores Home Assistant's own checks, so status sensors keep working
- Fires a `game_waker_wake` event for notifications

The first try to join fails while the game starts; join again once it's up.

See the **Documentation** tab for setup.

[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
