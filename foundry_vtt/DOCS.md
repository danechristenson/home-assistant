# Foundry VTT

Runs [Foundry Virtual Tabletop](https://foundryvtt.com) on Home Assistant.
The add-on provides Node.js; Foundry itself is downloaded from your own
account, because each copy is licensed to its owner. You need a Foundry VTT
license.

## Setup

1. Install the add-on. Don't start it yet.
2. On [foundryvtt.com](https://foundryvtt.com), sign in and open your
   profile's **Purchased Licenses**. Under **Download Foundry Virtual
   Tabletop**, choose the latest **Stable** version and the **Node.js**
   operating system (**Linux** works too, but is bigger).
3. Press **Timed URL** (the link icon). This copies a download link that
   works for 5 minutes.
4. In Home Assistant, open the add-on's **Configuration** tab, paste the link
   into **Download link** and press **Save**.
5. Start the add-on within 5 minutes. The log shows `Downloading Foundry
   VTT`, then `Starting Foundry VTT 14.xxx`. The download link is then
   cleared from the Configuration tab.
6. Press **Open web UI** (or go to `http://<your Home Assistant IP>:30000`).
   Foundry asks for your license key (from the same Purchased Licenses page)
   and to accept its license agreement.
7. **Set an administrator password straight away**: in Foundry's Setup,
   open **Configuration** and set **Administrator Password**. Until you do,
   anyone who can reach port 30000 can open Setup and change everything.

If the log says the download failed, the link has probably expired: get a
new one (step 3), paste it, save and restart.

## Your worlds, systems and modules

Foundry's data folder is the add-on's config folder. With the **Samba share**
add-on it's at `addon_configs\<id>_foundry_vtt` (the id is a few letters,
the same for every add-on from this repository):

- `Data/worlds`, `Data/systems`, `Data/modules`: what you install from
  Foundry's Setup screen
- `Data/` is also where to put your own maps, tokens and music, e.g.
  `Data/maps`; they're then available in Foundry's file picker
- `Config/options.json`: Foundry's settings (see below)
- `Logs/`: Foundry's own logs

Everything there is in Home Assistant backups, and so is Foundry itself (a
restore doesn't need a new download link). Backups can get large if you keep
a lot of maps and music.

**Moving from another Foundry install?** Stop the add-on, copy your old
`Data` folder's contents into this one's `Data` folder, and start it.

## Updating Foundry

- **Within a version** (e.g. 14.360 → 14.369): Foundry's Setup screen offers
  the update; install it there. The add-on keeps it.
- **A new major version** (e.g. V14 → V15): get a new download link for it
  and paste it into **Download link**, then restart. Foundry's own updater
  can't change major versions. Your worlds and modules are kept. Back up
  first, and check your game system and modules support the new version.

Updates of this add-on only change Node.js, never Foundry.

## Options

- **Download link**: installs (or reinstalls) Foundry from a timed link.
  Cleared once used.
- **Address players use**: the address shown in Foundry's invitation links,
  e.g. `foundry.example.com`. Empty: your Home Assistant's IP address.
- **Behind an HTTPS reverse proxy**: turn on when players reach Foundry
  through a reverse proxy with HTTPS (see below).
- **Open this world on start**: the world's ID (its folder name, shown under
  **Edit World** in Setup). Players can join straight away after a restart.
  Empty: Foundry starts at the Setup screen.

The add-on sets these in `Config/options.json` on each start, together with
port 30000 and UPnP off (so Foundry never opens ports on your router by
itself). Change everything else in Foundry's Setup → Configuration.

## Playing with people outside your home

At home, players go to `http://<your Home Assistant IP>:30000`.

For players elsewhere, put Foundry behind a reverse proxy with HTTPS, as for
Matrix. With the **NGINX Proxy Manager** add-on:

1. Point a domain or subdomain (e.g. `foundry.example.com`) at your home.
2. Add a **Proxy Host**: that domain, scheme `http`, your Home Assistant IP,
   port `30000`. Turn on **Websockets Support** (Foundry needs it) and
   **Block Common Exploits**.
3. On the **SSL** tab, request a certificate and turn on **Force SSL**.
4. On the **Advanced** tab, allow large uploads (maps, music):

   ```
   client_max_body_size 300M;
   ```

5. Here, set **Address players use** to `foundry.example.com` and turn on
   **Behind an HTTPS reverse proxy**, then restart the add-on.

Don't forward port 30000 on your router: it's unencrypted, so passwords
would cross the internet in plain text, and Foundry's voice and video chat
only work over HTTPS.

## Troubleshooting

- **"Foundry VTT isn't installed yet"**: paste a download link (see Setup).
- **"That download isn't a zip file"**: you chose the Windows or macOS
  download; choose Node.js.
- **The add-on stops right after "Starting Foundry VTT"**: check the log; a
  very old Foundry (V11 or before) may not run on Node.js 24. Install a
  current version.
- **Forgot the administrator password**: stop the add-on, delete
  `Config/admin.txt` in the config folder, start it and set a new one.
