# Matrix

## Installation

1. Add the DC Add-ons repository to Home Assistant (see the
   [repository README](https://github.com/danchristenson/home-assistant#installation)).
2. Find **Matrix** in the Add-on Store and click **Install**.
3. On the **Configuration** tab, set `server_name` (see below). The add-on
   will not start without it.
4. Click **Start**, then open **Element** in the sidebar.
5. The first time, it asks you to create the server's admin account. Click
   **Create admin account**, use your Home Assistant username as the Matrix
   username, and pick a password. Then click **I've created it**; from now on
   the sidebar signs you in to that admin account automatically.

Until the admin account exists, the sidebar won't create accounts for anyone
else. Anyone with access to the sidebar could claim admin at that point, so
do this step right after installing.

Without the sidebar (or with `ingress_autologin` off), use the one-time token
the server prints in the **Log** tab on first start:

```text
register an account on example.com using the registration token XXXXXXXX
```

Open `<homeserver_url>/_continuwuity/account/register/` in a browser and
enter it there, or use it in any Matrix app's **Create account** screen.

## Signing in from the sidebar (autologin)

When `ingress_autologin` is on, anyone who opens **Element** in the Home
Assistant sidebar is signed in automatically. They get a Matrix account
named after their Home Assistant username, e.g. Home Assistant user `alex`
becomes `@alex:example.com`, with their Home Assistant display name. If that
Matrix account already exists, they are signed in to it.

Things to know:

- Element only runs on secure pages, so open Home Assistant with
  `https://` to use it in the sidebar.
- Accounts created this way have no password. To also use them in the
  Element phone app, have the admin send
  `!admin users reset-password <username>` in the admin room, which replies
  with a new password.
- Signing out of Element in the sidebar also signs you out of Home Assistant
  in that browser, because both share the same browser storage.
- Autologin is unavailable while `ssl` is on. Use a reverse proxy for HTTPS
  instead (see [Access from outside your home](#access-from-outside-your-home)).

## Inviting people

Open **Element** in the sidebar and click **Invite people** on the Home
page. The invite page shows how to create an invite code and turns it into a
sign-up link and QR code. The person opens the link, picks a username and
password, then signs in with the Element app.

Invite codes are created by sending a command in the **Admin Room** in
Element:

| Command | Effect |
| ------- | ------ |
| `!admin token issue --once` | Code for one person |
| `!admin token issue --max-age 7d` | Code anyone can use for 7 days |
| `!admin token issue --max-uses 5` | Code for 5 people |
| `!admin token list` | Show active codes |
| `!admin token revoke <code>` | Cancel a code |

Single-use codes are the safest way to invite friends. Links only work for
people outside your home if `homeserver_url` is a public `https://` address.

## Access from outside your home

The recommended setup for letting friends connect from anywhere:

1. Run a reverse proxy such as the **NGINX Proxy Manager** add-on on port
   443, with a Let's Encrypt certificate (a wildcard certificate via your DNS
   provider's API avoids opening port 80).
2. Add a proxy host, e.g. `matrix.example.com` →
   `http://<Home Assistant IP>:8008`, with **Websockets support**, **Force
   SSL**, **HSTS** and **Block common exploits** turned on. To allow large
   files, add `client_max_body_size 100m;` under **Advanced**.
3. Forward port 443 on your router to the proxy. Do not forward port 8008.
4. In this add-on, leave `ssl` off and set `homeserver_url` to
   `https://matrix.example.com`.

Keep Home Assistant itself private (for example behind your router's
WireGuard VPN). Friends don't need it: they use the Element app and invite
links, and sidebar autologin is only for people with Home Assistant accounts.

## Configuration

Example:

```yaml
server_name: example.com
homeserver_url: https://matrix.example.com
registration_token: ""
allow_federation: false
ingress_autologin: true
ssl: false
certfile: fullchain.pem
keyfile: privkey.pem
```

### Option: `server_name`

**Required. Cannot be changed later.** The domain part of every user ID, e.g.
`example.com` gives users IDs like `@alex:example.com`. Changing it after
accounts exist breaks the server, so pick it once. For a home-only server
something like `home.lan` is fine; if you may want federation later, use a
domain you own.

If you change it anyway, the add-on refuses to start and the log tells you
the original name. Set it back, or uninstall and reinstall the add-on to start
over with an empty server.

### Option: `homeserver_url`

The URL browsers and Matrix apps use to reach the server, e.g.
`https://matrix.example.com` or `http://192.168.1.10:8008`. Element in the
sidebar connects here, and invite links point here. If left empty it defaults
to `http://<server_name>:8008` (or `https://` when `ssl` is on).

If you open Home Assistant over HTTPS, this must also be HTTPS, otherwise the
browser blocks Element from connecting.

### Option: `registration_token`

A shared code anyone can use to create an account. Leave empty and use
invite codes from the admin room instead, so there is no standing code that
could leak.

### Option: `allow_federation`

Let your users talk to people on other Matrix servers (e.g. matrix.org).
Off by default. Federation needs a public domain with valid HTTPS; see the
[Continuwuity docs](https://continuwuity.org) for the extra setup. Avoid
turning it off again once enabled.

### Option: `ingress_autologin`

Sign Home Assistant users in to Element in the sidebar automatically (see
[Signing in from the sidebar](#signing-in-from-the-sidebar-autologin)). On by
default.

### Option: `ssl`

Serve the Matrix API over HTTPS using the certificate files below. A reverse
proxy is recommended instead (see
[Access from outside your home](#access-from-outside-your-home)); while this
is on, sidebar autologin is unavailable.

### Option: `certfile` / `keyfile`

Certificate and key files, relative to Home Assistant's `/ssl` directory.
Only used when `ssl` is enabled.

## Network

| Port | Description |
| ---- | ----------- |
| 8008/tcp | Matrix client API. Point Matrix apps (or your reverse proxy) here. |

Element itself is served through Home Assistant ingress and needs no port.

## Mobile and desktop apps

Install [Element](https://element.io/download) (or another Matrix client),
choose **Edit** next to the homeserver on the sign-in screen, and enter your
`homeserver_url`. On a computer you can also use <https://app.element.io> in a
browser. Push notifications work through Element's own push gateway.

## Data and backups

The database is stored in the add-on's `/data` directory and is included in
Home Assistant backups.

## Support

- Add-on issues: <https://github.com/danchristenson/home-assistant/issues>
- Continuwuity documentation: <https://continuwuity.org>
