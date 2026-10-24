# DC Add-ons for Home Assistant

A collection of Home Assistant add-ons maintained by Dane Christenson.

[![Add repository to Home Assistant][repo-badge]][repo-link]

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**.
2. Open the **⋮** menu (top right) and choose **Repositories**.
3. Add this URL and click **Add**:

   ```
   https://github.com/danechristenson/home-assistant
   ```

4. Close the dialog and refresh the page. The add-ons below will appear under
   **DC Add-ons**.

Or click the button above to add the repository in one step.

## Add-ons

| Add-on | Description |
| ------ | ----------- |
| [Foundry VTT](./foundry_vtt) | Foundry Virtual Tabletop, installed from your own license with a timed download link. |
| [Game Waker](./game_waker) | Starts a stopped game server add-on (such as Valheim) when someone tries to join, so idle games can be stopped. No ports. |
| [Matrix](./matrix) | Private, encrypted group chat on Home Assistant, using the Element app |
| [Matrix Group Calls](./matrix_group_calls) | Companion to Matrix: group voice and video calls with screen sharing. Optional; needs a reverse proxy and router port forwards. |
| [Matrix Helper Bot](./matrix_helper_bot) | Companion to Matrix: lets people you choose reset forgotten passwords and manage Minecraft players from encrypted chat rooms. Optional; no ports. |
| [Matrix Call Relay](./matrix_call_relay) | Companion to Matrix: lets voice and video calls connect between different networks. Optional; needs router port forwards. |
| [Minecraft Bedrock](./minecraft_bedrock) | Minecraft Bedrock server (itzg image), with players managed from chat through the Matrix Helper Bot. |

## Before you forward ports

These add-ons work on your home network without any router changes. Letting
people reach them from the internet means forwarding ports on your router,
and every forwarded port lets anyone on the internet talk to the software
behind it. A mistake can expose your home network, so:

**Prefer a reverse proxy for web traffic.** Run one reverse proxy (such as
the NGINX Proxy Manager add-on) on port 443 with HTTPS, and forward only 443
to it. Matrix and Foundry VTT go behind it; their docs explain how.

**Never forward these:**

| Port | Why |
| ---- | --- |
| 8008 (Matrix) | Unencrypted unless the add-on's own HTTPS is on. Use the reverse proxy instead. |
| 8123 (Home Assistant) | Your whole home's controls. Use a VPN (many routers, e.g. GL.iNet, have WireGuard built in) or Home Assistant Cloud. If you must expose it, put it behind the proxy and turn on multi-factor authentication for every user. |
| 30000 (Foundry VTT) | Unencrypted. Use the reverse proxy instead (its docs explain how). |
| 7880, 7883 (Matrix Group Calls) | Reached through the reverse proxy over HTTPS, never directly. |
| 81 (NGINX Proxy Manager admin) | Controls all your proxy hosts and certificates. |
| 22, 445, 139, 1883 and other admin or file-sharing ports | SSH, Samba and MQTT are for your home network only. |
| Your router's admin page | Turn off remote administration in the router settings. |

**Only forward what you use.** Some add-ons need their own ports, and only
when you use them:

| Add-on | Forward |
| ------ | ------- |
| Matrix Group Calls | 7881 TCP, 7882 UDP (never 7880 or 7883: those go through the reverse proxy) |
| Matrix Call Relay | 3478 TCP and UDP, 49160-49179 UDP |

Remove forwards for add-ons you uninstall.

**Keep internet-facing add-ons updated.** Security fixes only reach you when
you update.

**Check from outside.** After changing forwards, turn off Wi-Fi on your phone
and confirm only what you intended is reachable, e.g. that
`http://<your public IP>:8123` does not open Home Assistant.

## License

[MIT](./LICENSE)

[repo-badge]: https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg
[repo-link]: https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdanechristenson%2Fhome-assistant
