# Home Assistant Add-on: Matrix

Private, encrypted group chat on Home Assistant, using the Element app.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

A chat app, similar to Slack or WhatsApp groups, that runs on your own Home
Assistant machine instead of someone else's servers. Messages are end-to-end
encrypted, so only the people in a conversation can read them.

- Chat in the Home Assistant sidebar, or with the free
  [Element](https://element.io/download) app on iPhone, Android and computers
- Group rooms, direct messages, file and photo sharing, voice messages
- Voice and video calls at home; for calls with people on other networks,
  also install the separate [Matrix Call Relay][call-relay] add-on
- No limits on users or message history
- Private to your household by default; can optionally connect to other
  [Matrix](https://matrix.org) servers so you can chat with people elsewhere

Under the hood this runs [Continuwuity](https://continuwuity.org), a
lightweight Matrix server.

See the **Documentation** tab for installation and configuration details.

[call-relay]: https://github.com/danchristenson/home-assistant/tree/main/matrix_call_relay
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
