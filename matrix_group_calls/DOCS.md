# Matrix Group Calls

## Do you need this?

For **group** calls (three or more people), screen sharing, and calls in
Element X. One-to-one calls in FluffyChat and classic Element work without
it (they use the [Matrix Call Relay](https://github.com/danechristenson/home-assistant/tree/main/matrix_call_relay) for
calls between networks).

This add-on needs a reverse proxy with HTTPS and two router port forwards.
Read [Before you forward ports](https://github.com/danechristenson/home-assistant#before-you-forward-ports) first.

## Setup

1. **DNS:** point a hostname such as `livekit.example.com` at your home's
   public IP in public DNS (a CNAME to your dynamic DNS name works). At home,
   also point it at your Home Assistant machine, like your Matrix hostname.
2. **Install this add-on** and set:
   - `domain`: that hostname, e.g. `livekit.example.com`;
   - `matrix_server_name`: your Matrix server name, e.g. `matrix.example.com`.

   Start it. The log shows
   `Group calls running at https://livekit.example.com (public IP ...)`.
3. **Reverse proxy** (NGINX Proxy Manager): add a proxy host for the
   hostname:
   - **Details:** forward to `http://<Home Assistant IP>:7880`, with
     **Websockets Support** and **Block Common Exploits** on.
   - **SSL:** your certificate (a `*.example.com` wildcard covers it),
     **Force SSL**, **HTTP/2**, **HSTS**.
   - **Advanced:**

     ```nginx
     location ~ ^/(sfu/get|healthz|get_token) {
         proxy_pass http://<Home Assistant IP>:7883;
     }
     ```

4. **Router:** forward these to your Home Assistant machine:
   - `7881` TCP
   - `7882` UDP
5. **Matrix add-on:** turn on `group_calls` and restart it. Its log shows
   `Group calls through https://livekit.example.com`.

Then start a call in a room in Element (sidebar, desktop) or Element X.

## Configuration

### Option: `domain`

Public hostname of the call service, without `https://`. Your reverse proxy
serves it. The add-on looks up its public IP through public DNS (1.1.1.1) and
checks again every 5 minutes, restarting LiveKit if your home IP changes.

### Option: `matrix_server_name`

Your Matrix add-on's server name. Only accounts on it can start calls.

## How it fits together

The add-on uses the host network: LiveKit offers your home-network address
to devices at home (so they connect directly) and your public IP to everyone
else. Only home addresses in 192.168.0.0/16 and 10.0.0.0/8 are offered.

| Piece | Port | Reached by |
| ----- | ---- | ---------- |
| LiveKit signalling | 7880/tcp | Your reverse proxy only |
| Call sign-in service (lk-jwt-service) | 7883/tcp | Your reverse proxy only |
| Call audio and video | 7881/tcp, 7882/udp | The internet (router forwards) |

When someone joins a call, their app asks your Matrix server for a short-lived
token, the sign-in service checks it with your Matrix server, and LiveKit
admits them to that room's call only.

## Security

- **Who can call:** only accounts on `matrix_server_name` can start calls,
  and LiveKit never creates call rooms by itself.
- **Federation:** Continuwuity currently needs its federation API on for the
  sign-in check (Continuwuity issue 1440). The Matrix add-on turns it on with
  **every other server forbidden**, so no outside server can connect. As a
  side effect, the federation API answers basic queries such as the server
  version.
- **Keys:** the LiveKit key and secret are generated on first start, kept in
  the add-on's data and never leave it.
- **Updates:** this is internet-facing software. Keep the add-on updated.

## Network

| Port | Description |
| ---- | ----------- |
| 7880/tcp | LiveKit signalling (reverse proxy only; don't forward) |
| 7883/tcp | Call sign-in service (reverse proxy only; don't forward) |
| 7881/tcp | Call audio and video over TCP (forward on the router) |
| 7882/udp | Call audio and video over UDP (forward on the router) |

Don't change these port numbers on the Network tab: LiveKit tells apps to use
exactly these ports.

## Support

- Add-on issues: <https://github.com/danechristenson/home-assistant/issues>
- Continuwuity call setup: <https://continuwuity.org/calls/livekit>
