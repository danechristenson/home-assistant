# Home Assistant Add-on: Matrix Call Relay

Lets Matrix voice and video calls connect between different networks.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

A companion to the [Matrix][matrix] add-on, which must also be installed;
after setting this up, turn on **Use call relay** there.

Calls between people on the same home network work without it, but when
someone is on mobile data or behind a strict router, their phone often can't
reach yours directly. This add-on runs a relay ([coturn][coturn], a TURN
server) that both phones connect out to, so the call goes through.

- Only people with an account on your Matrix server can use it
- Refuses to forward anything into your home network
- Needs router port forwards: read the
  [port forwarding warnings](https://github.com/danechristenson/home-assistant#before-you-forward-ports)
  before installing

See the **Documentation** tab for setup.

[matrix]: https://github.com/danechristenson/home-assistant/tree/main/matrix
[coturn]: https://github.com/coturn/coturn
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
