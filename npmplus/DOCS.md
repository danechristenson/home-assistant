# NPMplus

One reverse proxy for the web services you reach from outside: it takes
HTTPS on port 443, holds the certificates, and passes each name on to the
add-on or device behind it (e.g. `matrix.example.com` to Matrix,
`vtt.example.com` to Foundry VTT). Forward only port 443 on your router, to
Home Assistant.

[NPMplus](https://github.com/ZoeyVid/NPMplus) is ZoeyVid's fork of Nginx
Proxy Manager, kept up to date with nginx's fixes. Its own
[README](https://github.com/ZoeyVid/NPMplus#readme) covers the web UI in
detail; this add-on runs its image unchanged.

## Setup

1. Stop any other reverse proxy add-on (such as NGINX Proxy Manager): only
   one can have ports 80 and 443.
2. Start this add-on and open `https://<your Home Assistant's address>:81`.
   The certificate there is self-signed, so the browser warns once; accept
   it. Create your admin account in the setup page that opens.
3. Add a certificate (**TLS Certificates**). With a DNS challenge (your DNS
   provider's API key) you can get a wildcard certificate such as
   `*.example.com`, and port 80 needn't be reachable from outside.
4. Add a proxy host per service (**Proxy Hosts**): the name, then where it
   goes (scheme, address, port), and the certificate.
5. Forward port 443 (TCP, and UDP for HTTP/3) on your router to Home
   Assistant. If you use access lists by address, check from outside
   (e.g. with your phone off Wi-Fi) that they keep out who they should.

Port 81, the web UI, controls every host and certificate, so it's best kept
to your home network or a VPN. If you do want it reachable from outside,
turn on two-factor sign-in (TOTP) for every user first, and use strong
passwords.

## Ports

| Port | What |
| ---- | ---- |
| 80 | HTTP: redirects to HTTPS, and Let's Encrypt's HTTP challenge |
| 443 TCP and UDP | HTTPS and HTTP/3 |
| 81 | The web UI (HTTPS) |

They can be changed on the add-on's **Network** settings, if another add-on
already uses one. NPMplus still sees visitors' real addresses, for access
lists and logs.

## Custom nginx files

NPMplus includes files from its `custom_nginx` folder, such as
`http_top.conf` (in nginx's `http` block) or `server_proxy.conf` (in every
proxy host); see NPMplus's README for the list. Here that folder is the
add-on's config folder, `/addon_configs/<this add-on>`, reachable with the
Samba, SSH or File editor add-ons. Restart the add-on after changing them.
In nginx, the folder is `/data/custom_nginx` (e.g.
`include /data/custom_nginx/blocklist.conf;`).

## Updates

New NPMplus releases reach this add-on within a day, after it is built and
started in a test. Each update restarts NPMplus, and NPMplus then rewrites
every host's nginx files from its database. Check that your access lists
still protect what they should after an update (e.g. that a protected site
asks for its password).

## Moving from another reverse proxy

Add your hosts, access lists and certificates again in the web UI. Stop the
other proxy first: only one can have ports 80 and 443.

## Backups

Home Assistant's backups of this add-on contain everything: hosts, access
lists, users and certificates, with their private keys. Keep them safe.
