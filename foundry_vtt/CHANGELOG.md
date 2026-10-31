# Changelog

## 1.0.6

- Map the add-on's own folder as `app_config`, the Supervisor's current name for
  `addon_config` (same folder; stops a deprecation warning in its log).

## 1.0.5

- Update Node.js 26.12.1 → 26.13.0.

## 1.0.4

- Update Node.js 24.22.1 → 26.12.1.
- New major version: check the upstream release notes before merging.

## 1.0.3

- **Open web UI** goes to the **Address players use** (https when behind
  an HTTPS reverse proxy), not Home Assistant's address on port 30000.
  It can also be shown in the sidebar (**Show in sidebar**).

## 1.0.2

- Update Node.js 24.22.0 → 24.22.1 (security release).

## 1.0.1

- Fixed: bad image

## 1.0.0

- First release: Node.js 24.22.0. Installs Foundry VTT (Node.js or Linux
  download, V12 and newer) from your own timed download link, then runs it
  on port 30000. Foundry's data is in the add-on's config folder, reachable
  over Samba.
