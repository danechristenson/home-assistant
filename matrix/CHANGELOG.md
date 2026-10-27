# Changelog

## 26.9.1.16

- Update Element Web v1.12.32 → v1.12.33.

## 26.9.1.15

- Update qrcode-generator 1.5.2 → 2.0.4.
  Version 2 moved its file (now `dist/qrcode.js`); the invite page's QR
  codes are unchanged.

## 26.9.1.14

- Update Alpine (Element Web build stage) 3.22 → 3.24.
- Update busybox 1.37 → 1.38.

## 26.9.1.13

- New **Password for phone apps** page (linked from Element's home screen in
  the sidebar): everyone can set a password for their own account, including
  accounts created by sidebar sign-in, to sign in to Element X or FluffyChat.

## 26.9.1.12

- Update Element Web v1.12.31 → v1.12.32.

## 26.9.1.11

- Update Element Web v1.12.30 → v1.12.31.

## 26.9.1.10

- Everyone can join calls in rooms made by FluffyChat or Element X. Those
  rooms only let moderators join calls; with group calls and sidebar sign-in
  on, the add-on now lowers that one permission for every member.

## 26.9.1.9

- Profiles (display names and avatars) now require signing in, so they can't
  be looked up by anyone on the internet.

## 26.9.1.8

- The sidebar launcher is no longer cached by browsers, so fixes reach
  phones and the Home Assistant app right after an update.

## 26.9.1.7

- Element in the sidebar now opens on phones (including the Home Assistant
  app) instead of redirecting to a "get the Element X app" page.

## 26.9.1.6

- Show Element in the sidebar for all Home Assistant users, not only admins,
  so everyone can use sidebar sign-in.

## 26.9.1.5

- Group voice and video calls with screen sharing through the new Matrix
  Group Calls add-on (`group_calls`).
- Update qrcode-generator 1.4.4 → 1.5.2.

## 26.9.1.4

- Public landing page for parents and invitees on port 8010
  (`landing_contact`).
- Room options: `lockdown_room_directory` (only admins publish rooms, on by
  default), `allow_room_creation` and `auto_join_rooms`. Home Assistant
  users' accounts, new and existing, join the auto-join rooms when they open
  Element in the sidebar.
- Federation allow-list (`federation_allowed_servers`); turning federation on
  now publishes `/.well-known/matrix/server` so it works through the reverse
  proxy.
- The **Invite people** page opens in place with a link back to the chat, so
  it works in the Home Assistant mobile app.

## 26.9.1.3

- Fix "Missing option" errors when saving settings after updating from an
  earlier version: newer options are now optional and fall back to their
  defaults.

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
