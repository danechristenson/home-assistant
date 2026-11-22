# Home Assistant Add-on: NPMplus

Reverse proxy with HTTPS certificates and a web UI.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

[NPMplus](https://github.com/ZoeyVid/NPMplus) is ZoeyVid's fork of Nginx
Proxy Manager: the same way of adding proxy hosts, access lists and Let's
Encrypt certificates, with a current nginx and HTTP/3. This add-on runs
NPMplus's own image unchanged.

- New NPMplus releases arrive here within a day, built and tested first
- Proxy hosts, access lists, certificates (including DNS challenges for
  wildcard certificates) from the web UI on port 81
- Custom nginx files in the add-on's config folder
- Sees visitors' real addresses, for access lists and logs, without the host
  network

See the **Documentation** tab for setup.

[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
