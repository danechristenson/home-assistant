# Changelog

## 26.9.1.2

- Option to use the new Matrix Call Relay add-on for voice and video calls
  between different networks (`call_relay`).

## 26.9.1.1

- Sign Home Assistant users in to Element in the sidebar automatically
  (`ingress_autologin`). On a new server, the sidebar first walks you through
  creating the admin account.
- New **Invite people** page (linked from Element's home screen) that turns
  invite codes into sign-up links and QR codes, plus a QR code for setting up
  the Element app.
- Invite links and the server's own links now use `homeserver_url`.
- Documentation for reaching the server from outside your home through a
  reverse proxy.

## 26.9.1

- Initial release, based on Continuwuity v26.9.1 and Element Web v1.12.30.
- Element Web in the Home Assistant sidebar via ingress.
- Options for server name, homeserver URL, registration token, federation and
  HTTPS with certificates from `/ssl`.
