# Home Assistant Add-on: Game Waker

Starts a stopped game server add-on when someone tries to join it.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

Game servers use memory and CPU even when nobody is playing. Stop them when
they're idle, and Game Waker starts them again when they're wanted.

- From home or your VPN, just join: the knock on the port starts the game
- From elsewhere, press **Start** on the start page once; that address is
  remembered, so joining from there starts it next time
- Server browsers and scanners on the internet never wake a game
- Works with any game add-on that players reach over UDP (Valheim and more)
- Only watches the ports; the game still starts and stops as usual
- Fires a `game_waker_wake` event for notifications

See the **Documentation** tab for setup.

[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
