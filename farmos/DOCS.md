# Homestead for farmOS

This add-on runs [farmOS](https://farmos.org), which keeps records for a farm
or garden: each plant, bed or field as an item on a map, with its crop or
variety and a log of what happened to it.

## First start

1. Start the add-on and open the web UI.
2. The farmOS installer runs. At **Set up database**, choose **SQLite** and
   enter `/data/db/farmos.sqlite` as the database file.
3. At **Install modules**, keep the defaults. For importing from spreadsheets,
   also choose **CSV importer** (and **CSV exporter** to get them out again).
4. Name your farm and create your own admin account.

On every start the add-on applies any database updates a new farmOS version
brings, and rebuilds farmOS's caches.

## Trimming farmOS

farmOS's defaults suit a working farm: animals, equipment, seed stock,
sensors and more, each with its own menu entries. To keep only what you use,
uninstall the rest at `/admin/modules/uninstall` in the web UI. farmOS's own
**Setup → Modules** page only adds modules. Type a module's name into the
filter, tick it, and uninstall; a module others need stays greyed out until
they're gone. Anything uninstalled can be added again under
**Setup → Modules**.

For a garden or orchard, these are usually enough: Plant, Land and Structure
assets; Activity, Observation, Harvest, Input and Transplanting logs; the
default roles and the land and structure types; and the CSV importer. The
add-on's API key needs the Farm Manager role and the API's OAuth server, so it
turns them back on at the next start if they're uninstalled.

## Importing

With the CSV importer module, farmOS imports spreadsheets under
**Import → CSV**, one importer per kind of record. Import plant types (crops
and varieties) before plants: plants refer to them by name.

## An API key for sync scripts

Scripts on your network can read and write farmOS through its API with their
own key. Once farmOS is installed, the add-on makes an **API sync** account
(with the Farm Manager role) and an API client for it, `api_sync`, which can
only get Farm Manager tokens through the client credentials grant. The
client's id and a random secret are in `api_client.json` in the add-on's own
folder: `addon_configs/<this add-on>/` on your Home Assistant, readable over
SSH or Samba by an administrator.

A script gets a token from `/oauth/token` with `grant_type=client_credentials`,
`client_id`, `client_secret` and `scope=farm_manager`, then calls farmOS's
JSON:API (`/api/...`). Its changes show as the API sync account's.

- **New key:** delete `api_client.json` and restart the add-on.
- **No API access for scripts:** delete the `api_sync` client in farmOS
  (**Administration → Configuration → Web services → Consumers**).

## HTTPS

The web UI is plain HTTP on port 8096 until you choose one (or both) of these.

### On farmOS's own port

Turn on **HTTPS** (`ssl`) in the add-on's configuration. Port 8096 then
serves HTTPS with the certificate in Home Assistant's `/ssl` folder (by
default the Let's Encrypt add-on's `fullchain.pem` and `privkey.pem`), so
farmOS opens at `https://<your Home Assistant address>:8096`, using a name the
certificate covers. Plain `http://` stops working on that port.

Apache reads the certificate when the add-on starts, so restart the add-on
after the certificate is renewed (an automation can do it, for example when
a Certificate Expiry sensor for port 8096 shows the new date).

### Behind a reverse proxy

To open farmOS at its own name, such as `https://farmos.example.com`, through
the NGINX Proxy Manager add-on:

1. Turn on **Behind a reverse proxy** (`reverse_proxy`). farmOS then builds
   its links from the address and `https://` the proxy received. The default
   **Trusted proxy addresses**, `172.30.32.0/23`, is Home Assistant's internal
   network, where NGINX Proxy Manager runs; add your proxy's address if it runs
   on another machine.
2. In NGINX Proxy Manager, add a proxy host for that name, forwarding to
   `http://<Home Assistant IP>:8096` (or `https://` if HTTPS is also on here),
   with a certificate for the name and **Force SSL** on.
3. To keep farmOS to your home network and VPN, give the proxy host an
   **Access List** that allows only those address ranges.

Both can be on at once: `https://<Home Assistant address>:8096` directly and
`https://farmos.example.com` through the proxy.

## Satellite maps (Mapbox)

With the **Mapbox map layers** module and a Mapbox access token (under
**Setup → Settings → Map → Mapbox**), farmOS shows Mapbox satellite imagery
behind every map. Use a public (`pk.`) token. If your Mapbox account lets you
create tokens, make one for farmOS with only the `styles:tiles` scope and a
URL restriction for the exact address you open farmOS at, protocol and port
included (for example `https://homeassistant.local:8096`). Mapbox can't
restrict by IP address, so open farmOS by name. A token can list several
addresses, for example port 8096 and a proxy's name.

Each map view loads its tiles from Mapbox's Static Tiles API, which has a
monthly free allowance; your browser keeps tiles for 12 hours. The token
is only in pages for signed-in farmOS users.

## Privacy

farmOS keeps everything in the add-on and makes no connections of its own.
Its maps load their background tiles from OpenStreetMap in your browser, as
any web map does; the optional Mapbox module loads satellite tiles from
Mapbox instead.

## Notes

- Without the HTTPS options the web UI is plain HTTP on your network (see
  [HTTPS](#https)). To reach it from outside, put it behind a reverse proxy
  rather than forwarding the port.
- SQLite suits a household or a small farm. For many users at once, farmOS
  recommends PostgreSQL.

## Trademark

farmOS is a registered trademark of Michael Stenta. This add-on runs the
official farmOS software; it is not affiliated with or endorsed by the farmOS
project.
