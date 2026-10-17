# Home Assistant Add-on: Foundry VTT

Foundry Virtual Tabletop on Home Assistant, installed from your own license.

![Supports amd64 Architecture][amd64-shield]
![Supports aarch64 Architecture][aarch64-shield]

[Foundry Virtual Tabletop][foundry] is a self-hosted virtual tabletop for
role-playing games. This add-on provides Node.js and runs Foundry; Foundry
itself comes from your account on foundryvtt.com.

- Paste a timed download link from foundryvtt.com and start: the add-on
  downloads and installs Foundry, then clears the link
- Node.js or Linux downloads, Foundry V12 and newer (V14 needs this add-on's
  Node.js 24)
- Worlds, systems and modules are in the add-on's config folder, so you can
  copy maps and modules in over Samba; they're in Home Assistant backups
- Update Foundry from its own Setup screen, or with a new download link

A Foundry VTT license is required; buy one at [foundryvtt.com][foundry].
This add-on is not made by or affiliated with Foundry Gaming LLC.

See the **Documentation** tab for setup.

[foundry]: https://foundryvtt.com
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
