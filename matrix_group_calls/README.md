# Home Assistant Add-on: Matrix Group Calls

Group voice and video calls with screen sharing for the Matrix add-on.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

A companion to the [Matrix][matrix] add-on, which must also be installed;
after setting this up, turn on **Group calls** there.

It runs [LiveKit][livekit], the media server behind Element Call, plus the
small service that lets your Matrix users into calls. With it, people in a
room can start a group call in Element or Element X, with video and screen
sharing.

- Only accounts on your Matrix server can start calls
- Needs a reverse proxy (HTTPS) and two router port forwards: read the
  [port forwarding warnings][warnings] before installing

See the **Documentation** tab for setup.

[matrix]: https://github.com/danechristenson/home-assistant/tree/main/matrix
[livekit]: https://livekit.io
[warnings]: https://github.com/danechristenson/home-assistant#before-you-forward-ports
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
