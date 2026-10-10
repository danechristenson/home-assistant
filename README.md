# DC Add-ons for Home Assistant

A collection of Home Assistant add-ons maintained by Dane Christenson.

[![Add repository to Home Assistant][repo-badge]][repo-link]

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**.
2. Open the **⋮** menu (top right) and choose **Repositories**.
3. Add this URL and click **Add**:

   ```
   https://github.com/danchristenson/home-assistant
   ```

4. Close the dialog and refresh the page. The add-ons below will appear under
   **DC Add-ons**.

Or click the button above to add the repository in one step.

## Add-ons

| Add-on | Description |
| ------ | ----------- |
| [Matrix](./matrix) | Private, encrypted group chat on Home Assistant, using the Element app |
| [Matrix Call Relay](./matrix_call_relay) | Companion to Matrix: lets voice and video calls connect between different networks. Optional; needs router port forwards. |
| [VoceChat](./vocechat) | Private group chat for your family or team, hosted on Home Assistant |

## Before you forward ports

These add-ons work on your home network without any router changes. Letting
people reach them from the internet means forwarding ports on your router,
and every forwarded port lets anyone on the internet talk to the software
behind it. A mistake can expose your home network, so:

**Prefer a reverse proxy for web traffic.** Run one reverse proxy (such as
the NGINX Proxy Manager add-on) on port 443 with HTTPS, and forward only 443
to it. Matrix and VoceChat both go behind it; their docs explain how.

**Never forward these:**

| Port | Why |
| ---- | --- |
| 8008 (Matrix), 3000 (VoceChat) | Unencrypted unless the add-on's own HTTPS is on. Use the reverse proxy instead. |
| 8123 (Home Assistant) | Your whole home's controls. Use a VPN (many routers, e.g. GL.iNet, have WireGuard built in) or Home Assistant Cloud. If you must expose it, put it behind the proxy and turn on multi-factor authentication for every user. |
| 81 (NGINX Proxy Manager admin) | Controls all your proxy hosts and certificates. |
| 22, 445, 139, 1883 and other admin or file-sharing ports | SSH, Samba and MQTT are for your home network only. |
| Your router's admin page | Turn off remote administration in the router settings. |

**Only forward what you use.** The Matrix Call Relay needs its own ports
(3478 and 49160-49179); forward them only if you need calls with people
outside your home, and remove forwards for add-ons you uninstall.

**Keep internet-facing add-ons updated.** Security fixes only reach you when
you update.

**Check from outside.** After changing forwards, turn off Wi-Fi on your phone
and confirm only what you intended is reachable, e.g. that
`http://<your public IP>:8123` does not open Home Assistant.

## License

[MIT](./LICENSE)

[repo-badge]: https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg
[repo-link]: https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdanchristenson%2Fhome-assistant
