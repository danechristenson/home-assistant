# Matrix Call Relay

## Do you need this?

Only if people in calls are on different networks, e.g. a friend calling
from mobile data or another house. Calls on your home network work without
it. If you only call people over a VPN into your home, you don't need it
either.

This add-on requires opening ports on your router. Read
[Before you forward ports](https://github.com/danechristenson/home-assistant#before-you-forward-ports)
first.

## Setup

1. Make sure a hostname points to your home's public IP in **public** DNS,
   e.g. `matrix.example.com` (often the same one the Matrix add-on uses).
2. Install this add-on, set `address` to that hostname, and start it. The log
   shows:

   ```text
   [INFO] Call relay running at matrix.example.com (public IP 203.0.113.7)
   ```

3. On your router, forward these to your Home Assistant machine:
   - `3478` TCP and UDP
   - `49160`-`49179` UDP
4. In the **Matrix** add-on, turn on `call_relay` and restart it.

The people you call don't need to change anything; their apps only make
outgoing connections to the relay.

This can't go through a web reverse proxy such as NGINX Proxy Manager. The
relay carries call audio and video, not web traffic, and the other person's
phone must reach the relay ports directly at your public IP.

## Configuration

### Option: `address`

Public hostname (or IP) that phones use to reach the relay. It must resolve
to your home's public IP in public DNS; the add-on looks it up through
1.1.1.1, so a local DNS override for the same name doesn't interfere.

The add-on checks the public IP every 5 minutes and restarts the relay if it
changed (calls in progress at that moment drop, as they would anyway when your
IP changes). Use a plain hostname or IPv4 address: no `https://`, port or
path. IPv6 addresses aren't supported.

While the relay is stopped, the Matrix add-on stops offering it the next time
Matrix restarts.

## Security

- **Access:** the Matrix server hands its users temporary relay passwords
  (valid 24 hours). Nobody without a Matrix account can use the relay, and it
  doesn't answer anything without valid credentials.
- **Home network:** the relay refuses to forward traffic to private and local
  addresses, so it can't be used to reach devices in your home.
- **Abuse limits:** TCP relaying is off (calls only need UDP), each session is
  capped at about 4 Mbit/s, and the relay as a whole at 1200 sessions. There
  is no per-account cap: each set of temporary credentials is limited to 12
  sessions, but an account can request new credentials.
- **Privileges:** the relay drops root and runs as an unprivileged user once
  its ports are open.
- **Updates:** this is internet-facing software. Keep the add-on updated.
- **If an account is misused,** deactivate it in the Matrix admin room with
  `!admin users deactivate <username>`.

The 20 relay ports allow about 10 people in relayed calls at once. Each
relayed video call uses some of your home upload bandwidth.

## Network

| Port | Description |
| ---- | ----------- |
| 3478/tcp+udp | Relay connections from Matrix apps |
| 49160-49179/udp | Call audio and video |

Don't change these port numbers on the add-on's Network tab: the relay tells
phones to use exactly these ports.

## Support

- Add-on issues: <https://github.com/danechristenson/home-assistant/issues>
- coturn: <https://github.com/coturn/coturn>
