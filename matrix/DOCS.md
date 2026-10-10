# Matrix

## Installation

1. Add the DC Add-ons repository to Home Assistant (see the
   [repository README](https://github.com/danchristenson/HomeAssistant#installation)).
2. Find **Matrix** in the Add-on Store and click **Install**.
3. On the **Configuration** tab, set `server_name` (see below). The add-on
   will not start without it.
4. Click **Start** and open the **Log** tab. On first start the server prints
   a one-time registration token:

   ```
   register an account on example.com using the registration token XXXXXXXX
   ```

5. Click **Element** in the sidebar (or **Open Web UI**), choose
   **Create account**, and use that token. The first account becomes the
   server admin.

After the admin account exists, other people can register using the
`registration_token` you configure. Without one, only the admin can invite
new users (send `!admin token issue` in the admin room to create a token).

## Configuration

Example:

```yaml
server_name: example.com
homeserver_url: https://matrix.example.com
registration_token: a-long-random-string
allow_federation: false
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
sidebar connects here. If left empty it defaults to
`http://<server_name>:8008` (or `https://` when `ssl` is on).

If you open Home Assistant over HTTPS, this must also be HTTPS, otherwise the
browser blocks Element from connecting.

### Option: `registration_token`

A shared token people must enter to create an account. Leave empty to only
allow registration with tokens the admin issues.

### Option: `allow_federation`

Let your users talk to people on other Matrix servers (e.g. matrix.org).
Off by default. Federation needs a public domain with valid HTTPS; see the
[Continuwuity docs](https://continuwuity.org) for the extra setup. Avoid
turning it off again once enabled.

### Option: `ssl`

Serve the Matrix API over HTTPS using the certificate files below. Leave this
off if a reverse proxy (NGINX, Caddy, Cloudflare Tunnel, ...) handles TLS.

### Option: `certfile` / `keyfile`

Certificate and key files, relative to Home Assistant's `/ssl` directory.
Only used when `ssl` is enabled.

## Network

| Port | Description |
| ---- | ----------- |
| 8008/tcp | Matrix client API. Point Matrix apps here. |

Element itself is served through Home Assistant ingress and needs no port.

## Mobile and desktop apps

Install [Element](https://element.io/download) (or another Matrix client),
choose **Edit** next to the homeserver on the sign-in screen, and enter your
`homeserver_url`. Push notifications work through Element's own push gateway.

## Data and backups

The database is stored in the add-on's `/data` directory and is included in
Home Assistant backups.

## Support

- Add-on issues: <https://github.com/danchristenson/HomeAssistant/issues>
- Continuwuity documentation: <https://continuwuity.org>
