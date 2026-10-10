# VoceChat

## Installation

1. Add the DC Add-ons repository to Home Assistant (see the
   [repository README](https://github.com/danchristenson/home-assistant#installation)).
2. Find **VoceChat** in the Add-on Store and click **Install**.
3. Optionally adjust the configuration (see below).
4. Click **Start**, then **Open Web UI**.
5. The first account you create becomes the server admin. Do this right away,
   before exposing the port to anyone else.

The first start downloads the VoceChat web client from GitHub, so the host
needs internet access and the UI may take a minute to appear.

## Configuration

Example:

```yaml
domain: chat.example.com
frontend_url: https://chat.example.com
ssl: true
certfile: fullchain.pem
keyfile: privkey.pem
```

### Option: `domain`

The public domain name of the server, without a scheme. Optional; leave it
empty when you only use VoceChat on your local network.

### Option: `frontend_url`

The full URL people use to reach VoceChat, e.g. `https://chat.example.com`.
VoceChat uses it to build links in invitation and sign-in emails. Set this when
the server is behind a reverse proxy or reached by a different address than
the host's IP.

### Option: `ssl`

Serve VoceChat over HTTPS using the certificate files below. Leave this off if
a reverse proxy (NGINX, Caddy, Cloudflare Tunnel, ...) terminates TLS for you.

### Option: `certfile`

The certificate file, relative to Home Assistant's `/ssl` directory. Only used
when `ssl` is enabled. Default: `fullchain.pem`.

### Option: `keyfile`

The private key file, relative to `/ssl`. Only used when `ssl` is enabled.
Default: `privkey.pem`.

## Access from outside your home

To let people connect from anywhere, put VoceChat behind a reverse proxy
such as the **NGINX Proxy Manager** add-on rather than forwarding port 3000:

1. Add a proxy host, e.g. `chat.example.com` →
   `http://<Home Assistant IP>:3000`, with **Websockets support**, **Force
   SSL**, **HSTS** and **Block common exploits** turned on.
2. Under **Advanced**, add the following. VoceChat streams new messages over a
   long-lived connection that must not be buffered, and this also allows
   larger file uploads:

   ```nginx
   proxy_buffering off;
   proxy_read_timeout 1h;
   client_max_body_size 100m;
   ```

3. Forward only port 443 on your router to the proxy.
4. In this add-on, leave `ssl` off and set `frontend_url` to
   `https://chat.example.com` so invite links use the public address.

## Network

| Port | Description |
| ---- | ----------- |
| 3000/tcp | VoceChat web UI and API |

You can change the host port on the add-on's **Configuration** tab under
**Network**.

## Other settings

Login methods, sign-up rules, SMTP, push notifications and the server name are
managed inside VoceChat itself: open the web UI as the admin and go to
**Settings**.

## Data and backups

All VoceChat data (database, uploads, avatars, keys) is stored in the add-on's
`/data` directory and is included in Home Assistant backups.

## Support

- Add-on issues: <https://github.com/danchristenson/home-assistant/issues>
- VoceChat documentation: <https://doc.voce.chat>
