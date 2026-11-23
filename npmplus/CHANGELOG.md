# Changelog

## 1.1.0

- Removed the one-time import from another NPMplus; the add-on is now
  NPMplus's image with Gravatar turned off, and nothing else. To move in from
  another NPMplus or Nginx Proxy Manager, add your hosts again in the web UI.

## 1.0.0

- NPMplus 2026-11-18-r1 (ZoeyVid's image, unchanged and pinned by digest),
  for amd64 and aarch64.
- The add-on's config folder is NPMplus's `custom_nginx` folder.
- Moving in from another NPMplus: its data, put in `import` in the add-on's
  config folder, is copied in on the first start.
